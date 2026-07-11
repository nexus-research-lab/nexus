package dm

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *Service) queueRunningInput(
	ctx context.Context,
	sessionKey string,
	agentValue *protocol.Agent,
	sessionItem protocol.Session,
	request Request,
	initialMessageCount int,
) (bool, error) {
	content := strings.TrimSpace(request.Content)
	attachments := s.normalizeChatAttachments(request.Attachments, agentValue.AgentID)
	runningRoundIDs := s.runtime.GetRunningRoundIDs(sessionKey)
	if len(runningRoundIDs) == 0 {
		return false, runtimectx.ErrNoRunningRound
	}
	runtimeContent, err := s.renderRuntimeContentWithAttachments(ctx, content, attachments)
	if err != nil {
		return false, err
	}
	// 轮内注入不带 runtime context（情绪态）：避免逐步污染 prompt 前缀缓存。
	if _, err := s.runtime.SendContentToRunningRound(ctx, sessionKey, runtimeContent.Payload()); err != nil {
		return false, err
	}
	if err := s.recordRoundMarkerWithOptions(agentValue.WorkspacePath, sessionItem, request.RoundID, content, workspacestore.RoundMarkerOptions{
		UserMessageID:  request.UserMessageID,
		AgentRoundID:   request.AgentRoundID,
		DeliveryPolicy: string(protocol.ChatDeliveryPolicyQueue),
		Attachments:    attachments,
	}); err != nil {
		s.loggerFor(ctx).Error("DM 排队消息持久化失败",
			"session_key", sessionKey,
			"agent_id", agentValue.AgentID,
			"round_id", request.RoundID,
			"err", err,
		)
		return false, err
	}
	if _, err := s.refreshSessionMetaAfterRoundMarker(agentValue.WorkspacePath, sessionItem); err != nil {
		s.loggerFor(ctx).Error("DM 排队消息刷新 session meta 失败",
			"session_key", sessionKey,
			"agent_id", agentValue.AgentID,
			"round_id", request.RoundID,
			"err", err,
		)
		return false, err
	}
	runtimeProvider, runtimeModel := runtimeSelectionFromSession(sessionItem)
	s.scheduleTitleGeneration(ctx, protocol.ParseSessionKey(sessionKey), sessionItem, content, initialMessageCount, runtimeProvider, runtimeModel)
	s.broadcastEventWithTimeout(ctx, sessionKey, protocol.NewChatAckEvent(
		sessionKey,
		request.ClientRequestID,
		request.ClientMessageID,
		request.RoundID,
		request.UserMessageID,
		nil,
	))
	if request.BroadcastUserMessage {
		s.broadcastUserRoundMarker(ctx, sessionItem, request.RoundID, request.UserMessageID, content, protocol.ChatDeliveryPolicyQueue, attachments)
	}
	s.broadcastSessionStatus(ctx, sessionKey)
	s.loggerFor(ctx).Info("排队 DM 消息到运行中 round",
		"session_key", sessionKey,
		"agent_id", agentValue.AgentID,
		"round_id", request.RoundID,
		"running_round_ids", runningRoundIDs,
		"content_chars", utf8.RuneCountInString(content),
		"content_preview", logx.PreviewText(content, 240),
	)
	return true, nil
}

func (s *Service) guideRunningInput(
	ctx context.Context,
	sessionKey string,
	agentValue *protocol.Agent,
	sessionItem protocol.Session,
	request Request,
) (bool, error) {
	content := strings.TrimSpace(request.Content)
	attachments := s.normalizeChatAttachments(request.Attachments, agentValue.AgentID)
	runtimeContent, err := s.renderRuntimeContentWithAttachments(ctx, content, attachments)
	if err != nil {
		return false, err
	}
	// 轮内引导注入不带 runtime context（情绪态）：避免逐步污染 prompt 前缀缓存。
	runningRoundIDs, err := s.runtime.QueueGuidanceInput(ctx, sessionKey, request.RoundID, runtimeContent.PlainText())
	if err != nil {
		return false, err
	}
	s.broadcastEventWithTimeout(ctx, sessionKey, protocol.NewChatAckEvent(
		sessionKey,
		request.ClientRequestID,
		request.ClientMessageID,
		request.RoundID,
		request.UserMessageID,
		nil,
	))
	if request.BroadcastUserMessage {
		for _, targetRoundID := range runningRoundIDs {
			s.broadcastGuidanceMessage(ctx, sessionItem, targetRoundID, request.RoundID, content)
		}
	}
	s.broadcastSessionStatus(ctx, sessionKey)
	s.loggerFor(ctx).Info("登记 DM 引导消息等待 PostToolUse 注入",
		"session_key", sessionKey,
		"agent_id", agentValue.AgentID,
		"round_id", request.RoundID,
		"running_round_ids", runningRoundIDs,
		"content_chars", utf8.RuneCountInString(content),
		"content_preview", logx.PreviewText(content, 240),
	)
	return true, nil
}
