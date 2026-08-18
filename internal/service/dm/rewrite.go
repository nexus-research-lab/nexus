// INPUT: 最后一条 durable DM 用户消息、其 terminal 结果与可选 SDK transcript 边界。
// OUTPUT: 严格裁剪旧 round 后启动 replacement round；启动前失败允许精确 overlay-only 裁剪。
// POS: DM“编辑/重新运行”命令的服务端事务规划边界。
package dm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type rewritePruneInput struct {
	WorkspacePath      string
	SessionKey         string
	TargetRoundID      string
	ReplacementRoundID string
	RoundIDs           []string
	RemoveMessageCount int
}

// HandleRewriteLastUserMessage 编辑最后一条用户消息，并基于新的上下文重新生成。
func (s *Service) HandleRewriteLastUserMessage(ctx context.Context, request RewriteRequest) error {
	sessionKey, parsed, err := s.validateRewriteRequest(request)
	if err != nil {
		s.loggerFor(ctx).Warn("拒绝 DM rewrite 请求",
			"session_key", strings.TrimSpace(request.SessionKey),
			"agent_id", strings.TrimSpace(request.AgentID),
			"target_round_id", strings.TrimSpace(request.TargetRoundID),
			"client_request_id", strings.TrimSpace(request.ClientRequestID),
			"client_message_id", strings.TrimSpace(request.ClientMessageID),
			"err", err,
		)
		return err
	}
	logger := s.loggerFor(ctx).With(
		"session_key", sessionKey,
		"target_round_id", strings.TrimSpace(request.TargetRoundID),
		"client_request_id", strings.TrimSpace(request.ClientRequestID),
		"client_message_id", strings.TrimSpace(request.ClientMessageID),
	)
	if runningRoundIDs := s.runtime.GetRunningRoundIDs(sessionKey); len(runningRoundIDs) > 0 {
		logger.Warn("拒绝 DM rewrite：已有运行中 round", "running_round_ids", runningRoundIDs)
		return errors.New("cannot rewrite while a round is running")
	}

	agentID := dmdomain.FirstNonEmpty(parsed.AgentID, request.AgentID)
	if agentID == "" {
		defaultAgent, defaultErr := s.agents.GetDefaultAgent(ctx)
		if defaultErr != nil {
			logger.Warn("DM rewrite 读取默认 Agent 失败", "err", defaultErr)
			return defaultErr
		}
		agentID = defaultAgent.AgentID
	}
	logger = logger.With("agent_id", agentID)
	logger.Info("受理 DM rewrite 请求",
		"content_chars", utf8.RuneCountInString(strings.TrimSpace(request.Content)),
		"content_preview", logx.PreviewText(strings.TrimSpace(request.Content), 240),
		"attachment_count", len(request.Attachments),
	)
	agentValue, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		logger.Warn("DM rewrite 读取 Agent 失败", "err", err)
		return err
	}
	sessionItem, err := s.ensureSession(ctx, agentValue, parsed, sessionKey)
	if err != nil {
		logger.Warn("DM rewrite 确保 session 失败", "err", err)
		return err
	}
	ownerHistory := s.history.ForOwner(agentValue.OwnerUserID)
	rows, err := ownerHistory.ReadMessages(agentValue.WorkspacePath, sessionItem, nil)
	if err != nil {
		logger.Warn("DM rewrite 读取历史失败", "workspace_path", agentValue.WorkspacePath, "err", err)
		return fmt.Errorf("读取 DM rewrite 历史: %w", err)
	}
	lastUser, ok := lastVisibleUserMessage(rows)
	if !ok {
		logger.Warn("拒绝 DM rewrite：会话为空")
		return errors.New("cannot rewrite an empty conversation")
	}
	targetRoundID := strings.TrimSpace(request.TargetRoundID)
	if targetRoundID != strings.TrimSpace(dmdomain.NormalizeString(lastUser["round_id"])) {
		logger.Warn("拒绝 DM rewrite：目标不是最后一条用户消息",
			"last_user_round_id", strings.TrimSpace(dmdomain.NormalizeString(lastUser["round_id"])),
		)
		return fmt.Errorf("can only rewrite the last user message")
	}

	attachments := request.Attachments
	if len(attachments) == 0 {
		attachments = protocol.ChatAttachmentsFromAny(lastUser["attachments"])
	}
	tail, overlayOnly, err := resolveRewriteTail(
		ownerHistory,
		agentValue.WorkspacePath,
		sessionKey,
		sessionItem,
		rows,
		targetRoundID,
	)
	if err != nil {
		logger.Warn("DM rewrite 解析 runtime 历史尾部失败",
			"workspace_path", agentValue.WorkspacePath,
			"session_id", dmdomain.StringPointerValue(sessionItem.SessionID),
			"err", err,
		)
		return fmt.Errorf("解析 DM rewrite transcript 尾部: %w", err)
	}
	replacementRoundID := protocol.NewRoundID()
	logger.Info("准备重跑 DM rewrite",
		"replacement_round_id", replacementRoundID,
		"source_row_count", len(rows),
		"remove_message_uuid_count", len(tail.MessageUUIDs),
		"remove_round_ids", tail.RoundIDs,
		"target_message_uuid", tail.TargetMessageUUID,
		"attachment_count", len(attachments),
	)
	return s.HandleChat(ctx, Request{
		SessionKey:                sessionKey,
		AgentID:                   agentID,
		Content:                   request.Content,
		Attachments:               attachments,
		RoundID:                   replacementRoundID,
		ClientRequestID:           request.ClientRequestID,
		ClientMessageID:           request.ClientMessageID,
		DeliveryPolicy:            protocol.ChatDeliveryPolicyQueue,
		BroadcastUserMessage:      true,
		RewriteTargetRoundID:      targetRoundID,
		RewriteRemoveMessageUUIDs: tail.MessageUUIDs,
		RewriteRemoveRoundIDs:     tail.RoundIDs,
		RewriteRemoveMessageCount: countHistoryRowsForRound(rows, targetRoundID),
		rewriteOverlayOnly:        overlayOnly,
	})
}

func resolveRewriteTail(
	history *workspacestore.AgentHistoryStore,
	workspacePath string,
	sessionKey string,
	session protocol.Session,
	rows []protocol.Message,
	targetRoundID string,
) (workspacestore.TranscriptRoundTail, bool, error) {
	overlayOnlyCandidate := isUnmaterializedFailedRound(rows, targetRoundID)
	sessionID := dmdomain.StringPointerValue(session.SessionID)
	if strings.TrimSpace(sessionID) == "" {
		if overlayOnlyCandidate {
			return overlayOnlyRewriteTail(targetRoundID), true, nil
		}
		return workspacestore.TranscriptRoundTail{}, false, errors.New(
			"cannot rewrite before the runtime transcript exists",
		)
	}

	tail, err := history.ResolveTranscriptRoundTail(
		workspacePath,
		sessionKey,
		sessionID,
		targetRoundID,
	)
	if err == nil {
		return tail, false, nil
	}
	if overlayOnlyCandidate && errors.Is(err, workspacestore.ErrTranscriptRoundNotFound) {
		return overlayOnlyRewriteTail(targetRoundID), true, nil
	}
	return workspacestore.TranscriptRoundTail{}, false, err
}

func overlayOnlyRewriteTail(targetRoundID string) workspacestore.TranscriptRoundTail {
	targetRoundID = strings.TrimSpace(targetRoundID)
	return workspacestore.TranscriptRoundTail{
		TargetRoundID: targetRoundID,
		RoundIDs:      []string{targetRoundID},
	}
}

// isUnmaterializedFailedRound 只接受精确 user + terminal error 且没有 assistant
// 物化的 round。普通编辑、成功 round 和 transcript 不一致仍走严格 UUID 边界。
func isUnmaterializedFailedRound(rows []protocol.Message, targetRoundID string) bool {
	targetRoundID = strings.TrimSpace(targetRoundID)
	hasUser := false
	hasRuntimeAssistant := false
	hasTerminalError := false
	for _, row := range rows {
		if protocol.MessageRoundID(row) != targetRoundID {
			continue
		}
		switch protocol.MessageRole(row) {
		case "user":
			hasUser = true
		case "assistant":
			if isSyntheticTerminalErrorAssistant(row) {
				hasTerminalError = true
				continue
			}
			hasRuntimeAssistant = true
		case "result":
			hasTerminalError = row["is_error"] == true &&
				strings.EqualFold(dmdomain.NormalizeString(row["subtype"]), "error")
		}
	}
	return hasUser && hasTerminalError && !hasRuntimeAssistant
}

func isSyntheticTerminalErrorAssistant(row protocol.Message) bool {
	summary, ok := row["result_summary"].(map[string]any)
	if !ok || summary["is_error"] != true ||
		!strings.EqualFold(dmdomain.NormalizeString(summary["subtype"]), "error") {
		return false
	}
	resultMessageID := strings.TrimSpace(dmdomain.NormalizeString(summary["message_id"]))
	assistantMessageID := strings.TrimSpace(dmdomain.NormalizeString(row["message_id"]))
	return resultMessageID != "" && assistantMessageID == "assistant_"+resultMessageID
}

func (s *Service) validateRewriteRequest(request RewriteRequest) (string, protocol.SessionKey, error) {
	sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
	if err != nil {
		return "", protocol.SessionKey{}, err
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return "", protocol.SessionKey{}, ErrRoomSessionNotImplemented
	}
	if strings.TrimSpace(request.TargetRoundID) == "" {
		return "", protocol.SessionKey{}, errors.New("target_round_id is required")
	}
	if !protocol.HasChatInput(request.Content, request.Attachments) {
		return "", protocol.SessionKey{}, errors.New("content is required")
	}
	return sessionKey, parsed, nil
}

func (s *Service) pruneHistoryRewriteTail(ctx context.Context, input rewritePruneInput) error {
	if strings.TrimSpace(input.TargetRoundID) == "" {
		return nil
	}
	if strings.TrimSpace(input.ReplacementRoundID) == "" {
		return errors.New("replacement round id is required")
	}
	roundIDs := input.RoundIDs
	if len(roundIDs) == 0 {
		roundIDs = []string{input.TargetRoundID}
	}
	removed, err := s.history.ForOwner(authctx.OwnerUserID(ctx)).RemoveOverlayRounds(
		input.WorkspacePath,
		input.SessionKey,
		roundIDs,
	)
	if err != nil {
		s.loggerFor(ctx).Error("DM rewrite overlay 裁剪失败",
			"session_key", input.SessionKey,
			"target_round_id", input.TargetRoundID,
			"replacement_round_id", input.ReplacementRoundID,
			"round_ids", roundIDs,
			"err", err,
		)
	} else {
		s.loggerFor(ctx).Info("DM rewrite overlay 已裁剪",
			"session_key", input.SessionKey,
			"target_round_id", input.TargetRoundID,
			"replacement_round_id", input.ReplacementRoundID,
			"round_ids", roundIDs,
			"removed_overlay_rows", removed,
			"removed_message_count", input.RemoveMessageCount,
		)
	}
	return err
}

func (s *Service) broadcastHistoryRewriteResync(
	ctx context.Context,
	sessionKey string,
	targetRoundID string,
	replacementRoundID string,
) {
	event := protocol.NewEvent(protocol.EventTypeSessionResyncRequired, map[string]any{
		"reason":               "history_rewrite",
		"target_round_id":      strings.TrimSpace(targetRoundID),
		"replacement_round_id": strings.TrimSpace(replacementRoundID),
	})
	event.SessionKey = sessionKey
	s.loggerFor(ctx).Info("广播 DM rewrite 历史刷新",
		"session_key", sessionKey,
		"target_round_id", strings.TrimSpace(targetRoundID),
		"replacement_round_id", strings.TrimSpace(replacementRoundID),
	)
	s.broadcastEventWithTimeout(ctx, sessionKey, event)
}

func lastVisibleUserMessage(rows []protocol.Message) (protocol.Message, bool) {
	for index := len(rows) - 1; index >= 0; index-- {
		row := rows[index]
		if strings.TrimSpace(dmdomain.NormalizeString(row["role"])) == "user" {
			return row, true
		}
	}
	return nil, false
}

func countHistoryRowsForRound(rows []protocol.Message, roundID string) int {
	roundID = strings.TrimSpace(roundID)
	if roundID == "" {
		return 0
	}
	count := 0
	for _, row := range rows {
		if strings.TrimSpace(dmdomain.NormalizeString(row["round_id"])) == roundID {
			count++
		}
	}
	return count
}
