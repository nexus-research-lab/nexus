package dm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
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
		return err
	}
	if runningRoundIDs := s.runtime.GetRunningRoundIDs(sessionKey); len(runningRoundIDs) > 0 {
		return errors.New("cannot rewrite while a round is running")
	}

	agentID := dmdomain.FirstNonEmpty(parsed.AgentID, request.AgentID)
	if agentID == "" {
		defaultAgent, defaultErr := s.agents.GetDefaultAgent(ctx)
		if defaultErr != nil {
			return defaultErr
		}
		agentID = defaultAgent.AgentID
	}
	agentValue, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		return err
	}
	sessionItem, err := s.ensureSession(ctx, agentValue, parsed, sessionKey)
	if err != nil {
		return err
	}
	rows, err := s.history.ReadMessages(agentValue.WorkspacePath, sessionItem, nil)
	if err != nil {
		return err
	}
	lastUser, ok := lastVisibleUserMessage(rows)
	if !ok {
		return errors.New("cannot rewrite an empty conversation")
	}
	targetRoundID := strings.TrimSpace(request.TargetRoundID)
	if targetRoundID != strings.TrimSpace(dmdomain.NormalizeString(lastUser["round_id"])) {
		return fmt.Errorf("can only rewrite the last user message")
	}

	attachments := request.Attachments
	if len(attachments) == 0 {
		attachments = protocol.ChatAttachmentsFromAny(lastUser["attachments"])
	}
	historyContext := buildRewriteRuntimeHistoryContext(rows, targetRoundID)
	return s.HandleChat(ctx, Request{
		SessionKey:             sessionKey,
		AgentID:                agentID,
		Content:                request.Content,
		HistoryContextPrefix:   historyContext,
		Attachments:            attachments,
		RoundID:                protocol.NewRoundID(),
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
