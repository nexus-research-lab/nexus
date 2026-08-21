// INPUT: 已准备的 DM round、runtime 消息、工具面指纹与终态结果。
// OUTPUT: durable 历史、SDK session 工具面基线、ACK 门控的引导确认、Goal 结算及用户队列优先的后续派发。
// POS: DM 单轮执行生命周期的主状态机。
package dm

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/runtime/trace"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	orchestration "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	orchestrationruntimehook "github.com/nexus-research-lab/nexus/internal/service/orchestration/runtimehook"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type dmRoundMapperAdapter struct {
	mapper *dmdomain.MessageMapper
}

func (a dmRoundMapperAdapter) Map(
	incoming sdkprotocol.ReceivedMessage,
	interruptReason ...string,
) (exec.RoundMapResult, error) {
	events, durableMessages, terminalStatus, resultSubtype, err := a.mapper.Map(incoming, interruptReason...)
	if err != nil {
		return exec.RoundMapResult{}, err
	}
	return exec.RoundMapResult{
		Events:          events,
		DurableMessages: durableMessages,
		TerminalStatus:  terminalStatus,
		ResultSubtype:   resultSubtype,
	}, nil
}

func (a dmRoundMapperAdapter) SessionID() string {
	return a.mapper.SessionID()
}

type roundRunner struct {
	service                     *Service
	workspacePath               string
	session                     protocol.Session
	agent                       *protocol.Agent
	sessionKey                  string
	roundID                     string
	agentRoundID                string
	userMessageID               string
	clientRequestID             string
	content                     string
	runtimeContent              conversationsvc.RuntimeContent
	atomicInput                 bool
	recoveryContext             []runtimectx.ContextualInputBlock
	client                      runtimectx.Client
	runtimeKind                 string
	runtimeProvider             string
	runtimeModel                string
	toolSurfaceFingerprint      string
	forkSourceSessionID         string
	ownerUserID                 string
	mapper                      *dmdomain.MessageMapper
	inputOptions                sdkprotocol.OutboundMessageOptions
	internal                    bool
	executionOrigin             string
	deferredAssistant           *DeferredAssistantHooks
	trustedExternalInteractive  bool
	externalReplyTarget         *ExternalReplyTarget
	goalContext                 string
	executionID                 string
	goalIDForUsage              string
	childGoalIDForUsage         string
	goalObjectiveRevision       *atomic.Int64
	responsibilityState         *runtimectx.ResponsibilityAuthorityState
	sdkSessionIdentity          *runtimectx.SDKSessionIdentityState
	commandReceipts             *runtimecommand.ReceiptState
	commandResources            *runtimecommand.RoundResources
	commandReceiptSequence      uint64
	goalUsage                   *goalsvc.RuntimeUsageAccumulator
	goalUsageStarted            time.Time
	goalUsageBindingMu          sync.Mutex
	goalUsageMu                 sync.Mutex
	goalLastAssistant           protocol.Message
	goalCompletionCandidateID   string
	goalCompletionAssistant     protocol.Message
	goalCompletionReceipt       protocol.GoalCompletionReceipt
	goalCompletionReceiptStored bool
	goalToolProgress            bool
	automationRun               *protocol.AutomationRunContext
	goalTerminalUsageSnapshot   goalsvc.RuntimeUsageSnapshot
	goalTerminalUsageVersion    uint64
	goalTerminalUsagePending    bool
	goalTokenUsageObserved      bool
	goalUsageScopeConsumed      bool
	subagentTasks               map[string]struct{}
	subagentUsagePending        map[string]dmSubagentUsageObservation
	subagentUsageClaimPending   bool
	goalUsageRetryRunning       bool
	subagentParentTerminal      string
	subagentPostRoundDispatched bool
	postRoundDispatchHook       func()
	permissionMode              sdkpermission.Mode
	permissionHandler           sdkpermission.Handler
	resultUsageWritten          bool
	deferredRuntimeMessageUUIDs []string

	// goalUsageRetryBaseDelay 为零时使用生产退避；测试只调整时钟尺度。
	goalUsageRetryBaseDelay time.Duration
	// externalTypingDelay 为零时使用外部通道的产品延迟。
	externalTypingDelay time.Duration
}

func (r *roundRunner) run(ctx context.Context) {
	defer r.commandResources.Close()
	defer r.service.runtime.MarkRoundFinished(r.sessionKey, r.roundID)
	defer r.service.clearPendingInputQueueGuidance(r.sessionKey, r.roundID)
	logger := r.service.loggerFor(ctx).With(
		"session_key", r.sessionKey,
		"agent_id", r.agent.AgentID,
		"round_id", r.roundID,
	)
	logger.Info("开始执行 DM round")
	ownerCtx := contextWithExactOwner(context.Background(), r.ownerUserID)
	stopTyping := r.startExternalReplyTyping(ownerCtx)
	defer stopTyping()
	result, err := r.executeRound(ctx, logger)
	if err != nil {
		if errors.Is(err, exec.ErrRoundInterrupted) {
			r.finishInterrupted(result, r.service.runtime.GetInterruptReason(r.sessionKey, r.roundID))
			return
		}
		r.failRound(result, err)
		return
	}
	if r.deferredAssistant != nil {
		r.finishDeferredAssistant(result)
		return
	}
	if result.TerminalStatus == "finished" && (result.ResultSubtype == "" || result.ResultSubtype == "success") {
		if err := r.confirmInputQueueGuidanceFallback(context.Background()); err != nil {
			r.failRound(result, err)
			return
		}
	}

	r.service.loggerFor(context.Background()).Info("DM round 结束",
		"session_key", r.sessionKey,
		"agent_id", r.agent.AgentID,
		"round_id", r.roundID,
		"status", result.TerminalStatus,
		"result_subtype", result.ResultSubtype,
		"error_message", strings.TrimSpace(result.ErrorMessage),
	)
	finalAssistant := r.mapper.LastAssistantMessage()
	if result.CompletedByAssistant {
		r.deliverExternalAssistantReply(ownerCtx, finalAssistant)
		r.rememberGoalCompletionAssistant(finalAssistant)
		r.persistGoalCompletionReceipt(context.Background(), false)
	}
	r.recordGoalUsageLimit(result)
	r.recordGoalContinuationProgress(result)
	r.finalizeGoalUsage(context.Background(), result, finalAssistant)
	if result.CompletedByAssistant {
		r.recordTerminalAssistantUsage(finalAssistant)
	}
	r.service.runtime.MarkRoundTerminal(r.sessionKey, r.roundID)
	r.scheduleEchoAfterTerminal(result, finalAssistant)
	r.broadcastContextUsage()
	r.refreshSessionMetaAfterRoundFinished()
	r.service.broadcastEventWithTimeout(
		context.Background(),
		r.sessionKey,
		terminalRoundStatusEvent(r, result),
	)
	r.service.broadcastSessionStatus(context.Background(), r.sessionKey)
	if r.service.runtime.HasSubagentHistory(r.sessionKey) {
		r.startIdleSubagentNotificationDrain()
	}
	r.markSubagentParentTerminal(subagentParentTerminalNormal)
	if r.hasRunningSubagentTask() {
		return
	}
	r.dispatchPostRoundWorkAfterSubagents()
}

// terminalRoundStatusEvent 把 SDK 已经返回的终态统一投影成 round_status。
// result 终态可能是正常的 result 消息，也可能是 runtime 自己生成的错误结果；
// 后者不能只依赖瞬时 error 事件，否则客户端错过事件后就只剩“已停止”状态。
func terminalRoundStatusEvent(r *roundRunner, result exec.RoundExecutionResult) protocol.EventMessage {
	if r == nil {
		return protocol.NewRoundStatusEvent("", "", result.TerminalStatus, result.ResultSubtype)
	}
	var event protocol.EventMessage
	if result.TerminalStatus == "error" || result.ResultSubtype == "error" {
		event = protocol.NewRoundStatusErrorEvent(r.sessionKey, r.roundID, result.ErrorMessage)
	} else {
		event = protocol.NewRoundStatusEvent(r.sessionKey, r.roundID, result.TerminalStatus, result.ResultSubtype)
	}
	if r.agent != nil {
		event.AgentID = r.agent.AgentID
	}
	event.RoundID = r.roundID
	event.AgentRoundID = r.agentRoundID
	return event
}

func (r *roundRunner) executeRound(
	ctx context.Context,
	logger *slog.Logger,
) (exec.RoundExecutionResult, error) {
	actor := r.orchestrationActor()
	executionInputs, err := r.service.executionContextualInputs(ctx, actor)
	if err != nil {
		return exec.RoundExecutionResult{}, err
	}
	if r.service.subagentAdmission != nil {
		r.service.runtime.SetSubagentHookCallbacks(
			r.sessionKey,
			r.roundID,
			orchestrationruntimehook.Callbacks(
				r.service.subagentAdmission,
				orchestrationruntimehook.Context{
					Actor:             actor,
					RuntimeSessionKey: r.sessionKey,
					Logger:            r.service.loggerFor(ctx),
				},
			),
		)
		defer r.service.runtime.ClearSubagentHookCallbacks(r.sessionKey, r.roundID)
	}
	r.service.beginExecutionRuntimeGraph(actor)
	result, executeErr := exec.ExecuteRound(ctx, exec.RoundExecutionRequest{
		Content:          r.runtimeContent.Payload(),
		AtomicInput:      r.atomicInput,
		ContextualInputs: append(executionInputs, r.contextualInputs()...),
		InputOptions:     r.runtimeInputOptions(),
		Client:           r.client,
		Mapper:           dmRoundMapperAdapter{mapper: r.mapper},
		IdleTimeout:      r.service.config.RuntimeRoundIdleTimeout(),
		IdlePauseState: func() (bool, <-chan struct{}) {
			return r.service.permission.PendingRequestState(r.sessionKey)
		},
		InterruptReason: func() string {
			return r.service.runtime.GetInterruptReason(r.sessionKey, r.roundID)
		},
		ObserveIncomingMessage: func(incoming sdkprotocol.ReceivedMessage) {
			r.observeDeferredRuntimeMessage(incoming)
			r.service.observeExecutionRuntimeGraph(actor, incoming)
			r.observeExecutionPersistenceEvidence(actor, incoming)
			if incoming.Type == sdkprotocol.MessageTypeStreamEvent && !r.service.config.MessageDebugStreamEvent {
				return
			}
			fields := trace.BuildSDKMessageLogFieldsWithOptions(
				incoming,
				trace.SDKMessageLogOptions{
					IncludeStreamEvent:  r.service.config.MessageDebugStreamEvent,
					IncludeSnapshotData: true,
				},
			)
			if len(fields) == 0 {
				return
			}
			logger.Debug("Agent ", fields...)
		},
		SyncSessionID: func(sessionID string) error {
			if sourceSessionID := strings.TrimSpace(r.forkSourceSessionID); sourceSessionID != "" &&
				strings.TrimSpace(sessionID) == sourceSessionID {
				return errors.New("runtime fork 仍返回 source SDK session")
			}
			updatedSession, syncErr := r.service.syncSDKSessionIDForOwner(
				ctx,
				r.ownerUserID,
				r.workspacePath,
				r.session,
				sessionID,
				r.runtimeKind,
				r.runtimeProvider,
				r.runtimeModel,
				r.toolSurfaceFingerprint,
			)
			if syncErr != nil {
				return syncErr
			}
			r.session = updatedSession
			if r.sdkSessionIdentity != nil {
				r.sdkSessionIdentity.Set(sessionID)
			}
			if forkSessionStateCommitted(r.session, sessionID, r.toolSurfaceFingerprint) {
				r.forkSourceSessionID = ""
			}
			return nil
		},
		HandleDurableMessage: func(message protocol.Message) error {
			return r.handleDurableMessage(message)
		},
		EmitEvent: func(event protocol.EventMessage) error {
			if r.deferredAssistant != nil {
				return nil
			}
			r.service.broadcastEventWithTimeout(context.Background(), r.sessionKey, event)
			return nil
		},
	})
	if executeErr == nil && strings.TrimSpace(r.forkSourceSessionID) != "" {
		executeErr = errors.New("runtime fork 未提交可恢复的独立 SDK session")
	}
	if executeErr != nil && strings.TrimSpace(r.forkSourceSessionID) != "" {
		r.closeUncommittedForkRuntime(logger, executeErr)
	}
	failureReason := ""
	if executeErr != nil {
		failureReason = executeErr.Error()
	}
	r.service.finishExecutionRuntimeGraph(
		actor,
		result.TerminalStatus,
		failureReason,
	)
	return result, executeErr
}

func (r *roundRunner) closeUncommittedForkRuntime(logger *slog.Logger, forkErr error) {
	lease, ok := r.service.runtime.CaptureClientLease(r.sessionKey, r.client)
	if !ok {
		return
	}
	closeCtx, cancel := context.WithTimeout(context.Background(), runtimectx.RoundIdleAbortTimeout)
	defer cancel()
	_, closeErr := r.service.runtime.CloseSessionIfLease(closeCtx, lease)
	if closeErr != nil && !runtimectx.IsRuntimeTransportClosedError(closeErr) {
		logger.Warn("关闭未提交的 fork runtime 失败", "fork_err", forkErr, "close_err", closeErr)
	}
}

func (r *roundRunner) orchestrationActor() orchestration.ActorContext {
	agentID := strings.TrimSpace(r.session.AgentID)
	if r.agent != nil {
		agentID = strings.TrimSpace(r.agent.AgentID)
	}
	actor := orchestration.ActorContext{
		OwnerUserID:           r.ownerUserID,
		SessionKey:            r.sessionKey,
		ExecutionID:           strings.TrimSpace(r.executionID),
		GoalID:                strings.TrimSpace(r.goalIDForUsage),
		GoalObjectiveRevision: r.currentGoalObjectiveRevision(),
		AgentID:               agentID,
		Role:                  orchestration.ExecutionActorCoordinator,
		ActorKind:             protocol.ExecutionActorAgent,
		ScopeKind:             protocol.ExecutionScopeDM,
		RootRoundID:           r.roundID,
		RuntimeRoundID:        r.roundID,
		AgentRoundID:          r.agentRoundID,
		PlanMode:              r.permissionMode == sdkpermission.ModePlan,
	}
	if r.responsibilityState != nil {
		if snapshot, ok := r.responsibilityState.Load(); ok {
			if executionID := strings.TrimSpace(snapshot.ExecutionID); executionID != "" {
				actor.ExecutionID = executionID
			}
			if goalID := strings.TrimSpace(snapshot.GoalID); goalID != "" {
				actor.GoalID = goalID
			}
			actor.GoalObjectiveRevision = snapshot.ObjectiveRevision
			actor.WorkBinding = snapshot.WorkBinding
			actor.ReviewBinding = snapshot.ReviewBinding
			if snapshot.WorkBinding != nil || snapshot.ReviewBinding != nil {
				actor.Role = orchestration.ExecutionActorMember
			}
		}
	}
	return actor
}

// runtimeInputOptions 把产品包装前的真实用户文本单独交给原生 Recall。
func (r *roundRunner) runtimeInputOptions() sdkprotocol.OutboundMessageOptions {
	if r == nil {
		return sdkprotocol.OutboundMessageOptions{}
	}
	options := runtimectx.RuntimeInputOptionsForPurpose(r.inputOptions, "goal_continuation")
	options.RecallQuery = ""
	if r.deferredAssistant != nil {
		options.MessageUUID = strings.TrimSpace(r.userMessageID)
	}
	if r.internal || r.atomicInput || options.Meta || options.Synthetic || options.HiddenFromUser {
		return options
	}
	options.RecallQuery = strings.TrimSpace(r.content)
	return options
}

func (r *roundRunner) observeDeferredRuntimeMessage(incoming sdkprotocol.ReceivedMessage) {
	if r == nil || r.deferredAssistant == nil {
		return
	}
	if uuid := strings.TrimSpace(incoming.UUID); uuid != "" {
		r.deferredRuntimeMessageUUIDs = append(r.deferredRuntimeMessageUUIDs, uuid)
	}
}

func (r *roundRunner) handleDurableMessage(message protocol.Message) error {
	role := protocol.MessageRole(message)
	if r.deferredAssistant != nil {
		if role == "result" {
			r.recordUsage(message)
		}
		return nil
	}
	if role == "assistant" || (role == "result" && message["is_error"] != true &&
		(dmdomain.NormalizeString(message["subtype"]) == "" || dmdomain.NormalizeString(message["subtype"]) == "success")) {
		if err := r.confirmInputQueueGuidanceFallback(context.Background()); err != nil {
			return err
		}
	}
	r.annotateSubagentTaskRuntimeKind(message)
	if err := r.persistMessage(message); err != nil {
		return err
	}
	r.service.observeExecutionRuntimeArtifacts(r.orchestrationActor(), message)
	settledSubagentUsage := r.recordSubagentGoalUsage(context.Background(), message)
	r.rememberSubagentTaskMessage(message)
	for _, settled := range settledSubagentUsage {
		r.clearSubagentUsageObservationPending(settled.taskID, settled.observation)
	}
	r.rememberGoalAssistantMessage(message)
	r.recordGoalUsageFromAssistantMessage(message)
	if message["role"] == "assistant" {
		roomID, conversationID := dmRoomPermissionRoute(r.sessionKey, r.session)
		r.service.permission.BindSessionRoute(r.sessionKey, permissionctx.RouteContext{
			DispatchSessionKey: r.sessionKey,
			RoomID:             roomID,
			ConversationID:     conversationID,
			AgentID:            r.agent.AgentID,
			MessageID:          dmdomain.NormalizeString(message["message_id"]),
			RoundID:            r.roundID,
			AgentRoundID:       r.agentRoundID,
		})
	}
	return nil
}

func (r *roundRunner) confirmInputQueueGuidance(ctx context.Context) error {
	return r.service.confirmPendingInputQueueGuidance(ctx, r.sessionKey, workspacestore.InputQueueLocation{
		OwnerUserID:   r.ownerUserID,
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: r.workspacePath,
		SessionKey:    r.sessionKey,
	}, r.roundID, nil)
}

func (r *roundRunner) confirmInputQueueGuidanceFallback(ctx context.Context) error {
	if r.service.runtime != nil && r.service.runtime.SupportsHookResponseAck(r.sessionKey) {
		return nil
	}
	return r.confirmInputQueueGuidance(ctx)
}

func (r *roundRunner) dispatchNextInputQueueItem() {
	location := workspacestore.InputQueueLocation{
		OwnerUserID:   r.ownerUserID,
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: r.workspacePath,
		SessionKey:    r.sessionKey,
	}
	r.service.startSessionBackgroundTask(r.sessionKey, r.ownerUserID, func(ctx context.Context) {
		r.service.releaseUndeliveredInputQueueGuidance(ctx, r.sessionKey, location, r.roundID)
		r.service.dispatchNextInputQueueItemAtLocation(ctx, r.sessionKey, r.agent.AgentID, location)
	})
}

func (r *roundRunner) dispatchPostRoundWork() {
	if r.postRoundDispatchHook != nil {
		r.postRoundDispatchHook()
		return
	}
	location := workspacestore.InputQueueLocation{
		OwnerUserID:   r.ownerUserID,
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: r.workspacePath,
		SessionKey:    r.sessionKey,
	}
	r.service.startSessionBackgroundTask(r.sessionKey, r.ownerUserID, func(ctx context.Context) {
		r.service.releaseUndeliveredInputQueueGuidance(ctx, r.sessionKey, location, r.roundID)
		if r.service.dispatchNextInputQueueItemAtLocation(ctx, r.sessionKey, r.agent.AgentID, location) {
			return
		}
		if r.hasGoalRoundBinding() {
			r.dispatchGoalContinuation(ctx)
		}
	})
}

func (r *roundRunner) persistMessage(message protocol.Message) error {
	if err := r.service.appendRuntimeHistoryMessageForOwner(
		r.ownerUserID,
		r.workspacePath,
		r.session,
		message,
	); err != nil {
		return err
	}
	r.recordUsage(message)
	updated, err := r.service.refreshSessionMetaAfterMessageForOwner(
		r.ownerUserID,
		r.workspacePath,
		r.session,
		message,
	)
	if err != nil {
		return err
	}
	if updated != nil {
		r.session = *updated
	}
	return nil
}

func (r *roundRunner) refreshSessionMetaAfterRoundFinished() {
	updated, err := r.service.refreshSessionMetaRuntimeStateForOwner(
		r.ownerUserID,
		r.workspacePath,
		r.session,
	)
	if err != nil {
		r.service.loggerFor(context.Background()).Error("DM round 结束后刷新 session meta 失败",
			"session_key", r.sessionKey,
			"agent_id", r.agent.AgentID,
			"round_id", r.roundID,
			"err", err,
		)
		return
	}
	if updated != nil {
		r.session = *updated
	}
}

func (r *roundRunner) recordUsage(message protocol.Message) {
	if r.service.usage == nil || protocol.MessageRole(message) != "result" {
		return
	}
	if !usagesvc.MessageHasUsage(message) {
		return
	}
	if r.writeUsage(message) {
		r.resultUsageWritten = true
	}
}

func (r *roundRunner) recordTerminalAssistantUsage(message protocol.Message) {
	if r.service.usage == nil || protocol.MessageRole(message) != "assistant" {
		return
	}
	if r.resultUsageWritten || !usagesvc.MessageHasUsage(message) {
		return
	}
	r.writeUsage(message)
}

func (r *roundRunner) writeUsage(message protocol.Message) bool {
	input := usagesvc.MessageRecordInput(r.ownerUserID, "dm_runtime", message)
	goalBound := strings.TrimSpace(r.goalIDForUsage) != ""
	executionBound := strings.TrimSpace(r.executionID) != ""
	lane := string(runtimectx.ResponsibilityLaneUnbound)
	if executionBound {
		lane = string(runtimectx.ResponsibilityLaneExecution)
	}
	if r.responsibilityState != nil {
		if authority, ok := r.responsibilityState.Load(); ok {
			goalBound = strings.TrimSpace(authority.GoalID) != ""
			executionBound = strings.TrimSpace(authority.ExecutionID) != ""
			lane = string(authority.Lane)
		}
	}
	surface, observed := r.service.runtime.CacheSurface(r.sessionKey)
	input.CacheAttribution = usagesvc.RuntimeCacheAttribution(
		surface.Input(),
		observed,
		goalBound,
		executionBound,
		lane,
	)
	if err := r.service.usage.RecordMessageUsage(context.Background(), input); err != nil {
		r.service.loggerFor(context.Background()).Error("DM token usage 写入失败",
			"session_key", r.sessionKey,
			"agent_id", r.agent.AgentID,
			"round_id", r.roundID,
			"err", err,
		)
		return false
	}
	return true
}
