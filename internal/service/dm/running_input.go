// INPUT: 运行中 DM round 与用户 queue/guide 输入。
// OUTPUT: queue 先持久保留再等待可信 admission/下一轮，guide 等 runtime applied ACK 后归入实际消费它的 root round。
// POS: DM 轮内插话的唯一受理入口。
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
	request Request,
) (bool, error) {
	content := strings.TrimSpace(request.Content)
	attachments := s.normalizeChatAttachments(request.Attachments, agentValue.AgentID)
	runningRoundIDs := s.runtime.GetRunningRoundIDs(sessionKey)
	if len(runningRoundIDs) == 0 {
		return false, runtimectx.ErrNoRunningRound
	}
	location := workspacestore.InputQueueLocation{
		OwnerUserID:   agentValue.OwnerUserID,
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: agentValue.WorkspacePath,
		SessionKey:    sessionKey,
	}
	acceptedItem, items, err := s.enqueueRunningInput(location, protocol.InputQueueItem{
		ID:              request.RoundID,
		Scope:           protocol.InputQueueScopeDM,
		SessionKey:      sessionKey,
		AgentID:         agentValue.AgentID,
		AgentRoundID:    request.AgentRoundID,
		ClientMessageID: request.ClientMessageID,
		SourceMessageID: request.UserMessageID,
		Source:          protocol.InputQueueSourceUser,
		Content:         content,
		Attachments:     attachments,
		DeliveryPolicy:  protocol.ChatDeliveryPolicyQueue,
		OwnerUserID:     agentValue.OwnerUserID,
	}, request.ClientMessageID)
	if err != nil {
		return false, err
	}
	request.RoundID = acceptedItem.ID
	request.UserMessageID = acceptedItem.SourceMessageID
	request.AgentRoundID = acceptedItem.AgentRoundID
	if err = s.recordTrustedQueueAdmission(
		ctx,
		location,
		acceptedItem,
		request.TrustedConfigurationContext,
	); err != nil {
		return false, err
	}
	s.broadcastInputQueueSnapshot(ctx, sessionKey, items)
	s.broadcastEventWithTimeout(ctx, sessionKey, protocol.NewChatAckEvent(
		sessionKey,
		request.ClientRequestID,
		request.ClientMessageID,
		request.RoundID,
		request.UserMessageID,
		false,
		nil,
	))
	s.broadcastSessionStatus(ctx, sessionKey)
	s.loggerFor(ctx).Info("持久化 DM 消息等待当前 round 结束后接力",
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
		OwnerUserID:   agentValue.OwnerUserID,
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: agentValue.WorkspacePath,
		SessionKey:    sessionKey,
	}
	acceptedItem, items, err := s.enqueueRunningInput(location, protocol.InputQueueItem{
		ID:              request.RoundID,
		Scope:           protocol.InputQueueScopeDM,
		SessionKey:      sessionKey,
		AgentID:         agentValue.AgentID,
		AgentRoundID:    request.AgentRoundID,
		ClientMessageID: request.ClientMessageID,
		SourceMessageID: request.UserMessageID,
		Source:          protocol.InputQueueSourceUser,
		Content:         content,
		Attachments:     attachments,
		DeliveryPolicy:  protocol.ChatDeliveryPolicyGuide,
		OwnerUserID:     agentValue.OwnerUserID,
		RootRoundID:     targetRoundID,
	}, request.ClientMessageID)
	if err != nil {
		return false, err
	}
	request.RoundID = acceptedItem.ID
	request.UserMessageID = acceptedItem.SourceMessageID
	request.AgentRoundID = acceptedItem.AgentRoundID
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
		false,
		nil,
	))
	s.broadcastSessionStatus(ctx, sessionKey)
	if recovered {
		s.startSessionBackgroundTask(sessionKey, location.OwnerUserID, func(taskCtx context.Context) {
			s.dispatchNextInputQueueItemAtLocation(taskCtx, sessionKey, agentValue.AgentID, location)
		})
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

func (s *Service) enqueueRunningInput(
	location workspacestore.InputQueueLocation,
	item protocol.InputQueueItem,
	clientMessageID string,
) (protocol.InputQueueItem, []protocol.InputQueueItem, error) {
	if strings.TrimSpace(clientMessageID) == "" {
		items, err := s.inputQueue.Enqueue(location, item)
		return item, items, err
	}
	result, err := s.inputQueue.EnqueueIdempotent(location, item, clientMessageID)
	if err != nil {
		return protocol.InputQueueItem{}, nil, err
	}
	return result.Item, result.Items, nil
}
