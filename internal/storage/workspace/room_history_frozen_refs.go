// INPUT: 已离开 Room 尾轮的 transcript_ref 行与成员 transcript 当前投影。
// OUTPUT: 按引用身份聚合的冻结消息摘要，以及成员 transcript 追加后的语义校验结果。
// POS: Room 增量读模型的依赖栅栏；用"已发布引用的投影是否变化"替代"依赖文件是否变化"。
package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// roomFrozenTranscriptRef 记录某个成员 transcript 中已被非尾轮引用的消息及其投影摘要。
// 成员每跑一轮都会追加 transcript/overlay，文件快照必然变化；只要这些消息的投影不变，
// 已发布的旧分组就仍与 canonical 重建等价。
type roomFrozenTranscriptRef struct {
	WorkspacePath     string   `json:"workspace_path"`
	PrivateSessionKey string   `json:"private_session_key"`
	AgentID           string   `json:"agent_id"`
	SessionID         string   `json:"session_id"`
	MessageIDs        []string `json:"message_ids"`
	Digest            string   `json:"digest"`
}

func (ref roomFrozenTranscriptRef) transcriptRef() roomTranscriptRef {
	return roomTranscriptRef{
		WorkspacePath:     ref.WorkspacePath,
		PrivateSessionKey: ref.PrivateSessionKey,
		AgentID:           ref.AgentID,
		SessionID:         ref.SessionID,
	}
}

func (ref roomFrozenTranscriptRef) dependsOn(source historyPageSourceSnapshot) bool {
	if filepath.Clean(ref.WorkspacePath) != source.WorkspacePath {
		return false
	}
	switch source.Kind {
	case historyPageSourceRoomPrivateOverlay:
		return source.SessionKey == ref.PrivateSessionKey
	case historyPageSourceTranscript:
		return source.SessionID == ref.SessionID
	default:
		return false
	}
}

// collectRoomFrozenTranscriptRefs 把新离开尾轮的 transcript_ref 合入既有冻结集合，
// 并只为新增消息的引用身份重算摘要；cache 必须来自同一次行解析，避免重复读取 transcript。
func (s *RoomHistoryStore) collectRoomFrozenTranscriptRefs(
	ctx context.Context,
	ownerUserID string,
	frozenRows []map[string]any,
	previous []roomFrozenTranscriptRef,
	cache roomTranscriptIndexCache,
) ([]roomFrozenTranscriptRef, error) {
	byKey := make(map[string]*roomFrozenTranscriptRef, len(previous))
	ids := make(map[string]map[string]struct{}, len(previous))
	for _, item := range previous {
		copied := item
		copied.MessageIDs = append([]string(nil), item.MessageIDs...)
		key := copied.transcriptRef().cacheKey()
		byKey[key] = &copied
		ids[key] = make(map[string]struct{}, len(copied.MessageIDs))
		for _, id := range copied.MessageIDs {
			ids[key][id] = struct{}{}
		}
	}
	dirty := make(map[string]struct{})
	for _, row := range frozenRows {
		if stringFromAny(row[overlayKindField]) != overlayKindTranscriptRef {
			continue
		}
		ref, ok := roomTranscriptRefFromRow(protocol.Message(row))
		if !ok || !s.paths.workspacePathIsConfinedForOwner(ownerUserID, ref.WorkspacePath) {
			continue
		}
		key := ref.cacheKey()
		item := byKey[key]
		if item == nil {
			item = &roomFrozenTranscriptRef{
				WorkspacePath:     ref.WorkspacePath,
				PrivateSessionKey: ref.PrivateSessionKey,
				AgentID:           ref.AgentID,
				SessionID:         ref.SessionID,
			}
			byKey[key] = item
			ids[key] = make(map[string]struct{})
		}
		if _, exists := ids[key][ref.MessageID]; exists {
			continue
		}
		ids[key][ref.MessageID] = struct{}{}
		item.MessageIDs = append(item.MessageIDs, ref.MessageID)
		dirty[key] = struct{}{}
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]roomFrozenTranscriptRef, 0, len(keys))
	for _, key := range keys {
		item := byKey[key]
		if _, changed := dirty[key]; changed {
			sort.Strings(item.MessageIDs)
			index, err := s.loadRoomTranscriptIndex(ctx, ownerUserID, item.transcriptRef(), cache)
			if err != nil {
				return nil, err
			}
			digest, err := roomFrozenTranscriptDigest(index, item.MessageIDs)
			if err != nil {
				return nil, err
			}
			item.Digest = digest
		}
		result = append(result, *item)
	}
	return result, nil
}

// verifyRoomFrozenTranscriptRefs 只重读依赖于已变化源文件的引用身份；
// 任一冻结消息的投影变化（含重写、分支切换或 round marker 重新归属）都必须回退全量重建。
func (s *RoomHistoryStore) verifyRoomFrozenTranscriptRefs(
	ctx context.Context,
	ownerUserID string,
	frozen []roomFrozenTranscriptRef,
	changed []historyPageSourceSnapshot,
	cache roomTranscriptIndexCache,
) error {
	for _, item := range frozen {
		affected := false
		for _, source := range changed {
			if item.dependsOn(source) {
				affected = true
				break
			}
		}
		if !affected {
			continue
		}
		index, err := s.loadRoomTranscriptIndex(ctx, ownerUserID, item.transcriptRef(), cache)
		if err != nil {
			return err
		}
		digest, err := roomFrozenTranscriptDigest(index, item.MessageIDs)
		if err != nil {
			return err
		}
		if digest != item.Digest {
			return fmt.Errorf("published transcript message changed: %w", errHistoryPageIndexInvalid)
		}
	}
	return nil
}

func roomFrozenTranscriptDigest(index map[string]protocol.Message, messageIDs []string) (string, error) {
	hasher := sha256.New()
	for _, id := range messageIDs {
		var payload []byte
		if message, ok := index[id]; ok {
			encoded, err := json.Marshal(message)
			if err != nil {
				return "", err
			}
			payload = encoded
		}
		_, _ = fmt.Fprintf(hasher, "%d:%s|%d:", len(id), id, len(payload))
		_, _ = hasher.Write(payload)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
