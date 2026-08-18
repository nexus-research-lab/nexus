// INPUT: owner-scoped DM/Room 历史、请求取消信号、活跃 round 身份与 finalized Goal usage report。
// OUTPUT: 归一化消息页、当前 generation 大内容 detail、共享 Round Navigator 及按当前聚合真相刷新的 Goal 完成收据。
// POS: Session 历史统一读取与短冷建协议边界。
package session

import (
	"context"
	"path/filepath"
	"strings"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// GetSessionMessages 读取 session 历史消息。
func (s *Service) GetSessionMessages(ctx context.Context, rawSessionKey string) ([]protocol.Message, error) {
	sessionKey, parsed, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return nil, err
	}
	if parsed.Kind == protocol.SessionKeyKindRoom {
		items, readErr := s.roomHistory.ReadMessages(
			authctx.OwnerUserID(ctx),
			parsed.ConversationID,
			s.activeRoundIDs(sessionKey),
		)
		if readErr != nil {
			return nil, readErr
		}
		return s.refreshGoalCompletionReceipts(ctx, items), nil
	}

	workspacePaths, err := s.resolveWorkspacePaths(ctx, parsed.AgentID)
	if err != nil {
		return nil, err
	}
	sessionValue, workspacePath, err := s.loadHistorySession(ctx, workspacePaths, parsed, sessionKey)
	if err != nil {
		return nil, err
	}
	if sessionValue == nil {
		return nil, ErrSessionNotFound
	}
	items, err := s.ownerHistory(ctx).ReadMessages(
		workspacePath,
		*sessionValue,
		s.activeRoundIDs(sessionKey),
	)
	if err != nil {
		return nil, err
	}
	return s.refreshGoalCompletionReceipts(ctx, items), nil
}

// GetSessionMessagesPage 分页读取 session 历史消息。
func (s *Service) GetSessionMessagesPage(
	ctx context.Context,
	rawSessionKey string,
	request MessagePageRequest,
) (*protocol.MessagePage, error) {
	sessionKey, parsed, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if parsed.Kind == protocol.SessionKeyKindRoom {
		page, err := s.roomHistory.ReadMessagesPageContext(
			ctx,
			authctx.OwnerUserID(ctx),
			parsed.ConversationID,
			s.activeRoundIDs(sessionKey),
			workspacestore.HistoryPageQuery{
				Limit:                request.Limit,
				BeforeRoundID:        request.BeforeRoundID,
				BeforeRoundTimestamp: request.BeforeRoundTimestamp,
				AroundRoundID:        request.AroundRoundID,
				AroundLimit:          request.AroundLimit,
				DeferIndex:           request.DeferIndex,
			},
		)
		if err != nil {
			return nil, err
		}
		page.Items = s.refreshGoalCompletionReceipts(ctx, page.Items)
		page.Items = attachMessageDetailSessionKey(page.Items, sessionKey)
		return &page, nil
	}

	workspacePaths, err := s.resolveWorkspacePaths(ctx, parsed.AgentID)
	if err != nil {
		return nil, err
	}
	sessionValue, workspacePath, err := s.loadHistorySession(ctx, workspacePaths, parsed, sessionKey)
	if err != nil {
		return nil, err
	}
	if sessionValue == nil {
		return nil, ErrSessionNotFound
	}
	page, err := s.ownerHistory(ctx).ReadMessagesPageContext(
		ctx,
		workspacePath,
		*sessionValue,
		s.activeRoundIDs(sessionKey),
		workspacestore.HistoryPageQuery{
			Limit:                request.Limit,
			BeforeRoundID:        request.BeforeRoundID,
			BeforeRoundTimestamp: request.BeforeRoundTimestamp,
			AroundRoundID:        request.AroundRoundID,
			AroundLimit:          request.AroundLimit,
			DeferIndex:           request.DeferIndex,
		},
	)
	if err != nil {
		return nil, err
	}
	page.Items = s.refreshGoalCompletionReceipts(ctx, page.Items)
	page.Items = attachMessageDetailSessionKey(page.Items, sessionKey)
	return &page, nil
}

// GetSessionMessageDetail 读取消息页中大型 Tool result / 图片的当前派生内容。
func (s *Service) GetSessionMessageDetail(
	ctx context.Context,
	rawSessionKey string,
	ref string,
) (*MessageDetail, error) {
	sessionKey, parsed, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	var detail workspacestore.HistoryMessageDetail
	if parsed.Kind == protocol.SessionKeyKindRoom {
		detail, err = s.roomHistory.ReadMessageDetailContext(
			ctx,
			authctx.OwnerUserID(ctx),
			parsed.ConversationID,
			ref,
		)
	} else {
		var workspacePaths []string
		workspacePaths, err = s.resolveWorkspacePaths(ctx, parsed.AgentID)
		if err == nil {
			var sessionValue *protocol.Session
			var workspacePath string
			sessionValue, workspacePath, err = s.loadHistorySession(
				ctx,
				workspacePaths,
				parsed,
				sessionKey,
			)
			if err == nil && sessionValue == nil {
				err = ErrSessionNotFound
			}
			if err == nil {
				detail, err = s.ownerHistory(ctx).ReadMessageDetailContext(
					ctx,
					workspacePath,
					*sessionValue,
					ref,
				)
			}
		}
	}
	if err != nil {
		return nil, err
	}
	return &MessageDetail{
		Ref:       detail.Ref,
		Kind:      detail.Kind,
		MediaType: detail.MediaType,
		ByteSize:  detail.ByteSize,
		Payload:   detail.Payload,
	}, nil
}

func attachMessageDetailSessionKey(
	items []protocol.Message,
	sessionKey string,
) []protocol.Message {
	result := items
	copied := false
	for index, item := range items {
		content, changed := attachMessageDetailSessionKeyValue(
			item["content"],
			sessionKey,
		)
		if !changed {
			continue
		}
		if !copied {
			result = append([]protocol.Message(nil), items...)
			copied = true
		}
		result[index] = protocol.Clone(item)
		result[index]["content"] = content
	}
	return result
}

func attachMessageDetailSessionKeyValue(value any, sessionKey string) (any, bool) {
	switch typed := value.(type) {
	case []any:
		result := make([]any, len(typed))
		changed := false
		for index, item := range typed {
			projected, itemChanged := attachMessageDetailSessionKeyValue(item, sessionKey)
			result[index] = projected
			changed = changed || itemChanged
		}
		return result, changed
	case []map[string]any:
		result := make([]map[string]any, len(typed))
		changed := false
		for index, item := range typed {
			projected, itemChanged := attachMessageDetailSessionKeyValue(item, sessionKey)
			result[index], _ = projected.(map[string]any)
			changed = changed || itemChanged
		}
		return result, changed
	case map[string]any:
		result := typed
		changed := false
		if ref := strings.TrimSpace(historyDetailString(typed["detail_ref"])); ref != "" {
			result = cloneStringAnyMap(typed)
			result["detail_session_key"] = sessionKey
			changed = true
		}
		for _, key := range []string{"content", "source"} {
			projected, itemChanged := attachMessageDetailSessionKeyValue(typed[key], sessionKey)
			if !itemChanged {
				continue
			}
			if !changed {
				result = cloneStringAnyMap(typed)
				changed = true
			}
			result[key] = projected
		}
		return result, changed
	default:
		return value, false
	}
}

func cloneStringAnyMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value)+1)
	for key, item := range value {
		result[key] = item
	}
	return result
}

func historyDetailString(value any) string {
	result, _ := value.(string)
	return result
}

func (s *Service) refreshGoalCompletionReceipts(
	ctx context.Context,
	items []protocol.Message,
) []protocol.Message {
	if s == nil || s.goalUsage == nil || len(items) == 0 {
		return items
	}
	ownerUserID := authctx.OwnerUserID(ctx)
	reports := make(map[string]*protocol.GoalUsageReport)
	result := items
	copied := false
	for index := range items {
		receipt, ok := protocol.GoalCompletionReceiptFromAny(
			items[index][protocol.GoalCompletionReceiptField],
		)
		if !ok {
			continue
		}
		report, loaded := reports[receipt.GoalID]
		if !loaded {
			value, err := s.goalUsage.UsageByGoalIDForOwner(ctx, receipt.GoalID, ownerUserID)
			if err != nil || value == nil {
				reports[receipt.GoalID] = nil
				continue
			}
			report = value
			reports[receipt.GoalID] = report
		}
		if report == nil || !report.UsageFinalized ||
			protocol.NormalizeGoalStatus(report.Status) != protocol.GoalStatusComplete {
			continue
		}
		updated := messageutil.BuildGoalCompletionReceipt(receipt.GoalID, receipt.RoundID, report)
		if receipt.Equal(updated) {
			continue
		}
		if !copied {
			result = append([]protocol.Message(nil), items...)
			copied = true
		}
		result[index] = protocol.Clone(items[index])
		result[index][protocol.GoalCompletionReceiptField] = updated
	}
	return result
}

// GetSessionRoundIndex 读取 session 的轻量 round 导航索引。
func (s *Service) GetSessionRoundIndex(ctx context.Context, rawSessionKey string) (*protocol.SessionRoundIndex, error) {
	return s.GetSessionRoundIndexPage(ctx, rawSessionKey, false)
}

// GetSessionRoundIndexPage 读取 round 导航索引，可显式选择短冷建协议。
func (s *Service) GetSessionRoundIndexPage(
	ctx context.Context,
	rawSessionKey string,
	deferIndex bool,
) (*protocol.SessionRoundIndex, error) {
	sessionKey, parsed, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return nil, err
	}
	if parsed.Kind == protocol.SessionKeyKindRoom {
		index, err := s.roomHistory.ReadRoundIndexPageContext(
			ctx,
			authctx.OwnerUserID(ctx),
			parsed.ConversationID,
			s.activeRoundIDs(sessionKey),
			deferIndex,
		)
		if err != nil {
			return nil, err
		}
		return &index, nil
	}

	workspacePaths, err := s.resolveWorkspacePaths(ctx, parsed.AgentID)
	if err != nil {
		return nil, err
	}
	sessionValue, workspacePath, err := s.loadHistorySession(ctx, workspacePaths, parsed, sessionKey)
	if err != nil {
		return nil, err
	}
	if sessionValue == nil {
		return nil, ErrSessionNotFound
	}
	index, err := s.ownerHistory(ctx).ReadRoundIndexPageContext(
		ctx,
		workspacePath,
		*sessionValue,
		s.activeRoundIDs(sessionKey),
		deferIndex,
	)
	if err != nil {
		return nil, err
	}
	return &index, nil
}

func (s *Service) loadHistorySession(
	ctx context.Context,
	workspacePaths []string,
	parsed protocol.SessionKey,
	sessionKey string,
) (*protocol.Session, string, error) {
	roomSession, err := s.repository.GetRoomSessionByKey(ctx, authctx.OwnerUserID(ctx), parsed)
	if err != nil {
		return nil, "", err
	}
	if roomSession != nil {
		workspacePath := resolveHistoryWorkspacePath(workspacePaths, parsed)
		hydrated, hydrateErr := s.hydrateRoomHistorySession(ctx, workspacePath, sessionKey, *roomSession)
		if hydrateErr != nil {
			return nil, "", hydrateErr
		}
		return hydrated, workspacePath, nil
	}

	item, workspacePath, err := s.ownerFiles(ctx).FindSession(workspacePaths, sessionKey)
	if err != nil {
		return nil, "", err
	}
	return item, workspacePath, nil
}

func resolveHistoryWorkspacePath(workspacePaths []string, parsed protocol.SessionKey) string {
	for _, workspacePath := range workspacePaths {
		if filepath.Base(workspacePath) == parsed.AgentID {
			return workspacePath
		}
	}
	if len(workspacePaths) > 0 {
		return workspacePaths[0]
	}
	return ""
}

func (s *Service) hydrateRoomHistorySession(
	ctx context.Context,
	workspacePath string,
	sessionKey string,
	roomSession protocol.Session,
) (*protocol.Session, error) {
	if workspacePath == "" {
		return &roomSession, nil
	}

	fileSession, _, err := s.ownerFiles(ctx).FindSession([]string{workspacePath}, sessionKey)
	if err != nil {
		return nil, err
	}
	if fileSession == nil {
		return &roomSession, nil
	}

	merged := dmdomain.MergeRoomBackedSession(*fileSession, roomSession)
	roomSessionID := strings.TrimSpace(stringPointerValue(roomSession.SessionID))
	fileSessionID := strings.TrimSpace(stringPointerValue(fileSession.SessionID))
	if roomSessionID == "" && fileSessionID != "" {
		merged.SessionID = fileSession.SessionID
		if merged.RoomSessionID != nil && strings.TrimSpace(*merged.RoomSessionID) != "" {
			if updateErr := s.repository.UpdateRoomSessionSDKSessionID(
				ctx,
				strings.TrimSpace(*merged.RoomSessionID),
				fileSessionID,
			); updateErr != nil {
				return nil, updateErr
			}
		}
	}
	return &merged, nil
}
