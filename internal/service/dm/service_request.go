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

	if !request.Internal && deliveryPolicy == protocol.ChatDeliveryPolicyInterrupt {
		if err = s.interruptSession(ctx, sessionKey, "收到新的用户消息，上一轮已停止"); err != nil {
			return err
		}
	}
	runtimeContent, err := s.renderRuntimeContentWithAttachments(ctx, request.Content, request.Attachments)
	if err != nil {
		return err
	}
	runtimeContent = s.injectMemoryContext(ctx, agentValue, sessionItem, sessionKey, request.Content, runtimeContent)
	runtimeContent = s.appendRuntimeUserContext(ctx, sessionKey, agentValue, runtimeContent)

	client, runtimeProvider, runtimeModel, goalIDForUsage, goalContext, permissionMode, err := s.ensureClient(ctx, sessionKey, agentValue, sessionItem, request)
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
	if updatedSession, syncErr := s.syncSDKSessionID(ctx, agentValue.WorkspacePath, sessionItem, client.SessionID(), runtimeProvider, runtimeModel); syncErr != nil {
		return syncErr
	} else {
		sessionItem = updatedSession
	}

	roundCtx, cancel := context.WithCancel(context.Background())
	s.runtime.StartRound(sessionKey, request.RoundID, cancel)
	s.permission.BindSessionRoute(sessionKey, permissionctx.RouteContext{
		DispatchSessionKey: sessionKey,
		AgentID:            agentID,
		CausedBy:           request.RoundID,
	})

	runner := &roundRunner{
		service:           s,
		workspacePath:     agentValue.WorkspacePath,
		session:           sessionItem,
		agent:             agentValue,
		sessionKey:        sessionKey,
		roundID:           request.RoundID,
		reqID:             dmdomain.FirstNonEmpty(request.ReqID, request.RoundID),
		content:           strings.TrimSpace(request.Content),
		runtimeContent:    runtimeContent,
		client:            client,
		runtimeProvider:   runtimeProvider,
		runtimeModel:      runtimeModel,
		ownerUserID:       authctx.OwnerUserID(ctx),
		mapper:            dmdomain.NewMessageMapper(sessionKey, agentID, request.RoundID, agentValue.WorkspacePath),
		inputOptions:      request.InputOptions,
		internal:          request.Internal,
		goalContext:       goalContext,
		goalIDForUsage:    goalIDForUsage,
		goalUsage:         goalsvc.NewRuntimeUsageAccumulator(strings.TrimSpace(goalIDForUsage) != ""),
		goalUsageStarted:  time.Now(),
		permissionMode:    permissionMode,
		permissionHandler: request.PermissionHandler,
	}
	s.runtime.RegisterGoalAccountingFlush(sessionKey, request.RoundID, runner.flushGoalUsage)
	s.runtime.RegisterGoalAccountingClear(sessionKey, request.RoundID, runner.clearGoalUsage)
	s.runtime.RegisterGoalAccountingActivate(sessionKey, request.RoundID, runner.activateGoalUsage)

	s.loggerFor(ctx).Info("受理 DM 会话消息",
		"session_key", sessionKey,
		"agent_id", agentID,
		"round_id", request.RoundID,
		"req_id", runner.reqID,
		"content_chars", utf8.RuneCountInString(runner.content),
		"content_preview", logx.PreviewText(runner.content, 240),
		"attachment_count", len(request.Attachments),
	)

	markerOptions := workspacestore.RoundMarkerOptions{
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
		s.broadcastEventWithTimeout(ctx, sessionKey, protocol.NewChatAckEvent(sessionKey, runner.reqID, request.RoundID, []map[string]any{}))
	}
	if request.BroadcastUserMessage {
		s.broadcastUserRoundMarker(ctx, runner.session, runner.roundID, runner.content, deliveryPolicy, request.Attachments)
	}
	s.broadcastEventWithTimeout(ctx, sessionKey, protocol.NewRoundStatusEvent(sessionKey, request.RoundID, "running", ""))
	s.broadcastSessionStatus(ctx, sessionKey)

	go runner.run(roundCtx)
	return nil
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
	if strings.TrimSpace(request.RoundID) == "" {
		return "", protocol.SessionKey{}, errors.New("round_id is required")
	}

	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return "", protocol.SessionKey{}, ErrRoomSessionNotImplemented
	}
	return sessionKey, parsed, nil
}
