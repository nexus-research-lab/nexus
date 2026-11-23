// INPUT: 已离开 Room 尾轮的 transcript_ref 行、成员 transcript 当前投影与读模型里的成员检查点。
// OUTPUT: 按引用身份聚合的冻结消息摘要与成员检查点，以及成员 transcript 追加后的语义校验结果。
// POS: Room 增量读模型的依赖栅栏；用"已发布引用的投影是否变化"替代"依赖文件是否变化"，
// 成员只追加时从检查点续读，不再重读和重算整份 transcript。
package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// roomFrozenTranscriptRef 记录某个成员 transcript 中已被非尾轮引用的消息及其投影摘要。
// 成员每跑一轮都会追加 transcript/overlay，文件快照必然变化；只要这些消息的投影不变，
// 已发布的旧分组就仍与 canonical 重建等价。JSON 形状存放在读模型里，改动需提升读模型版本。
type roomFrozenTranscriptRef struct {
	WorkspacePath     string `json:"workspace_path"`
	PrivateSessionKey string `json:"private_session_key"`
	AgentID           string `json:"agent_id"`
	SessionID         string `json:"session_id"`
	// MessageIDs 按封存先后排列，Digest 沿这个顺序逐条折叠各消息的投影摘要：
	// 新封存的消息接在旧摘要之后，完整校验按同一顺序从头折叠。
	MessageIDs []string `json:"message_ids"`
	Digest     string   `json:"digest"`
	// Checkpoint 是成员 transcript 的投影位置：MessageIDs 的投影全部在它之前，此后不再变化；
	// nil 表示下次只能完整读取 transcript 并重算摘要。
	Checkpoint *transcriptCheckpoint `json:"checkpoint,omitempty"`
	// Pending 是已冻结、但投影仍在 Checkpoint 之后的消息：每次续读都会重新投影到它们，逐条比对摘要。
	// 没有 Checkpoint 时为空。
	Pending []roomFrozenTranscriptMessage `json:"pending,omitempty"`
}

// roomFrozenTranscriptMessage 是一条冻结消息及其投影摘要（消息不在 transcript 里时摘要为空）。
type roomFrozenTranscriptMessage struct {
	ID     string `json:"id"`
	Digest string `json:"digest,omitempty"`
}

// frozenIDs 返回全部冻结消息的 ID：已封存的在前，待封存的在后。
func (ref roomFrozenTranscriptRef) frozenIDs() []string {
	ids := make([]string, 0, len(ref.MessageIDs)+len(ref.Pending))
	ids = append(ids, ref.MessageIDs...)
	for _, message := range ref.Pending {
		ids = append(ids, message.ID)
	}
	return ids
}

func (ref roomFrozenTranscriptRef) transcriptRef() roomTranscriptRef {
	return roomTranscriptRef{
		WorkspacePath:     ref.WorkspacePath,
		PrivateSessionKey: ref.PrivateSessionKey,
		AgentID:           ref.AgentID,
		SessionID:         ref.SessionID,
	}
}

// roomFrozenKeyBytes 是成员身份键摘要保留的字节数。键只用来判断“续读内容是否触及已冻结前缀”，
// 两个不同的键摘要相同只会多报一次冲突并回到完整读取，不会漏掉变化；完整重建一次要写上万个键，
// 写入成本与键的长度成正比，所以只保留足以区分的定长摘要。
const roomFrozenKeyBytes = 12

// frozenKeys 把成员 transcript 的身份键放进 Room scope 的冻结键集合：每个键是引用身份与原键的定长摘要。
func (ref roomTranscriptRef) frozenKeys(keys historyTailKeys) historyTailKeys {
	member := historyPageScope(ref.cacheKey())
	scoped := make(historyTailKeys, len(keys))
	for key := range keys {
		sum := sha256.Sum256([]byte(member + key.Key))
		scoped.add(key.Kind, base64.RawURLEncoding.EncodeToString(sum[:roomFrozenKeyBytes]))
	}
	return scoped
}

func (ref roomTranscriptRef) dependsOn(source historyPageSourceSnapshot) bool {
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

// collectRoomFrozenTranscriptRefs 把新离开尾轮的 transcript_ref 合入既有冻结集合，并为本次读过
// transcript 的成员建立或推进检查点；tail 是留在尾轮里的引用。从检查点续读的成员只处理待封存与
// 新冻结的消息，完整读取的成员按当前投影重新封存全部冻结消息。返回的身份键与冻结集合在同一事务内发布。
// cache 必须来自同一次行解析，避免重复读取 transcript。
func (s *RoomHistoryStore) collectRoomFrozenTranscriptRefs(
	ctx context.Context,
	ownerUserID string,
	frozenRows []map[string]any,
	tail roomRefSelections,
	previous []roomFrozenTranscriptRef,
	cache roomTranscriptIndexCache,
) ([]roomFrozenTranscriptRef, historyTailKeys, error) {
	byKey := make(map[string]roomFrozenTranscriptRef, len(previous))
	for _, item := range previous {
		byKey[item.transcriptRef().cacheKey()] = item
	}
	// 同一条消息被再次引用时不重复冻结：续读的成员若引用到已封存的消息，会先被身份键挡回完整读取。
	added := s.selectRoomRefs(ownerUserID, frozenRows)
	for key, member := range added {
		if _, exists := byKey[key]; !exists {
			byKey[key] = roomFrozenTranscriptRef{
				WorkspacePath:     member.ref.WorkspacePath,
				PrivateSessionKey: member.ref.PrivateSessionKey,
				AgentID:           member.ref.AgentID,
				SessionID:         member.ref.SessionID,
			}
		}
	}
	result := make([]roomFrozenTranscriptRef, 0, len(byKey))
	keys := make(historyTailKeys)
	for _, key := range slices.Sorted(maps.Keys(byKey)) {
		item := byKey[key]
		ref := item.transcriptRef()
		var fresh []string
		if member := added[key]; member != nil {
			fresh = slices.Sorted(maps.Keys(member.ids))
			if _, err := s.loadRoomTranscriptIndex(ctx, ownerUserID, ref, cache); err != nil {
				return nil, nil, err
			}
		}
		// 本次没有读过的成员依赖未变，也没有新冻结的消息，原状态继续成立。
		if projection := cache.members[key]; projection != nil {
			var tailIDs map[string]bool
			if member := tail[key]; member != nil {
				tailIDs = member.ids
			}
			memberKeys, err := item.seal(projection, fresh, tailIDs)
			if err != nil {
				return nil, nil, err
			}
			maps.Copy(keys, ref.frozenKeys(memberKeys))
		}
		result = append(result, item)
	}
	return result, keys, nil
}

// seal 按 projection 推进成员状态：fresh 是本次新冻结的消息。检查点取不晚于尾轮引用消息的最后一个
// 轮次起点，投影全部在它之前（或不在 transcript 里）的冻结消息被封存进 Digest，其余留在 Pending；
// 没有可用起点或起点之前的 marker 对齐无法定型时全部封存、不留检查点，下次完整读取。返回建立或推进检查点时新冻结的身份：起点之前的 transcript 条目、
// 投影行的 message_id 与封存的消息，续读时任何一个再次出现都说明检查点之前的投影可能已变。
func (ref *roomFrozenTranscriptRef) seal(
	projection *roomTranscriptProjection,
	fresh []string,
	tailIDs map[string]bool,
) (historyTailKeys, error) {
	// 续读时已封存的消息都在检查点之前，不必重看；完整读取到的是全部消息的当前投影，冻结集合按它重新封存。
	candidates := ref.Pending
	if !projection.Resumed {
		fresh = append(ref.frozenIDs(), fresh...)
		candidates, ref.MessageIDs, ref.Digest = nil, nil, ""
	}
	candidates = slices.Grow(slices.Clip(candidates), len(fresh))
	seen := make(map[string]struct{}, len(candidates)+len(fresh))
	for _, message := range candidates {
		seen[message.ID] = struct{}{}
	}
	for _, id := range fresh {
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		digest, err := roomFrozenMessageDigest(projection.Messages, id)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, roomFrozenTranscriptMessage{ID: id, Digest: digest})
	}

	var keys historyTailKeys
	reprojected := make(map[string]struct{})
	ref.Checkpoint = nil
	if replay := projection.Replay; replay != nil {
		if checkpoint, entries, boundary, ok := roomFrozenTranscriptCheckpoint(replay, tailIDs); ok {
			ref.Checkpoint, keys = &checkpoint, entries
			for _, row := range replay.Rows[:boundary] {
				keys.add(historyTailKeyMessage, stringFromAny(row["message_id"]))
			}
			for _, row := range replay.Rows[boundary:] {
				reprojected[stringFromAny(row["message_id"])] = struct{}{}
			}
		}
	}
	ref.MessageIDs, ref.Pending = slices.Clip(ref.MessageIDs), nil
	for _, message := range candidates {
		if _, ok := reprojected[message.ID]; ok {
			ref.Pending = append(ref.Pending, message)
			continue
		}
		ref.MessageIDs = append(ref.MessageIDs, message.ID)
		ref.Digest = extendRoomFrozenTranscriptDigest(ref.Digest, message)
		if ref.Checkpoint != nil {
			keys.add(historyTailKeyMessage, message.ID)
		}
	}
	return keys, nil
}

// roomFrozenTranscriptCheckpoint 在不晚于尾轮引用消息的最后一个轮次起点建立检查点，并返回新冻结的条目身份
// 与该起点之前的投影行数：尾轮引用的消息仍可能变化，必须留在检查点之后由每次续读重新投影。
// 没有这样的起点，或该起点之前的 marker 对齐无法定型时返回 false。
func roomFrozenTranscriptCheckpoint(
	replay *transcriptReplay,
	tailIDs map[string]bool,
) (transcriptCheckpoint, historyTailKeys, int, bool) {
	firstTail := len(replay.Rows)
	for index, row := range replay.Rows {
		if tailIDs[stringFromAny(row["message_id"])] {
			firstTail = index
			break
		}
	}
	start := -1
	for index, candidate := range replay.Starts {
		if candidate.Rows <= firstTail {
			start = index
		}
	}
	if start < 0 {
		return transcriptCheckpoint{}, nil, 0, false
	}
	checkpoint, entries, ok := replay.checkpoint(start)
	return checkpoint, entries, replay.Starts[start].Rows, ok
}

// resumeRoomFrozenTranscripts 让带检查点的成员从检查点续读 transcript：续读成立即证明检查点之前的
// 投影（含全部已封存消息）没有变化，待封存消息再按续读到的投影逐条比对，cache 里只放检查点之后的消息。
// wanted 是本次要解析的引用；只处理被引用或依赖已变化的成员。previous 与 current 是成员依赖在
// 上次发布和本次刷新时的快照，逐项对应。无法证明前缀不变的成员留给完整读取与摘要校验。
func (s *RoomHistoryStore) resumeRoomFrozenTranscripts(
	ctx context.Context,
	db historyReadModelQuerier,
	ownerUserID string,
	scopeKey string,
	frozen []roomFrozenTranscriptRef,
	wanted roomRefSelections,
	previous []historyPageSourceSnapshot,
	current []historyPageSourceSnapshot,
	cache roomTranscriptIndexCache,
) error {
	ownerHistory := s.agentHistory.ForOwner(ownerUserID)
	for _, item := range frozen {
		if item.Checkpoint == nil {
			continue
		}
		ref := item.transcriptRef()
		key := ref.cacheKey()
		member := wanted[key]
		needed := member != nil
		transcript, rewritten := -1, false
		var before, after []historyPageSourceSnapshot
		for index, source := range previous {
			if !ref.dependsOn(source) {
				continue
			}
			next := current[index]
			needed = needed || source != next
			// 只把变长的文件当作追加；长度不变的改动是原地改写，首尾指纹看不到两端之间的内容，必须重读整份文件。
			// overlay 变长的同时两端之外被改写仍看不到，判定强度与 overlay 进程内回放一致。
			rewritten = rewritten || (source != next && next.Size <= source.Size)
			if source.Kind == historyPageSourceTranscript {
				transcript = len(before)
			}
			before, after = append(before, source), append(after, next)
		}
		if !needed || rewritten || transcript < 0 {
			continue
		}
		// marker 对齐的续接只核对下标，检查点之前的 marker 内容要靠 overlay 只被追加来证明。
		if err := ownerHistory.checkAgentHistorySourcesAppended(ctx, ref.WorkspacePath, ref.PrivateSessionKey, before, after); errors.Is(err, errHistoryPageIndexInvalid) {
			continue
		} else if err != nil {
			return err
		}
		overlayState, err := ownerHistory.readOverlayHistoryStateContext(ctx, ref.WorkspacePath, ref.PrivateSessionKey)
		if err != nil {
			return err
		}
		touchesFrozen := func(keys historyTailKeys) (bool, error) {
			return historyReadTailKeysContain(ctx, db, scopeKey, ref.frozenKeys(keys))
		}
		replay, err := ownerHistory.resumeTranscriptProjection(
			ctx,
			ref.WorkspacePath,
			protocol.Session{SessionKey: ref.PrivateSessionKey, AgentID: ref.AgentID},
			before[transcript],
			after[transcript],
			*item.Checkpoint,
			overlayState.RoundMarkers,
			touchesFrozen,
		)
		if errors.Is(err, errHistoryPageIndexInvalid) {
			continue
		}
		if err != nil {
			return err
		}
		// 续读到的消息不能与检查点之前的身份重合，否则已封存的消息被续写或重新出现，需要完整投影。
		touched := make(historyTailKeys)
		for _, row := range replay.Rows {
			touched.add(historyTailKeyMessage, stringFromAny(row["message_id"]))
		}
		conflict, err := touchesFrozen(touched)
		if err != nil {
			return err
		}
		if conflict {
			continue
		}
		messages := indexRoomTranscriptMessages(replay.Rows)
		// 本次引用的消息必须都在续读结果里：指向检查点之前的消息，或落在续读暂不接受的
		// 无换行末行里的消息，只有完整投影才能解析。
		if !member.resolvedBy(messages) {
			continue
		}
		if err = item.verifyPending(messages); err != nil {
			return err
		}
		cache.members[key] = &roomTranscriptProjection{Messages: messages, Replay: replay, Resumed: true}
	}
	return nil
}

// resolvedBy 判断本次引用的消息是否都在 messages 里；没有引用时成立。
func (m *roomRefSelection) resolvedBy(messages map[string]protocol.Message) bool {
	if m == nil {
		return true
	}
	for id := range m.ids {
		if _, ok := messages[id]; !ok {
			return false
		}
	}
	return true
}

// verifyRoomFrozenTranscriptRefs 为依赖已变化、又没能从检查点续读的引用身份重读 transcript 并重算摘要；
// 任一冻结消息的投影变化（含重写、分支切换或 round marker 重新归属）都必须回退全量重建。
func (s *RoomHistoryStore) verifyRoomFrozenTranscriptRefs(
	ctx context.Context,
	ownerUserID string,
	frozen []roomFrozenTranscriptRef,
	changed []historyPageSourceSnapshot,
	cache roomTranscriptIndexCache,
) error {
	for _, item := range frozen {
		ref := item.transcriptRef()
		if cache.resumed(ref.cacheKey()) {
			continue
		}
		affected := false
		for _, source := range changed {
			if ref.dependsOn(source) {
				affected = true
				break
			}
		}
		if !affected {
			continue
		}
		projection, err := s.loadRoomTranscriptIndex(ctx, ownerUserID, ref, cache)
		if err != nil {
			return err
		}
		digest := ""
		for _, id := range item.MessageIDs {
			messageDigest, err := roomFrozenMessageDigest(projection.Messages, id)
			if err != nil {
				return err
			}
			digest = extendRoomFrozenTranscriptDigest(digest, roomFrozenTranscriptMessage{ID: id, Digest: messageDigest})
		}
		if digest != item.Digest {
			return fmt.Errorf("published transcript message changed: %w", errHistoryPageIndexInvalid)
		}
		if err = item.verifyPending(projection.Messages); err != nil {
			return err
		}
	}
	return nil
}

// verifyPending 按 index 里的当前投影逐条比对待封存的冻结消息。
func (ref roomFrozenTranscriptRef) verifyPending(index map[string]protocol.Message) error {
	for _, message := range ref.Pending {
		digest, err := roomFrozenMessageDigest(index, message.ID)
		if err != nil {
			return err
		}
		if digest != message.Digest {
			return fmt.Errorf("published transcript message changed: %w", errHistoryPageIndexInvalid)
		}
	}
	return nil
}

// roomFrozenMessageDigest 返回 index 里该消息投影的摘要；消息不在 transcript 里时为空。
func roomFrozenMessageDigest(index map[string]protocol.Message, id string) (string, error) {
	message, ok := index[id]
	if !ok {
		return "", nil
	}
	encoded, err := json.Marshal(message)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

// extendRoomFrozenTranscriptDigest 把一条消息折叠到 digest 之后。
func extendRoomFrozenTranscriptDigest(digest string, message roomFrozenTranscriptMessage) string {
	sum := sha256.Sum256(fmt.Appendf(nil, "%s|%d:%s|%s", digest, len(message.ID), message.ID, message.Digest))
	return hex.EncodeToString(sum[:])
}
