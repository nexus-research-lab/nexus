// INPUT: DM 会话的 round marker、overlay 消息、实时收到的 transcript 原生回复、改写裁剪、读取请求与会话删除。
// OUTPUT: 写入时落表的 DM 历史、按 canonical 读取的同步整会话回填，以及分页、导航、detail 的读表入口。
// POS: DM 会话接入历史读模型（见 conversation_history.go）。只覆盖 agent:<id>:<channel>:dm:<ref> 会话：
// Room 成员私有会话的 transcript 原生回复不经过 DM 写入点，fork 尚未完成的会话读的是源会话 transcript 前缀，
// 这两类会话分页时直接按 canonical 读取。
package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// dmConversationHistoryScope 以 workspace 路径与 session key 定位会话：workspace 路径已经区分 owner，
// 不依赖调用方是否经过 ForOwner。
func dmConversationHistoryScope(workspacePath, sessionKey string) (conversationHistoryScope, bool) {
	sessionKey = strings.TrimSpace(sessionKey)
	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind != protocol.SessionKeyKindAgent || parsed.ChatType != "dm" || strings.TrimSpace(workspacePath) == "" {
		return conversationHistoryScope{}, false
	}
	return conversationHistoryScope{
		key:            "dm\x00" + filepath.Clean(strings.TrimSpace(workspacePath)) + "\x00" + sessionKey,
		defaultAgentID: strings.TrimSpace(parsed.AgentID),
	}, true
}

// dmConversationHistoryIdentity 是 canonical 读取依赖的会话来源；变化时（换 transcript、开启分段）整会话回填。
func dmConversationHistoryIdentity(sessionValue protocol.Session) (string, bool) {
	if source, _ := pendingForkTranscript(sessionValue); source != "" {
		return "", false
	}
	segmented, _ := sessionValue.Options[protocol.OptionRuntimeSegmentedTranscript].(bool)
	parts := []string{"dm", strings.TrimSpace(sessionValue.AgentID), fmt.Sprintf("segmented=%t", segmented)}
	parts = append(parts, historyTranscriptSessionIDs(sessionValue)...)
	return strings.Join(parts, "\x00"), true
}

// projectDMConversationRows 把刚写进 overlay（或只在 transcript 里）的 DM 行落表。失败只标记失效，不影响写入结果。
func (s *AgentHistoryStore) projectDMConversationRows(workspacePath, sessionKey string, rows ...protocol.Message) {
	if len(rows) == 0 {
		return
	}
	scope, ok := dmConversationHistoryScope(workspacePath, sessionKey)
	if !ok {
		return
	}
	h := s.history
	ctx := context.Background()
	unlock := h.lock(scope.key)
	defer unlock()
	if err := h.applyLiveRows(ctx, scope, rows); err != nil {
		logx.FromContext(ctx).Warn("DM 历史落表失败，等待整会话回填", "session_key", sessionKey, "err", err)
		h.markBroken(ctx, scope.key)
	}
}

// projectDMRoundMarker 把刚追加的 round marker 物化成用户行后落表，与 canonical 读取的物化规则相同。
func (s *AgentHistoryStore) projectDMRoundMarker(workspacePath, sessionKey string, row map[string]any) {
	if _, ok := dmConversationHistoryScope(workspacePath, sessionKey); !ok {
		return
	}
	// 经过一次 JSON 往返，让 metadata、attachments 等字段与从 overlay 文件读出的形状一致。
	encoded, err := json.Marshal(row)
	if err != nil {
		return
	}
	var decoded map[string]any
	if err = json.Unmarshal(encoded, &decoded); err != nil {
		return
	}
	normalized, ok := normalizeDecodedJSONValue(decoded).(map[string]any)
	if !ok {
		return
	}
	agentID := protocol.ParseSessionKey(strings.TrimSpace(sessionKey)).AgentID
	if userRow, visible := materializeRoundMarkerMessage(sessionKey, agentID, roundMarkerFromOverlayRow(normalized)); visible {
		s.projectDMConversationRows(workspacePath, sessionKey, userRow)
	}
}

// RecordTranscriptNativeMessage 记录实时收到、只写进 transcript 的 assistant 回复：它不进 overlay，
// 直接落表（canonical 读取会从 transcript 投影出同一条消息），并更新会话摘要。
func (s *AgentHistoryStore) RecordTranscriptNativeMessage(workspacePath string, sessionKey string, message protocol.Message) {
	s.projectDMConversationRows(workspacePath, sessionKey, message)
	s.RecordReplyPreview(sessionKey, message)
}

// removeDMConversationRounds 删除改写裁掉的轮次。
func (s *AgentHistoryStore) removeDMConversationRounds(workspacePath, sessionKey string, roundIDs map[string]struct{}) {
	scope, ok := dmConversationHistoryScope(workspacePath, sessionKey)
	if !ok {
		return
	}
	keys := make([]string, 0, len(roundIDs))
	for roundID := range roundIDs {
		if key := historyPageGroupKey(protocol.Message{"round_id": roundID}, false); key != "" {
			keys = append(keys, key)
		}
	}
	h := s.history
	ctx := context.Background()
	unlock := h.lock(scope.key)
	defer unlock()
	if err := h.removeRounds(ctx, scope, keys); err != nil {
		logx.FromContext(ctx).Warn("DM 历史裁剪轮次失败，等待整会话回填", "session_key", sessionKey, "err", err)
		h.markBroken(ctx, scope.key)
	}
}

// invalidateDMConversationHistory 让会话下一次读取整会话回填；用于绕过常规写入点直接改写 overlay 的操作（fork）。
func (s *AgentHistoryStore) invalidateDMConversationHistory(workspacePath, sessionKey string) {
	scope, ok := dmConversationHistoryScope(workspacePath, sessionKey)
	if !ok {
		return
	}
	h := s.history
	unlock := h.lock(scope.key)
	defer unlock()
	h.markBroken(context.Background(), scope.key)
}

// backfillDMConversationHistory 按 canonical 读取整会话回填。回填开始后的写入由实时写入直接落表，
// 表里已有的行比回填快照新，按「已有行在后」合并。
func (s *AgentHistoryStore) backfillDMConversationHistory(
	ctx context.Context,
	workspacePath string,
	sessionValue protocol.Session,
) error {
	scope, ok := dmConversationHistoryScope(workspacePath, sessionValue.SessionKey)
	identity, identified := dmConversationHistoryIdentity(sessionValue)
	if !ok || !identified {
		return nil
	}
	h := s.history
	unlock := h.lock(scope.key)
	err := h.resetScope(ctx, scope.key, identity, 0)
	unlock()
	if err != nil {
		return err
	}
	rows, err := withRuntimePermissionRepair(s, func() ([]protocol.Message, error) {
		return s.readHistoryRowsContext(ctx, workspacePath, sessionValue)
	})
	if errors.Is(err, os.ErrNotExist) {
		rows, err = nil, nil
	}
	if err != nil {
		return err
	}
	if err = h.applyBackfillRows(ctx, scope, compactConversationHistoryBackfillRows(rows)); err != nil {
		return err
	}
	unlock = h.lock(scope.key)
	defer unlock()
	return h.finishBackfill(ctx, scope.key, func(state conversationHistoryState) error {
		if state.Identity != identity {
			return errConversationHistoryBackfillAborted
		}
		return nil
	})
}

// readyDMConversationHistory 保证 DM 会话可以读表：未回填、已失效或来源身份变化时同步整会话回填。
// covered=false 表示会话不在读模型覆盖范围内，调用方按 canonical 读取。
func (s *AgentHistoryStore) readyDMConversationHistory(
	ctx context.Context,
	workspacePath string,
	sessionValue protocol.Session,
) (scope conversationHistoryScope, identity string, covered bool, err error) {
	scope, ok := dmConversationHistoryScope(workspacePath, sessionValue.SessionKey)
	identity, identified := dmConversationHistoryIdentity(sessionValue)
	if !ok || !identified {
		return scope, "", false, nil
	}
	h := s.history
	backfill := func(ctx context.Context) error {
		return s.backfillDMConversationHistory(ctx, workspacePath, sessionValue)
	}
	for attempt := 0; attempt < 3; attempt++ {
		state, exists, err := h.state(ctx, scope.key)
		if err != nil {
			return scope, "", true, err
		}
		if !exists || !state.ready() || state.Identity != identity {
			// 来源身份变了也要回填：同一把锁里 resetScope 会换上新的身份。
			if err = h.ensure(ctx, scope.key+"\x00"+identity, backfill); err != nil && !retryableConversationHistoryErr(err) {
				return scope, "", true, err
			}
			continue
		}
		if h.needsVerification(ctx, scope.key) {
			h.markBroken(ctx, scope.key)
			continue
		}
		return scope, identity, true, nil
	}
	return scope, "", true, errConversationHistoryNotReady
}

// readCanonicalHistoryPage 是读模型不覆盖的会话的分页：按 canonical 读取整段历史后切页。
func (s *AgentHistoryStore) readCanonicalHistoryPage(
	ctx context.Context,
	workspacePath string,
	sessionValue protocol.Session,
	request historyPageIndexRequest,
) (protocol.MessagePage, error) {
	rows, err := s.readHistoryRowsContext(ctx, workspacePath, sessionValue)
	if errors.Is(err, os.ErrNotExist) {
		rows, err = nil, nil
	}
	if err != nil {
		return protocol.MessagePage{}, err
	}
	groups, err := buildHistoryPageIndexedGroups(ctx, rows, request.CollapseRoomRounds)
	if err != nil {
		return protocol.MessagePage{}, err
	}
	return paginateHistoryPageIndexedGroups(groups, request), nil
}

// deleteDMConversationHistory 在删除 DM 会话目录后清掉表里的数据。
func deleteDMConversationHistory(h *conversationHistoryStore, workspacePath, sessionKey string) {
	scope, ok := dmConversationHistoryScope(workspacePath, sessionKey)
	if !ok {
		return
	}
	if _, err := os.Stat(h.path); errors.Is(err, os.ErrNotExist) {
		return
	}
	unlock := h.lock(scope.key)
	defer unlock()
	if err := h.deleteScope(context.Background(), scope.key); err != nil {
		logx.FromContext(context.Background()).Warn("DM 历史删除失败", "session_key", sessionKey, "err", err)
	}
}

// ReadMessageDetailContext 展开 DM 历史页里的大块内容。读模型不覆盖的会话按 canonical 分页，页面里没有 detail 引用。
func (s *AgentHistoryStore) ReadMessageDetailContext(
	ctx context.Context,
	workspacePath string,
	sessionValue protocol.Session,
	ref string,
) (HistoryMessageDetail, error) {
	scope, ok := dmConversationHistoryScope(workspacePath, sessionValue.SessionKey)
	identity, identified := dmConversationHistoryIdentity(sessionValue)
	if !ok || !identified {
		return HistoryMessageDetail{}, ErrHistoryMessageDetailUnavailable
	}
	detail, found, err := s.history.readDetail(ctx, scope, identity, ref)
	if err != nil {
		return HistoryMessageDetail{}, err
	}
	if !found {
		return HistoryMessageDetail{}, ErrHistoryMessageDetailUnavailable
	}
	return detail, nil
}
