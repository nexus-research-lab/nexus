package dm

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// HandleChat 处理一条 DM 写请求。
func (s *Service) HandleChat(ctx context.Context, request Request) error {
	sessionKey, parsed, err := s.validateRequest(request)
	if err != nil {
		return err
	}
	// round_id / user_message_id / agent_round_id 一律由后端 mint；
	// RoundID 允许后端内部调用方预置（automation / queue / goal）。
	if strings.TrimSpace(request.RoundID) == "" {
		request.RoundID = protocol.NewRoundID()
	}
	if strings.TrimSpace(request.UserMessageID) == "" {
		request.UserMessageID = protocol.NewUserMessageID()
	}
	if strings.TrimSpace(request.AgentRoundID) == "" {
		request.AgentRoundID = protocol.NewAgentRoundID()
	}
	agentID := dmdomain.FirstNonEmpty(parsed.AgentID, request.AgentID)
	if agentID == "" {
		defaultAgent, defaultErr := s.agents.GetDefaultAgent(ctx)
		if defaultErr != nil {
			return defaultErr
		}
		agentID = defaultAgent.AgentID
	}
	request.Attachments = s.normalizeChatAttachments(request.Attachments, agentID)

	agentValue, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		return err
	}

	sessionItem, err := s.ensureSession(ctx, agentValue, parsed, sessionKey)
	if err != nil {
		return err
	}
	initialMessageCount := sessionItem.MessageCount
	deliveryPolicy := protocol.NormalizeChatDeliveryPolicy(string(request.DeliveryPolicy))

	if !request.Internal && protocol.ShouldGuideRunningRound(deliveryPolicy) {
		delivered, guideErr := s.guideRunningInput(ctx, sessionKey, agentValue, sessionItem, request)
		if guideErr != nil && !errors.Is(guideErr, runtimectx.ErrNoRunningRound) {
			return guideErr
		}
		if delivered {
			return nil
		}
		// 引导只对已运行的 round 有意义；空闲时退化为普通新一轮，避免历史里出现假“已引导”用户消息。
		deliveryPolicy = protocol.ChatDeliveryPolicyQueue
	}

	if !request.Internal && protocol.ShouldQueueRunningRound(deliveryPolicy) {
		delivered, queueErr := s.queueRunningInput(ctx, sessionKey, agentValue, sessionItem, request, initialMessageCount)
		if queueErr != nil && !errors.Is(queueErr, runtimectx.ErrNoRunningRound) {
			return queueErr
		}
		if delivered {
			return nil
		}
	}

	if err = s.ensureQuotaAvailable(ctx); err != nil {
		return err
	}

	if !request.Internal && deliveryPolicy == protocol.ChatDeliveryPolicyInterrupt {
		if err = s.interruptSession(ctx, sessionKey, "收到新的用户消息，上一轮已停止"); err != nil {
			return err
		}
	}
	runtimeContent, err := s.renderRuntimeContentWithAttachments(ctx, request.Content, request.Attachments)
	if err != nil {
		return err
	}
	if agentValue != nil && !runtimeContent.IsEmpty() {
		runtimeContent = runtimeContent.AppendText(s.agents.BuildRuntimeUserMessageSuffixForContext(ctx, agentValue, "dm:"+strings.TrimSpace(sessionKey)))
	}

	client, runtimeKind, runtimeProvider, runtimeModel, goalIDForUsage, goalContext, permissionMode, err := s.ensureClient(ctx, sessionKey, agentValue, sessionItem, request)
	if err != nil {
		s.loggerFor(ctx).Error("DM runtime client 初始化失败",
			"session_key", sessionKey,
			"agent_id", agentID,
			"round_id", request.RoundID,
			"err", err,
		)
		return err
	}
	if override := strings.TrimSpace(request.GoalContext); request.Internal && override != "" {
		goalContext = override
	}
	if strings.TrimSpace(request.RewriteTargetRoundID) != "" && len(request.RewriteRemoveMessageUUIDs) == 0 {
		return errors.New("rewrite remove message uuids are required")
	}
	if len(request.RewriteRemoveMessageUUIDs) > 0 {
		if err = client.RemoveMessages(ctx, request.RewriteRemoveMessageUUIDs); err != nil {
			s.loggerFor(ctx).Error("DM rewrite 删除 runtime 历史失败",
				"session_key", sessionKey,
				"agent_id", agentID,
				"round_id", request.RoundID,
				"target_round_id", request.RewriteTargetRoundID,
				"message_uuid_count", len(request.RewriteRemoveMessageUUIDs),
				"err", err,
			)
			return err
		}
	}
	if err = s.pruneHistoryRewriteTail(ctx, rewritePruneInput{
		WorkspacePath:      agentValue.WorkspacePath,
		SessionKey:         sessionKey,
		TargetRoundID:      request.RewriteTargetRoundID,
		ReplacementRoundID: request.RoundID,
		RoundIDs:           request.RewriteRemoveRoundIDs,
		RemoveMessageCount: request.RewriteRemoveMessageCount,
	}); err != nil {
		if closeErr := s.runtime.CloseSession(ctx, sessionKey); closeErr != nil && !runtimectx.IsRuntimeTransportClosedError(closeErr) {
			s.loggerFor(ctx).Warn("DM rewrite overlay 裁剪失败后关闭 runtime 失败",
				"session_key", sessionKey,
				"agent_id", agentID,
				"round_id", request.RoundID,
				"err", closeErr,
			)
		}
		return err
	}
	if request.RewriteRemoveMessageCount > 0 {
		sessionItem.MessageCount -= request.RewriteRemoveMessageCount
		if sessionItem.MessageCount < 0 {
			sessionItem.MessageCount = 0
		}
		initialMessageCount = sessionItem.MessageCount
	}
	roundCtx, cancel := context.WithCancel(context.Background())
	s.runtime.StartRound(sessionKey, request.RoundID, cancel)
	s.permission.BindSessionRoute(sessionKey, permissionctx.RouteContext{
		DispatchSessionKey: sessionKey,
		AgentID:            agentID,
		RoundID:            request.RoundID,
		AgentRoundID:       request.AgentRoundID,
	})

	runner := &roundRunner{
		service:             s,
		workspacePath:       agentValue.WorkspacePath,
		session:             sessionItem,
		agent:               agentValue,
		sessionKey:          sessionKey,
		roundID:             request.RoundID,
		agentRoundID:        request.AgentRoundID,
		userMessageID:       request.UserMessageID,
		clientRequestID:     request.ClientRequestID,
		content:             strings.TrimSpace(request.Content),
		runtimeContent:      runtimeContent,
		client:              client,
		runtimeKind:         runtimeKind,
		runtimeProvider:     runtimeProvider,
		runtimeModel:        runtimeModel,
		ownerUserID:         authctx.OwnerUserID(ctx),
		mapper:              dmdomain.NewMessageMapper(sessionKey, agentID, request.RoundID, request.AgentRoundID, request.UserMessageID, agentValue.WorkspacePath),
		inputOptions:        request.InputOptions,
		internal:            request.Internal,
		externalReplyTarget: request.ExternalReplyTarget,
		goalContext:         goalContext,
		goalIDForUsage:      goalIDForUsage,
		goalUsage:           goalsvc.NewRuntimeUsageAccumulator(strings.TrimSpace(goalIDForUsage) != ""),
		goalUsageStarted:    time.Now(),
		permissionMode:      permissionMode,
		permissionHandler:   request.PermissionHandler,
	}
	s.runtime.RegisterGoalAccountingFlush(sessionKey, request.RoundID, runner.flushGoalUsage)
	s.runtime.RegisterGoalAccountingClear(sessionKey, request.RoundID, runner.clearGoalUsage)
	s.runtime.RegisterGoalAccountingActivate(sessionKey, request.RoundID, runner.activateGoalUsage)

	s.loggerFor(ctx).Info("受理 DM 会话消息",
		"session_key", sessionKey,
		"agent_id", agentID,
		"round_id", request.RoundID,
		"client_request_id", runner.clientRequestID,
		"content_chars", utf8.RuneCountInString(runner.content),
		"content_preview", logx.PreviewText(runner.content, 240),
		"attachment_count", len(request.Attachments),
	)

	markerOptions := workspacestore.RoundMarkerOptions{
		UserMessageID:  request.UserMessageID,
		AgentRoundID:   request.AgentRoundID,
		DeliveryPolicy: string(deliveryPolicy),
		Attachments:    request.Attachments,
		HiddenFromUser: request.Internal || request.InputOptions.HiddenFromUser,
		Synthetic:      request.InputOptions.Synthetic,
		Purpose:        request.InputOptions.Purpose,
		Metadata:       request.InputOptions.Metadata,
	}
	if request.Internal {
		markerOptions.Synthetic = true
	}
	if err = s.recordRoundMarkerWithOptions(runner.workspacePath, runner.session, runner.roundID, runner.content, markerOptions); err != nil {
		s.runtime.MarkRoundFinished(sessionKey, request.RoundID)
		if closeErr := s.refreshSessionMetaRuntimeStateByKey(ctx, sessionKey); closeErr != nil {
			s.loggerFor(ctx).Warn("DM 轮次标记失败后刷新 session meta 失败",
				"session_key", sessionKey,
				"agent_id", agentID,
				"round_id", request.RoundID,
				"err", closeErr,
			)
		}
		s.permission.CancelRequestsForSession(sessionKey, "轮次标记持久化失败")
		s.loggerFor(ctx).Error("DM 轮次标记持久化失败",
			"session_key", sessionKey,
			"agent_id", agentID,
			"round_id", request.RoundID,
			"err", err,
		)
		return err
	}

	var (
		updatedSession *protocol.Session
		syncErr        error
	)
	if !request.Internal {
		updatedSession, syncErr = s.refreshSessionMetaAfterRoundMarker(runner.workspacePath, runner.session)
	}
	if syncErr != nil {
		s.runtime.MarkRoundFinished(sessionKey, request.RoundID)
		if closeErr := s.refreshSessionMetaRuntimeStateByKey(ctx, sessionKey); closeErr != nil {
			s.loggerFor(ctx).Warn("DM 轮次元数据失败后刷新 session meta 失败",
				"session_key", sessionKey,
				"agent_id", agentID,
				"round_id", request.RoundID,
				"err", closeErr,
			)
		}
		s.permission.CancelRequestsForSession(sessionKey, "会话元数据持久化失败")
		s.loggerFor(ctx).Error("DM 轮次元数据持久化失败",
			"session_key", sessionKey,
			"agent_id", agentID,
			"round_id", request.RoundID,
			"err", syncErr,
		)
		return syncErr
	} else if updatedSession != nil {
		runner.session = *updatedSession
	}

	if !request.Internal {
		s.scheduleTitleGeneration(ctx, parsed, runner.session, runner.content, initialMessageCount, runtimeProvider, runtimeModel)
	}

	if !request.Internal {
		s.broadcastEventWithTimeout(ctx, sessionKey, protocol.NewChatAckEvent(
			sessionKey,
			request.ClientRequestID,
			request.ClientMessageID,
			request.RoundID,
			request.UserMessageID,
			dmChatAckPendingSlots(agentID, request.AgentRoundID),
		))
	}
	if request.BroadcastUserMessage {
		s.broadcastUserRoundMarker(ctx, runner.session, runner.roundID, request.UserMessageID, runner.content, deliveryPolicy, request.Attachments)
	}
	if strings.TrimSpace(request.RewriteTargetRoundID) != "" {
		s.broadcastHistoryRewriteResync(ctx, sessionKey, request.RewriteTargetRoundID, request.RoundID)
	}
	s.broadcastEventWithTimeout(ctx, sessionKey, protocol.NewRoundStatusEvent(sessionKey, request.RoundID, "running", ""))
	s.broadcastSessionStatus(ctx, sessionKey)

	go runner.run(roundCtx)
	return nil
}

// dmChatAckPendingSlots 构造 DM chat_ack 的单 slot 占位，DM 与 Room 共用 agent_slots 语义。
func dmChatAckPendingSlots(agentID string, agentRoundID string) []protocol.ChatAckPendingSlot {
	return []protocol.ChatAckPendingSlot{{
		AgentID:      agentID,
		AgentRoundID: agentRoundID,
		MsgID:        agentRoundID,
		Status:       "pending",
		Timestamp:    time.Now().UnixMilli(),
		Index:        0,
	}}
}

func (s *Service) validateRequest(request Request) (string, protocol.SessionKey, error) {
	sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
	if err != nil {
		return "", protocol.SessionKey{}, err
	}
	if !protocol.HasChatInput(request.Content, request.Attachments) &&
		!(request.Internal && strings.TrimSpace(request.GoalContext) != "") {
		return "", protocol.SessionKey{}, errors.New("content is required")
	}

	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return "", protocol.SessionKey{}, ErrRoomSessionNotImplemented
	}
	return sessionKey, parsed, nil
}
