// INPUT: DM 用户请求、内部 Goal 续跑与当前会话运行态。
// OUTPUT: 恰好一次的运行中投递、持久队列登记或新 round 启动。
// POS: DM 输入受理与 runtime 启动的串行交接边界。
package dm

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// HandleChat 处理一条 DM 写请求。显式输入与队列交接、Goal 续跑共享同一个启动边界。
func (s *Service) HandleChat(ctx context.Context, request Request) error {
	s.inputQueueDispatchMu.Lock()
	defer s.inputQueueDispatchMu.Unlock()
	return s.handleChat(ctx, request)
}

// handleChat 要求调用方已为显式输入持有 inputQueueDispatchMu；内部输入由各调度器自行保证互斥。
func (s *Service) handleChat(ctx context.Context, request Request) error {
	execution, err := s.prepareChatExecution(ctx, request)
	if err != nil {
		return err
	}
	if handled, routeErr := execution.routeRunningInput(); handled || routeErr != nil {
		return routeErr
	}
	if err = execution.prepareRunner(); err != nil {
		return err
	}
	if err = execution.persistRound(); err != nil {
		return err
	}
	execution.launch()
	return nil
}

// dmChatExecution 聚合单次写请求在各业务阶段共享的状态，避免用参数组串联编排链路。
type dmChatExecution struct {
	service             *Service
	ctx                 context.Context
	request             Request
	sessionKey          string
	parsed              protocol.SessionKey
	agent               *protocol.Agent
	session             protocol.Session
	initialMessageCount int
	deliveryPolicy      protocol.ChatDeliveryPolicy
	runner              *roundRunner
	roundCtx            context.Context
}

type dmRuntimePreparation struct {
	content               conversationsvc.RuntimeContent
	client                runtimectx.Client
	runtimeKind           string
	runtimeProvider       string
	runtimeModel          string
	goalIDForUsage        string
	goalContext           string
	goalObjectiveRevision *atomic.Int64
	permissionMode        sdkpermission.Mode
}

func (s *Service) prepareChatExecution(ctx context.Context, request Request) (*dmChatExecution, error) {
	sessionKey, parsed, err := s.validateRequest(request)
	if err != nil {
		return nil, err
	}
	// 所有入口共享后端 ID 生成规则；内部调度可预置 round_id 以维持续跑关联。
	if strings.TrimSpace(request.RoundID) == "" {
		request.RoundID = protocol.NewRoundID()
	}
	if strings.TrimSpace(request.UserMessageID) == "" {
		request.UserMessageID = protocol.NewUserMessageID()
	}
	if strings.TrimSpace(request.AgentRoundID) == "" {
		request.AgentRoundID = protocol.NewAgentRoundID()
	}
	agentID, err := s.resolveChatAgentID(ctx, parsed, request.AgentID)
	if err != nil {
		return nil, err
	}
	request.Attachments = s.normalizeChatAttachments(request.Attachments, agentID)
	agentValue, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	sessionItem, err := s.ensureSession(ctx, agentValue, parsed, sessionKey)
	if err != nil {
		return nil, err
	}
	return &dmChatExecution{
		service:             s,
		ctx:                 ctx,
		request:             request,
		sessionKey:          sessionKey,
		parsed:              parsed,
		agent:               agentValue,
		session:             sessionItem,
		initialMessageCount: sessionItem.MessageCount,
		deliveryPolicy:      protocol.NormalizeChatDeliveryPolicy(string(request.DeliveryPolicy)),
	}, nil
}

func (s *Service) resolveChatAgentID(ctx context.Context, parsed protocol.SessionKey, requestedAgentID string) (string, error) {
	if agentID := dmdomain.FirstNonEmpty(parsed.AgentID, requestedAgentID); agentID != "" {
		return agentID, nil
	}
	defaultAgent, err := s.agents.GetDefaultAgent(ctx)
	if err != nil {
		return "", err
	}
	return defaultAgent.AgentID, nil
}

func (e *dmChatExecution) routeRunningInput() (bool, error) {
	if e.request.Internal {
		return false, nil
	}
	if protocol.ShouldGuideRunningRound(e.deliveryPolicy) {
		delivered, guideErr := e.service.guideRunningInput(e.ctx, e.sessionKey, e.agent, e.request)
		if guideErr != nil && !errors.Is(guideErr, runtimectx.ErrNoRunningRound) {
			return false, guideErr
		}
		if delivered {
			return true, nil
		}
		// 引导只对已运行的 round 有意义；空闲时退化为普通新一轮，避免历史里出现假“已引导”用户消息。
		e.deliveryPolicy = protocol.ChatDeliveryPolicyQueue
	}
	if protocol.ShouldQueueRunningRound(e.deliveryPolicy) {
		delivered, queueErr := e.service.queueRunningInput(
			e.ctx,
			e.sessionKey,
			e.agent,
			e.request,
		)
		if queueErr != nil && !errors.Is(queueErr, runtimectx.ErrNoRunningRound) {
			return false, queueErr
		}
		if delivered {
			return true, nil
		}
	}
	return false, nil
}

func (e *dmChatExecution) prepareRunner() error {
	if err := e.service.ensureQuotaAvailable(e.ctx); err != nil {
		return err
	}
	if err := e.interruptRunningRound(); err != nil {
		return err
	}
	preparation, err := e.prepareRuntime()
	if err != nil {
		return err
	}
	e.startRound()
	e.runner = e.newRoundRunner(preparation)
	e.registerRunner()
	return nil
}

func (e *dmChatExecution) interruptRunningRound() error {
	if e.request.Internal || e.deliveryPolicy != protocol.ChatDeliveryPolicyInterrupt {
		return nil
	}
	return e.service.interruptSession(e.ctx, e.sessionKey, "收到新的用户消息，上一轮已停止")
}

func (e *dmChatExecution) prepareRuntime() (dmRuntimePreparation, error) {
	runtimeContent, err := e.service.renderRuntimeContentWithAttachments(e.ctx, e.request.Content, e.request.Attachments)
	if err != nil {
		return dmRuntimePreparation{}, err
	}
	if !runtimeContent.IsEmpty() {
		runtimeContent = runtimeContent.AppendText(e.service.agents.BuildRuntimeUserMessageSuffixForContext(
			e.ctx,
			e.agent,
			"dm:"+strings.TrimSpace(e.sessionKey),
		))
	}
	client, runtimeKind, runtimeProvider, runtimeModel, goalIDForUsage, goalContext, goalObjectiveRevision, permissionMode, err := e.service.ensureClient(
		e.ctx,
		e.sessionKey,
		e.agent,
		e.session,
		e.request,
	)
	if err != nil {
		e.service.loggerFor(e.ctx).Error("DM runtime client 初始化失败",
			"session_key", e.sessionKey,
			"agent_id", e.agent.AgentID,
			"round_id", e.request.RoundID,
			"err", err,
		)
		return dmRuntimePreparation{}, err
	}
	if override := strings.TrimSpace(e.request.GoalContext); e.request.Internal && override != "" {
		goalContext = override
		if goalID := strings.TrimSpace(e.request.GoalID); goalID != "" {
			goalIDForUsage = goalID
		}
	}
	if err = e.applyHistoryRewrite(client); err != nil {
		return dmRuntimePreparation{}, err
	}
	return dmRuntimePreparation{
		content:               runtimeContent,
		client:                client,
		runtimeKind:           runtimeKind,
		runtimeProvider:       runtimeProvider,
		runtimeModel:          runtimeModel,
		goalIDForUsage:        goalIDForUsage,
		goalContext:           goalContext,
		goalObjectiveRevision: goalObjectiveRevision,
		permissionMode:        permissionMode,
	}, nil
}

func (e *dmChatExecution) newRoundRunner(preparation dmRuntimePreparation) *roundRunner {
	return &roundRunner{
		service:               e.service,
		workspacePath:         e.agent.WorkspacePath,
		session:               e.session,
		agent:                 e.agent,
		sessionKey:            e.sessionKey,
		roundID:               e.request.RoundID,
		agentRoundID:          e.request.AgentRoundID,
		userMessageID:         e.request.UserMessageID,
		clientRequestID:       e.request.ClientRequestID,
		content:               strings.TrimSpace(e.request.Content),
		runtimeContent:        preparation.content,
		client:                preparation.client,
		runtimeKind:           preparation.runtimeKind,
		runtimeProvider:       preparation.runtimeProvider,
		runtimeModel:          preparation.runtimeModel,
		ownerUserID:           authctx.OwnerUserID(e.ctx),
		mapper:                dmdomain.NewMessageMapper(e.sessionKey, e.agent.AgentID, e.request.RoundID, e.request.AgentRoundID, e.request.UserMessageID, e.agent.WorkspacePath),
		inputOptions:          e.request.InputOptions,
		internal:              e.request.Internal,
		externalReplyTarget:   e.request.ExternalReplyTarget,
		goalContext:           preparation.goalContext,
		goalIDForUsage:        preparation.goalIDForUsage,
		goalObjectiveRevision: preparation.goalObjectiveRevision,
		goalUsage:             goalsvc.NewRuntimeUsageAccumulator(strings.TrimSpace(preparation.goalIDForUsage) != ""),
		goalUsageStarted:      time.Now(),
		permissionMode:        preparation.permissionMode,
		permissionHandler:     e.request.PermissionHandler,
	}
}

func (e *dmChatExecution) applyHistoryRewrite(client runtimectx.Client) error {
	if strings.TrimSpace(e.request.RewriteTargetRoundID) != "" && len(e.request.RewriteRemoveMessageUUIDs) == 0 {
		return errors.New("rewrite remove message uuids are required")
	}
	if len(e.request.RewriteRemoveMessageUUIDs) > 0 {
		if err := client.RemoveMessages(e.ctx, e.request.RewriteRemoveMessageUUIDs); err != nil {
			e.service.loggerFor(e.ctx).Error("DM rewrite 删除 runtime 历史失败",
				"session_key", e.sessionKey,
				"agent_id", e.agent.AgentID,
				"round_id", e.request.RoundID,
				"target_round_id", e.request.RewriteTargetRoundID,
				"message_uuid_count", len(e.request.RewriteRemoveMessageUUIDs),
				"err", err,
			)
			return err
		}
	}
	if err := e.service.pruneHistoryRewriteTail(e.ctx, rewritePruneInput{
		WorkspacePath:      e.agent.WorkspacePath,
		SessionKey:         e.sessionKey,
		TargetRoundID:      e.request.RewriteTargetRoundID,
		ReplacementRoundID: e.request.RoundID,
		RoundIDs:           e.request.RewriteRemoveRoundIDs,
		RemoveMessageCount: e.request.RewriteRemoveMessageCount,
	}); err != nil {
		if closeErr := e.service.runtime.CloseSession(e.ctx, e.sessionKey); closeErr != nil && !runtimectx.IsRuntimeTransportClosedError(closeErr) {
			e.service.loggerFor(e.ctx).Warn("DM rewrite overlay 裁剪失败后关闭 runtime 失败",
				"session_key", e.sessionKey,
				"agent_id", e.agent.AgentID,
				"round_id", e.request.RoundID,
				"err", closeErr,
			)
		}
		return err
	}
	if e.request.RewriteRemoveMessageCount > 0 {
		e.session.MessageCount = max(e.session.MessageCount-e.request.RewriteRemoveMessageCount, 0)
		e.initialMessageCount = e.session.MessageCount
	}
	return nil
}

func (e *dmChatExecution) startRound() {
	roundCtx, cancel := context.WithCancel(context.Background())
	e.roundCtx = roundCtx
	e.service.runtime.StartRound(e.sessionKey, e.request.RoundID, cancel)
	e.service.permission.BindSessionRoute(e.sessionKey, permissionctx.RouteContext{
		DispatchSessionKey: e.sessionKey,
		AgentID:            e.agent.AgentID,
		RoundID:            e.request.RoundID,
		AgentRoundID:       e.request.AgentRoundID,
	})
}

func (e *dmChatExecution) registerRunner() {
	e.service.runtime.RegisterGoalAccountingFlush(e.sessionKey, e.request.RoundID, e.runner.flushGoalUsage)
	e.service.runtime.RegisterGoalAccountingClear(e.sessionKey, e.request.RoundID, e.runner.clearGoalUsage)
	e.service.runtime.RegisterGoalAccountingActivate(e.sessionKey, e.request.RoundID, e.runner.activateGoalUsage)
	e.service.runtime.RegisterGoalObjectiveRevision(e.sessionKey, e.request.RoundID, e.runner.goalObjectiveRevision)
	e.service.loggerFor(e.ctx).Info("受理 DM 会话消息",
		"session_key", e.sessionKey,
		"agent_id", e.agent.AgentID,
		"round_id", e.request.RoundID,
		"client_request_id", e.runner.clientRequestID,
		"content_chars", utf8.RuneCountInString(e.runner.content),
		"content_preview", logx.PreviewText(e.runner.content, 240),
		"attachment_count", len(e.request.Attachments),
	)
}

func (e *dmChatExecution) persistRound() error {
	markerOptions := workspacestore.RoundMarkerOptions{
		UserMessageID:  e.request.UserMessageID,
		AgentRoundID:   e.request.AgentRoundID,
		DeliveryPolicy: string(e.deliveryPolicy),
		Attachments:    e.request.Attachments,
		HiddenFromUser: e.request.Internal || e.request.InputOptions.HiddenFromUser,
		Synthetic:      e.request.InputOptions.Synthetic || e.request.Internal,
		Purpose:        e.request.InputOptions.Purpose,
		Metadata:       e.request.InputOptions.Metadata,
	}
	if err := e.service.recordRoundMarkerWithOptions(e.runner.workspacePath, e.runner.session, e.runner.roundID, e.runner.content, markerOptions); err != nil {
		return e.failPersistence(
			err,
			"轮次标记持久化失败",
			"DM 轮次标记失败后刷新 session meta 失败",
			"DM 轮次标记持久化失败",
		)
	}
	if e.request.Internal {
		return nil
	}
	updatedSession, err := e.service.refreshSessionMetaAfterRoundMarker(e.runner.workspacePath, e.runner.session)
	if err != nil {
		return e.failPersistence(
			err,
			"会话元数据持久化失败",
			"DM 轮次元数据失败后刷新 session meta 失败",
			"DM 轮次元数据持久化失败",
		)
	}
	if updatedSession != nil {
		e.runner.session = *updatedSession
	}
	return nil
}

func (e *dmChatExecution) failPersistence(err error, cancelReason, refreshWarning, errorMessage string) error {
	e.service.runtime.MarkRoundFinished(e.sessionKey, e.request.RoundID)
	if closeErr := e.service.refreshSessionMetaRuntimeStateByKey(e.ctx, e.sessionKey); closeErr != nil {
		e.service.loggerFor(e.ctx).Warn(refreshWarning,
			"session_key", e.sessionKey,
			"agent_id", e.agent.AgentID,
			"round_id", e.request.RoundID,
			"err", closeErr,
		)
	}
	e.service.permission.CancelRequestsForSession(e.sessionKey, cancelReason)
	e.service.loggerFor(e.ctx).Error(errorMessage,
		"session_key", e.sessionKey,
		"agent_id", e.agent.AgentID,
		"round_id", e.request.RoundID,
		"err", err,
	)
	return err
}

func (e *dmChatExecution) launch() {
	if !e.request.Internal {
		e.service.scheduleTitleGeneration(
			e.ctx,
			e.parsed,
			e.runner.session,
			e.runner.content,
			e.initialMessageCount,
			e.runner.runtimeProvider,
			e.runner.runtimeModel,
		)
		e.broadcastAck()
	}
	if e.request.BroadcastUserMessage {
		e.service.broadcastUserRoundMarker(
			e.ctx,
			e.runner.session,
			e.runner.roundID,
			"",
			e.request.UserMessageID,
			e.runner.content,
			e.deliveryPolicy,
			e.request.Attachments,
		)
	}
	if strings.TrimSpace(e.request.RewriteTargetRoundID) != "" {
		e.service.broadcastHistoryRewriteResync(e.ctx, e.sessionKey, e.request.RewriteTargetRoundID, e.request.RoundID)
	}
	e.service.broadcastEventWithTimeout(e.ctx, e.sessionKey, protocol.NewRoundStatusEvent(e.sessionKey, e.request.RoundID, "running", ""))
	e.service.broadcastSessionStatus(e.ctx, e.sessionKey)
	go e.runner.run(e.roundCtx)
}

func (e *dmChatExecution) broadcastAck() {
	e.service.broadcastEventWithTimeout(e.ctx, e.sessionKey, protocol.NewChatAckEvent(
		e.sessionKey,
		e.request.ClientRequestID,
		e.request.ClientMessageID,
		e.request.RoundID,
		e.request.UserMessageID,
		true,
		dmChatAckPendingSlots(e.agent.AgentID, e.request.AgentRoundID),
	))
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
