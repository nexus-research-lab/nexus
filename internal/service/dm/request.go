// INPUT: DM 用户请求、内部 Goal 续跑与当前会话运行态。
// OUTPUT: 运行中投递、持久队列登记或新 round 启动。
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
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// HandleChat 处理一条 DM 写请求。显式输入与队列交接、Goal 续跑共享同一个启动边界。
func (s *Service) HandleChat(ctx context.Context, request Request) error {
	return s.handleChatLocked(ctx, request, chatExecutionInline)
}

// HandleRealtimeChat 持久化并 ACK 后，把 round 生命周期与实时连接分离。
func (s *Service) HandleRealtimeChat(ctx context.Context, request Request) error {
	return s.handleChatLocked(ctx, request, chatExecutionDetached)
}

type chatExecutionMode uint8

const (
	chatExecutionInline chatExecutionMode = iota
	chatExecutionDetached
)

func (s *Service) handleChatLocked(
	ctx context.Context,
	request Request,
	mode chatExecutionMode,
) error {
	if err := s.inputQueueDispatchMu.LockContext(ctx); err != nil {
		return err
	}
	defer s.inputQueueDispatchMu.Unlock()
	return s.handleChat(ctx, request, mode)
}

// handleChat 要求调用方已为显式输入持有 inputQueueDispatchMu；内部输入由各调度器自行保证互斥。
func (s *Service) handleChat(
	ctx context.Context,
	request Request,
	mode chatExecutionMode,
) error {
	execution, err := s.prepareChatExecution(ctx, request)
	if err != nil {
		return err
	}
	if handled, routeErr := execution.routeRunningInput(); handled || routeErr != nil {
		return routeErr
	}
	if mode == chatExecutionDetached && !execution.request.Internal {
		return execution.acceptAndLaunch()
	}
	if err = execution.prepareRunner(); err != nil {
		return err
	}
	if err = execution.persistRound(); err != nil {
		return err
	}
	execution.logAcceptance()
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
	dmClientPreparation
	content         conversationsvc.RuntimeContent
	atomicInput     bool
	recoveryContext []runtimectx.ContextualInputBlock
}

func (s *Service) prepareChatExecution(
	ctx context.Context,
	request Request,
) (*dmChatExecution, error) {
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
	agentValue, sessionItem, err := s.resolveDMSession(
		ctx,
		parsed,
		sessionKey,
		request.AgentID,
	)
	if err != nil {
		return nil, err
	}
	request.Attachments = s.normalizeChatAttachments(request.Attachments, agentValue.AgentID)
	deliveryPolicy := safeDMDeliveryPolicy(request)
	if !request.TrustedConfigurationContext && deliveryPolicy == protocol.ChatDeliveryPolicyGuide {
		deliveryPolicy = protocol.ChatDeliveryPolicyQueue
	}
	if conversationsvc.IsSlashCommandInput(request.Content) &&
		protocol.ShouldGuideRunningRound(deliveryPolicy) {
		deliveryPolicy = protocol.ChatDeliveryPolicyQueue
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
		deliveryPolicy:      deliveryPolicy,
	}, nil
}

func safeDMDeliveryPolicy(request Request) protocol.ChatDeliveryPolicy {
	policy := protocol.NormalizeChatDeliveryPolicy(string(request.DeliveryPolicy))
	if !request.TrustedConfigurationContext && policy == protocol.ChatDeliveryPolicyGuide {
		return protocol.ChatDeliveryPolicyQueue
	}
	return policy
}

func (s *Service) resolveDMSession(
	ctx context.Context,
	parsed protocol.SessionKey,
	sessionKey string,
	requestedAgentID string,
) (*protocol.Agent, protocol.Session, error) {
	agentID, err := s.resolveChatAgentID(ctx, parsed, requestedAgentID)
	if err != nil {
		return nil, protocol.Session{}, err
	}
	agentValue, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		return nil, protocol.Session{}, err
	}
	if strings.TrimSpace(agentValue.OwnerUserID) != authctx.OwnerUserID(ctx) {
		return nil, protocol.Session{}, errors.New("agent owner does not match request owner")
	}
	sessionItem, err := s.ensureSession(ctx, agentValue, parsed, sessionKey)
	if err != nil {
		return nil, protocol.Session{}, err
	}
	return agentValue, sessionItem, nil
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
	if err := e.prepareRoundStart(); err != nil {
		return err
	}
	admissionBaseContext := e.ctx
	admission, err := clientopts.BeginAgentRuntimeAdmission(
		admissionBaseContext,
		e.service.admission,
	)
	if err != nil {
		return err
	}
	defer admission.Release()
	e.ctx = admission.Context()

	preparation, err := e.prepareRuntime()
	if err != nil {
		return err
	}
	if !e.startRound() {
		return runtimectx.ErrRuntimeSessionClosing
	}
	// session 与 round 已同时进入 Manager；后续认证转场可由 owner 级关闭完整撤销。
	e.ctx = admissionBaseContext
	admission.Release()
	e.runner = e.newRoundRunner()
	e.runner.bindRuntime(preparation)
	e.registerRunner()
	return nil
}

func (e *dmChatExecution) prepareRoundStart() error {
	if err := e.service.ensureQuotaAvailable(e.ctx); err != nil {
		if e.request.Internal && strings.TrimSpace(e.request.GoalID) != "" {
			e.service.recordGoalQuotaLimit(e.ctx, e.sessionKey, e.request.RoundID, err)
		}
		return err
	}
	if err := e.interruptRunningRound(); err != nil {
		return err
	}
	return nil
}

func (e *dmChatExecution) acceptAndLaunch() error {
	if err := e.prepareRoundStart(); err != nil {
		return err
	}
	if !e.startRound() {
		return runtimectx.ErrRuntimeSessionClosing
	}
	e.runner = e.newRoundRunner()
	if err := e.persistRound(); err != nil {
		return err
	}
	e.logAcceptance()
	e.broadcastAcceptance()
	go e.runAcceptedRound()
	return nil
}

func (e *dmChatExecution) runAcceptedRound() {
	admissionBaseContext := e.ctx
	admission, err := clientopts.BeginAgentRuntimeAdmission(
		admissionBaseContext,
		e.service.admission,
	)
	if err != nil {
		defer e.service.runtime.MarkRoundFinished(e.sessionKey, e.request.RoundID)
		e.runner.failRuntimeStartup(err)
		return
	}
	e.ctx = admission.Context()
	preparation, err := e.prepareRuntime()
	e.ctx = admissionBaseContext
	admission.Release()
	if err != nil {
		defer e.service.runtime.MarkRoundFinished(e.sessionKey, e.request.RoundID)
		e.runner.failRuntimeStartup(err)
		return
	}
	e.runner.bindRuntime(preparation)
	e.registerRunner()
	e.service.scheduleTitleGeneration(
		e.roundCtx,
		e.parsed,
		e.runner.session,
		e.runner.content,
		e.initialMessageCount,
		e.runner.runtimeProvider,
		e.runner.runtimeModel,
	)
	e.runner.run(e.roundCtx)
}

func (e *dmChatExecution) interruptRunningRound() error {
	if e.request.Internal || e.deliveryPolicy != protocol.ChatDeliveryPolicyInterrupt {
		return nil
	}
	return e.service.interruptSession(e.ctx, e.sessionKey, "收到新的用户消息，上一轮已停止")
}

func (e *dmChatExecution) prepareRuntime() (dmRuntimePreparation, error) {
	runtimeCtx := e.runtimeContext()
	slashInput := conversationsvc.IsSlashCommandInput(e.request.Content)
	if slashInput && len(e.request.Attachments) > 0 {
		return dmRuntimePreparation{}, slashCommandAttachmentError{}
	}
	runtimeContent, err := e.service.renderRuntimeContentWithAttachments(
		runtimeCtx,
		e.request.Content,
		e.request.Attachments,
	)
	if err != nil {
		return dmRuntimePreparation{}, err
	}
	clientPreparation, err := e.service.ensureClient(
		runtimeCtx,
		e.sessionKey,
		e.agent,
		e.session,
		e.request,
	)
	if err != nil {
		e.service.loggerFor(runtimeCtx).Error("DM runtime client 初始化失败",
			"session_key", e.sessionKey,
			"agent_id", e.agent.AgentID,
			"round_id", e.request.RoundID,
			"err", err,
		)
		return dmRuntimePreparation{}, err
	}
	if !runtimeContent.IsEmpty() && !slashInput {
		runtimeContent = runtimeContent.AppendText(e.service.agents.BuildRuntimeUserMessageSuffixForContext(
			runtimeCtx,
			e.agent,
			"dm:"+strings.TrimSpace(e.sessionKey),
			clientPreparation.emotionEnabled,
		))
	}
	if override := strings.TrimSpace(e.request.GoalContext); e.request.Internal && override != "" {
		clientPreparation.goalContext = override
		if goalID := strings.TrimSpace(e.request.GoalID); goalID != "" {
			clientPreparation.goalIDForUsage = goalID
		}
	}
	if err = e.applyHistoryRewrite(clientPreparation.client); err != nil {
		return dmRuntimePreparation{}, err
	}
	recoveryContext := e.recoveryContextualInputs()
	atomicInput := slashInput
	if slashInput {
		clientPreparation.goalContext = ""
		recoveryContext = nil
	}
	return dmRuntimePreparation{
		dmClientPreparation: clientPreparation,
		content:             runtimeContent,
		atomicInput:         atomicInput,
		recoveryContext:     recoveryContext,
	}, nil
}

func (e *dmChatExecution) runtimeContext() context.Context {
	if e.roundCtx != nil {
		return e.roundCtx
	}
	return e.ctx
}

func (e *dmChatExecution) newRoundRunner() *roundRunner {
	return &roundRunner{
		service:                    e.service,
		workspacePath:              e.agent.WorkspacePath,
		session:                    e.session,
		agent:                      e.agent,
		sessionKey:                 e.sessionKey,
		roundID:                    e.request.RoundID,
		agentRoundID:               e.request.AgentRoundID,
		userMessageID:              e.request.UserMessageID,
		clientRequestID:            e.request.ClientRequestID,
		content:                    strings.TrimSpace(e.request.Content),
		ownerUserID:                strings.TrimSpace(e.agent.OwnerUserID),
		mapper:                     dmdomain.NewMessageMapper(e.sessionKey, e.agent.AgentID, e.request.RoundID, e.request.AgentRoundID, e.request.UserMessageID, e.agent.WorkspacePath),
		inputOptions:               e.request.InputOptions,
		internal:                   e.request.Internal,
		trustedExternalInteractive: e.request.TrustedExternalInteractiveContext,
		externalReplyTarget:        e.request.ExternalReplyTarget,
		executionID:                strings.TrimSpace(e.request.ExecutionID),
		goalObjectiveRevision:      &atomic.Int64{},
		goalUsage:                  goalsvc.NewRuntimeUsageAccumulator(false),
		goalUsageStarted:           time.Now(),
		permissionHandler:          e.request.PermissionHandler,
		automationRun:              cloneAutomationRunContext(e.request.AutomationRun),
	}
}

func cloneAutomationRunContext(value *protocol.AutomationRunContext) *protocol.AutomationRunContext {
	if value == nil {
		return nil
	}
	result := value.Normalized()
	return &result
}

func (r *roundRunner) bindRuntime(preparation dmRuntimePreparation) {
	r.runtimeContent = preparation.content
	r.atomicInput = preparation.atomicInput
	r.recoveryContext = preparation.recoveryContext
	r.client = preparation.client
	r.runtimeKind = preparation.runtimeKind
	r.runtimeProvider = preparation.runtimeProvider
	r.runtimeModel = preparation.runtimeModel
	r.goalContext = preparation.goalContext
	r.goalIDForUsage = preparation.goalIDForUsage
	r.childGoalIDForUsage = preparation.goalIDForUsage
	if preparation.goalObjectiveRevision != nil {
		r.goalObjectiveRevision = preparation.goalObjectiveRevision
	}
	r.goalUsage = goalsvc.NewRuntimeUsageAccumulator(
		strings.TrimSpace(preparation.goalIDForUsage) != "",
	)
	r.goalUsageStarted = time.Now()
	r.goalUsageScopeConsumed = strings.TrimSpace(preparation.goalIDForUsage) != ""
	r.permissionMode = preparation.permissionMode
}

func (e *dmChatExecution) applyHistoryRewrite(client runtimectx.Client) error {
	runtimeCtx := e.runtimeContext()
	if strings.TrimSpace(e.request.RewriteTargetRoundID) != "" && len(e.request.RewriteRemoveMessageUUIDs) == 0 {
		return errors.New("rewrite remove message uuids are required")
	}
	lease, hasLease := e.service.runtime.CaptureClientLease(e.sessionKey, client)
	if len(e.request.RewriteRemoveMessageUUIDs) > 0 {
		if err := client.RemoveMessages(runtimeCtx, e.request.RewriteRemoveMessageUUIDs); err != nil {
			e.service.loggerFor(runtimeCtx).Error("DM rewrite 删除 runtime 历史失败",
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
	if err := e.service.pruneHistoryRewriteTail(runtimeCtx, rewritePruneInput{
		WorkspacePath:      e.agent.WorkspacePath,
		SessionKey:         e.sessionKey,
		TargetRoundID:      e.request.RewriteTargetRoundID,
		ReplacementRoundID: e.request.RoundID,
		RoundIDs:           e.request.RewriteRemoveRoundIDs,
		RemoveMessageCount: e.request.RewriteRemoveMessageCount,
	}); err != nil {
		if hasLease {
			closeCtx, cancelClose := context.WithTimeout(context.Background(), runtimectx.RoundIdleAbortTimeout)
			_, closeErr := e.service.runtime.CloseSessionIfLease(closeCtx, lease)
			cancelClose()
			if closeErr != nil && !runtimectx.IsRuntimeTransportClosedError(closeErr) {
				e.service.loggerFor(runtimeCtx).Warn("DM rewrite overlay 裁剪失败后关闭 runtime 失败",
					"session_key", e.sessionKey,
					"agent_id", e.agent.AgentID,
					"round_id", e.request.RoundID,
					"err", closeErr,
				)
			}
		}
		return err
	}
	if e.request.RewriteRemoveMessageCount > 0 {
		e.session.MessageCount = max(e.session.MessageCount-e.request.RewriteRemoveMessageCount, 0)
		e.initialMessageCount = e.session.MessageCount
	}
	return nil
}

func (e *dmChatExecution) startRound() bool {
	roundBase := contextWithExactOwner(context.WithoutCancel(e.ctx), e.agent.OwnerUserID)
	roundCtx, cancel := context.WithCancel(roundBase)
	if err := e.service.runtime.StartRound(roundCtx, e.sessionKey, e.request.RoundID, cancel); err != nil {
		return false
	}
	e.roundCtx = roundCtx
	roomID, conversationID := dmRoomPermissionRoute(e.sessionKey, e.session)
	e.service.permission.BindSessionRoute(e.sessionKey, permissionctx.RouteContext{
		DispatchSessionKey: e.sessionKey,
		RoomID:             roomID,
		ConversationID:     conversationID,
		AgentID:            e.agent.AgentID,
		RoundID:            e.request.RoundID,
		AgentRoundID:       e.request.AgentRoundID,
	})
	return true
}

func dmRoomPermissionRoute(sessionKey string, session protocol.Session) (string, string) {
	if dmRoomConversationID(protocol.ParseSessionKey(sessionKey)) == "" {
		return "", ""
	}
	return strings.TrimSpace(dmdomain.StringPointerValue(session.RoomID)),
		strings.TrimSpace(dmdomain.StringPointerValue(session.ConversationID))
}

func (e *dmChatExecution) registerRunner() {
	e.service.runtime.RegisterGoalAccountingFlush(e.sessionKey, e.request.RoundID, e.runner.flushGoalUsage)
	e.service.runtime.RegisterGoalAccountingClear(e.sessionKey, e.request.RoundID, e.runner.clearGoalUsage)
	e.service.runtime.RegisterGoalAccountingFinalize(e.sessionKey, e.request.RoundID, e.runner.beginGoalUsageFinalizing)
	e.service.runtime.RegisterGoalAccountingActivate(e.sessionKey, e.request.RoundID, e.runner.activateGoalUsage)
	e.runner.initializeGoalUsageCreateGuard()
	e.service.runtime.RegisterGoalAccountingCreateGuard(
		e.sessionKey,
		e.request.RoundID,
		e.request.RoundID,
		e.runner.goalUsageScopeWasConsumed,
	)
	e.service.runtime.RegisterGoalObjectiveRevision(e.sessionKey, e.request.RoundID, e.runner.goalObjectiveRevision)
}

func (e *dmChatExecution) logAcceptance() {
	e.service.loggerFor(e.ctx).Info("受理 DM 会话消息",
		"session_key", e.sessionKey,
		"agent_id", e.agent.AgentID,
		"round_id", e.request.RoundID,
		"client_request_id", e.request.ClientRequestID,
		"content_chars", utf8.RuneCountInString(strings.TrimSpace(e.request.Content)),
		"content_preview", logx.PreviewText(strings.TrimSpace(e.request.Content), 240),
		"attachment_count", len(e.request.Attachments),
	)
}

func (e *dmChatExecution) persistRound() error {
	markerOptions := workspacestore.RoundMarkerOptions{
		UserMessageID:   e.request.UserMessageID,
		AgentRoundID:    e.request.AgentRoundID,
		ClientMessageID: e.request.ClientMessageID,
		DeliveryPolicy:  string(e.deliveryPolicy),
		Attachments:     e.request.Attachments,
		HiddenFromUser:  e.request.Internal || e.request.InputOptions.HiddenFromUser,
		Synthetic:       e.request.InputOptions.Synthetic || e.request.Internal,
		Purpose:         e.request.InputOptions.Purpose,
		Metadata:        e.request.InputOptions.Metadata,
	}
	if err := e.service.recordRoundMarkerWithOptionsForOwner(
		e.agent.OwnerUserID,
		e.agent.WorkspacePath,
		e.session,
		e.request.RoundID,
		strings.TrimSpace(e.request.Content),
		markerOptions,
	); err != nil {
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
	if dmRequestHasCanonicalUserInput(e.request) && dmRoomConversationID(e.parsed) != "" {
		if err := e.service.markRoomConversationStarted(
			e.runtimeContext(),
			e.sessionKey,
			time.Now().UTC(),
		); err != nil {
			// round marker 已是 durable acceptance point，后续派生状态失败
			// 不能把已受理输入变成永远不会执行的幽灵消息。
			e.service.loggerFor(e.runtimeContext()).Warn(
				"DM 已受理后写入 Room conversation 活动状态失败",
				"session_key", e.sessionKey,
				"agent_id", e.agent.AgentID,
				"round_id", e.request.RoundID,
				"err", err,
			)
		}
	}
	updatedSession, err := e.service.refreshSessionMetaAfterRoundMarkerForOwner(
		e.agent.OwnerUserID,
		e.agent.WorkspacePath,
		e.session,
	)
	if err != nil {
		e.service.loggerFor(e.runtimeContext()).Warn(
			"DM 已受理后刷新会话元数据失败",
			"session_key", e.sessionKey,
			"agent_id", e.agent.AgentID,
			"round_id", e.request.RoundID,
			"err", err,
		)
		// 保持 runner 的派生计数单调；后续任一成功的消息 meta 写入都能
		// 把这次 durable user marker 一并收敛回 session 快照。
		e.session = closePersistedSessionMeta(e.session)
		e.session.LastActivity = time.Now().UTC()
		e.session.MessageCount++
		if e.runner != nil {
			e.runner.session = e.session
		}
		return nil
	}
	if updatedSession != nil {
		e.session = *updatedSession
		if e.runner != nil {
			e.runner.session = *updatedSession
		}
	}
	return nil
}

func dmRequestHasCanonicalUserInput(request Request) bool {
	return !request.Internal &&
		!request.InputOptions.HiddenFromUser &&
		!request.InputOptions.Synthetic
}

func (s *Service) markRoomConversationStarted(
	ctx context.Context,
	sessionKey string,
	activityAt time.Time,
) error {
	if s == nil || s.roomActivity == nil {
		return nil
	}
	conversationID := dmRoomConversationID(protocol.ParseSessionKey(sessionKey))
	if conversationID == "" {
		return nil
	}
	if activityAt.IsZero() {
		activityAt = time.Now().UTC()
	}
	return s.roomActivity.MarkConversationStarted(ctx, conversationID, activityAt.UTC())
}

func dmRoomConversationID(parsed protocol.SessionKey) string {
	if parsed.Kind != protocol.SessionKeyKindAgent ||
		protocol.NormalizeSessionKeyChannelSegment(parsed.Channel) != protocol.SessionChannelWebSocketSegment ||
		strings.TrimSpace(parsed.ChatType) != "dm" {
		return ""
	}
	return strings.TrimSpace(parsed.Ref)
}

func (e *dmChatExecution) failPersistence(err error, cancelReason, refreshWarning, errorMessage string) error {
	e.service.runtime.MarkRoundTerminal(e.sessionKey, e.request.RoundID)
	defer e.service.runtime.MarkRoundFinished(e.sessionKey, e.request.RoundID)
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
	e.broadcastRoundStarted(e.ctx)
	go e.runner.run(e.roundCtx)
}

func (e *dmChatExecution) broadcastAcceptance() {
	e.broadcastAck()
	e.broadcastRoundStarted(e.runtimeContext())
}

func (e *dmChatExecution) broadcastRoundStarted(ctx context.Context) {
	if e.request.BroadcastUserMessage {
		e.service.broadcastUserRoundMarker(
			ctx,
			e.session,
			e.request.RoundID,
			"",
			e.request.UserMessageID,
			strings.TrimSpace(e.request.Content),
			e.deliveryPolicy,
			e.request.Attachments,
		)
	}
	if strings.TrimSpace(e.request.RewriteTargetRoundID) != "" {
		e.service.broadcastHistoryRewriteResync(
			ctx,
			e.sessionKey,
			e.request.RewriteTargetRoundID,
			e.request.RoundID,
		)
	}
	e.service.broadcastEventWithTimeout(
		ctx,
		e.sessionKey,
		protocol.NewRoundStatusEvent(e.sessionKey, e.request.RoundID, protocol.RoundStatusRunning, ""),
	)
	e.service.broadcastSessionStatus(ctx, e.sessionKey)
}

func (e *dmChatExecution) broadcastAck() {
	e.service.broadcastEventWithTimeout(e.runtimeContext(), e.sessionKey, protocol.NewChatAckEvent(
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
	// durable user queue 只承载真实用户输入；隐藏或 synthetic 消息必须走
	// internal 路径，避免排队后丢失来源语义并误消费 conversation draft。
	if !request.Internal && !dmRequestHasCanonicalUserInput(request) {
		return "", protocol.SessionKey{}, errors.New("hidden or synthetic input must be internal")
	}
	sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
	if err != nil {
		return "", protocol.SessionKey{}, err
	}
	if !protocol.HasChatInput(request.Content, request.Attachments) &&
		!(request.Internal && strings.TrimSpace(request.GoalContext) != "") {
		return "", protocol.SessionKey{}, errors.New("content is required")
	}
	if len(strings.TrimSpace(request.ClientMessageID)) > protocol.MaxClientMessageIDBytes {
		return "", protocol.SessionKey{}, errors.New("client_message_id 过长")
	}
	if request.Internal &&
		strings.TrimSpace(request.InputOptions.Purpose) == "goal_continuation" &&
		(strings.TrimSpace(request.GoalID) == "" ||
			request.GoalObjectiveRevision <= 0 ||
			strings.TrimSpace(request.ExecutionID) == "") {
		return "", protocol.SessionKey{}, errors.New(
			"goal continuation requires exact goal, objective revision, and execution binding",
		)
	}

	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return "", protocol.SessionKey{}, ErrRoomSessionNotImplemented
	}
	return sessionKey, parsed, nil
}
