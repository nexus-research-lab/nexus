// INPUT: 运行中 DM round 与用户 queue/guide 输入。
// OUTPUT: 注入当前 runtime，并把 durable 用户输入归入实际消费它的 root round。
// POS: DM 轮内插话的唯一受理入口。
package dm

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
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
	runtimeContent, err := s.renderRuntimeContentWithAttachments(ctx, content, attachments)
	if err != nil {
		return false, err
	}
	// 轮内注入不带 runtime context（情绪态）：避免逐步污染 prompt 前缀缓存。
	runningRoundIDs, err := s.runtime.SendContentToRunningRound(ctx, sessionKey, runtimeContent.Payload())
	if err != nil {
		return false, err
	}
	if len(runningRoundIDs) == 0 {
		return false, runtimectx.ErrNoRunningRound
	}
	// 一个 DM runtime 同时只承载一个顶层 round；使用投递瞬间返回的 id，避免预读运行态的竞态。
	targetRoundID := strings.TrimSpace(runningRoundIDs[0])
	if targetRoundID == "" {
		return false, runtimectx.ErrNoRunningRound
	}
	if err := s.recordRoundMarkerWithOptions(agentValue.WorkspacePath, sessionItem, targetRoundID, content, workspacestore.RoundMarkerOptions{
		UserMessageID:  request.UserMessageID,
		AgentRoundID:   request.AgentRoundID,
		SourceRoundID:  request.RoundID,
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
		s.broadcastUserRoundMarker(ctx, sessionItem, targetRoundID, request.RoundID, request.UserMessageID, content, protocol.ChatDeliveryPolicyQueue, attachments)
	}
	s.broadcastSessionStatus(ctx, sessionKey)
	s.loggerFor(ctx).Info("排队 DM 消息到运行中 round",
		"session_key", sessionKey,
		"agent_id", agentValue.AgentID,
		"round_id", request.RoundID,
		"target_round_id", targetRoundID,
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
	request Request,
) (bool, error) {
	content := strings.TrimSpace(request.Content)
	attachments := s.normalizeChatAttachments(request.Attachments, agentValue.AgentID)
	runningRoundIDs := s.runtime.GetRunningRoundIDs(sessionKey)
	if len(runningRoundIDs) == 0 {
		return false, runtimectx.ErrNoRunningRound
	}
	targetRoundID := strings.TrimSpace(runningRoundIDs[0])
	if targetRoundID == "" {
		return false, runtimectx.ErrNoRunningRound
	}
	location := workspacestore.InputQueueLocation{
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: agentValue.WorkspacePath,
		SessionKey:    sessionKey,
	}
	items, err := s.inputQueue.Enqueue(location, protocol.InputQueueItem{
		ID:              request.RoundID,
		Scope:           protocol.InputQueueScopeDM,
		SessionKey:      sessionKey,
		AgentID:         agentValue.AgentID,
		SourceMessageID: request.UserMessageID,
		Source:          protocol.InputQueueSourceUser,
		Content:         content,
		Attachments:     attachments,
		DeliveryPolicy:  protocol.ChatDeliveryPolicyGuide,
		OwnerUserID:     authctx.OwnerUserID(ctx),
		RootRoundID:     targetRoundID,
	})
	if err != nil {
		return false, err
	}
	items, recovered, err := s.recoverStaleInputQueueGuidance(location, request.RoundID, targetRoundID, items)
	if err != nil {
		return false, err
	}
	s.broadcastInputQueueSnapshot(ctx, sessionKey, items)
	s.broadcastEventWithTimeout(ctx, sessionKey, protocol.NewChatAckEvent(
		sessionKey,
		request.ClientRequestID,
		request.ClientMessageID,
		request.RoundID,
		request.UserMessageID,
		nil,
	))
	s.broadcastSessionStatus(ctx, sessionKey)
	if recovered {
		go s.dispatchNextInputQueueItemAtLocation(
			contextWithQueueOwner(context.Background(), authctx.OwnerUserID(ctx)),
			sessionKey,
			agentValue.AgentID,
			location,
		)
	}
	s.loggerFor(ctx).Info("登记 DM 引导消息等待 PostToolUse 注入",
		"session_key", sessionKey,
		"agent_id", agentValue.AgentID,
		"round_id", request.RoundID,
		"target_round_id", targetRoundID,
		"recovered_to_queue", recovered,
		"running_round_ids", runningRoundIDs,
		"content_chars", utf8.RuneCountInString(content),
		"content_preview", logx.PreviewText(content, 240),
	)
	return true, nil
}
