package dm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

const rewriteHistoryContextMaxChars = 16000

type runnerRewriteInput struct {
	WorkspacePath      string
	SessionKey         string
	TargetRoundID      string
	ReplacementRoundID string
	Content            string
	Attachments        []protocol.ChatAttachment
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
	rows, err := s.history.ReadMessages(agentValue.WorkspacePath, sessionItem, nil)
	if err != nil {
		logger.Warn("DM rewrite 读取历史失败", "workspace_path", agentValue.WorkspacePath, "err", err)
		return err
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
	historyContext := buildRewriteRuntimeHistoryContext(rows, targetRoundID)
	replacementRoundID := protocol.NewRoundID()
	logger.Info("准备重跑 DM rewrite",
		"replacement_round_id", replacementRoundID,
		"history_context_chars", utf8.RuneCountInString(historyContext),
		"source_row_count", len(rows),
		"attachment_count", len(attachments),
	)
	return s.HandleChat(ctx, Request{
		SessionKey:             sessionKey,
		AgentID:                agentID,
		Content:                request.Content,
		HistoryContextPrefix:   historyContext,
		Attachments:            attachments,
		RoundID:                replacementRoundID,
		ClientRequestID:        request.ClientRequestID,
		ClientMessageID:        request.ClientMessageID,
		DeliveryPolicy:         protocol.ChatDeliveryPolicyQueue,
		BroadcastUserMessage:   true,
		ForceNewRuntimeSession: true,
		RewriteTargetRoundID:   targetRoundID,
	})
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

func (s *Service) recordHistoryRewrite(ctx context.Context, input runnerRewriteInput) error {
	if strings.TrimSpace(input.TargetRoundID) == "" {
		return nil
	}
	if strings.TrimSpace(input.ReplacementRoundID) == "" {
		return errors.New("replacement round id is required")
	}
	err := s.history.AppendHistoryRewrite(input.WorkspacePath, input.SessionKey, workspacestore.HistoryRewriteOptions{
		TargetRoundID:      input.TargetRoundID,
		ReplacementRoundID: input.ReplacementRoundID,
		Content:            input.Content,
		Attachments:        input.Attachments,
	})
	if err != nil {
		s.loggerFor(ctx).Error("DM history rewrite marker 写入失败",
			"session_key", input.SessionKey,
			"target_round_id", input.TargetRoundID,
			"replacement_round_id", input.ReplacementRoundID,
			"err", err,
		)
	} else {
		s.loggerFor(ctx).Info("DM history rewrite marker 已写入",
			"session_key", input.SessionKey,
			"target_round_id", input.TargetRoundID,
			"replacement_round_id", input.ReplacementRoundID,
			"content_chars", utf8.RuneCountInString(strings.TrimSpace(input.Content)),
			"attachment_count", len(input.Attachments),
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

func buildRewriteRuntimeHistoryContext(rows []protocol.Message, targetRoundID string) string {
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(dmdomain.NormalizeString(row["round_id"])) == targetRoundID {
			break
		}
		line := rewriteHistoryLine(row)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return ""
	}
	body := strings.Join(lines, "\n\n")
	if len([]rune(body)) > rewriteHistoryContextMaxChars {
		body = trimLeftRunes(body, rewriteHistoryContextMaxChars)
	}
	return strings.TrimSpace("<nexus_history_context>\n以下是用户编辑最后一条消息后仍然有效的历史上下文。被编辑的旧消息和旧回复不再有效，请只基于这里的历史和本轮新消息继续。\n\n" + body + "\n</nexus_history_context>")
}

func rewriteHistoryLine(row protocol.Message) string {
	role := strings.TrimSpace(dmdomain.NormalizeString(row["role"]))
	switch role {
	case "user":
		if content := strings.TrimSpace(dmdomain.NormalizeString(row["content"])); content != "" {
			return "用户: " + content
		}
	case "assistant":
		if content := strings.TrimSpace(messageutil.ExtractAssistantDisplayText(row)); content != "" {
			return "助手: " + content
		}
	case "result":
		if result := strings.TrimSpace(dmdomain.NormalizeString(row["result"])); result != "" {
			return "结果: " + result
		}
	}
	return ""
}

func trimLeftRunes(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return "...\n" + string(runes[len(runes)-maxRunes:])
}
