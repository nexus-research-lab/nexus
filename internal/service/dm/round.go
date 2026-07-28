// INPUT: 已准备的 DM round、runtime 消息与终态结果。
// OUTPUT: durable 历史、ACK 门控的引导确认、Goal 结算及用户队列优先的后续派发。
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
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/runtime/trace"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
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
	client                      runtimectx.Client
	runtimeKind                 string
	runtimeProvider             string
	runtimeModel                string
	ownerUserID                 string
	mapper                      *dmdomain.MessageMapper
	inputOptions                sdkprotocol.OutboundMessageOptions
	internal                    bool
	externalReplyTarget         *ExternalReplyTarget
	goalContext                 string
	goalIDForUsage              string
	goalObjectiveRevision       *atomic.Int64
	goalUsage                   *goalsvc.RuntimeUsageAccumulator
	goalUsageStarted            time.Time
	goalUsageMu                 sync.Mutex
	goalLastAssistant           protocol.Message
	goalToolProgress            bool
	subagentTasks               map[string]struct{}
	subagentPostRoundDispatched bool
	permissionMode              sdkpermission.Mode
	permissionHandler           sdkpermission.Handler
	resultUsageWritten          bool
}

func (r *roundRunner) run(ctx context.Context) {
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
			r.finishInterrupted(r.service.runtime.GetInterruptReason(r.sessionKey, r.roundID))
			return
		}
		r.failRound(err)
		return
	}
	if result.TerminalStatus == "finished" && (result.ResultSubtype == "" || result.ResultSubtype == "success") {
		if err := r.confirmInputQueueGuidanceFallback(context.Background()); err != nil {
			r.failRound(err)
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
	}
	r.recordGoalUsage(context.Background(), result, finalAssistant)
	r.recordGoalUsageLimit(result)
	r.recordGoalContinuationProgress(result)
	if result.CompletedByAssistant {
		r.recordTerminalAssistantUsage(finalAssistant)
	}
	r.service.runtime.MarkRoundTerminal(r.sessionKey, r.roundID)
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
	return exec.ExecuteRound(ctx, exec.RoundExecutionRequest{
		Content:          r.runtimeContent.Payload(),
		ContextualInputs: goalContextualInputs(r.goalContext, r.goalIDForUsage, r.sessionKey),
		InputOptions:     runtimectx.RuntimeInputOptionsForPurpose(r.inputOptions, "goal_continuation"),
		Client:           r.client,
		Mapper:           dmRoundMapperAdapter{mapper: r.mapper},
		IdleTimeout:      r.service.config.RuntimeRoundIdleTimeout(),
		InterruptReason: func() string {
			return r.service.runtime.GetInterruptReason(r.sessionKey, r.roundID)
		},
		ObserveIncomingMessage: func(incoming sdkprotocol.ReceivedMessage) {
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
			updatedSession, syncErr := r.service.syncSDKSessionIDForOwner(
				ctx,
				r.ownerUserID,
				r.workspacePath,
				r.session,
				sessionID,
				r.runtimeKind,
				r.runtimeProvider,
				r.runtimeModel,
			)
			if syncErr != nil {
				return syncErr
			}
			r.session = updatedSession
			return nil
		},
		HandleDurableMessage: func(message protocol.Message) error {
			return r.handleDurableMessage(message)
		},
		EmitEvent: func(event protocol.EventMessage) error {
			r.service.broadcastEventWithTimeout(context.Background(), r.sessionKey, event)
			return nil
		},
	})
}

func (r *roundRunner) handleDurableMessage(message protocol.Message) error {
	role := protocol.MessageRole(message)
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
	r.rememberSubagentTaskMessage(message)
	r.rememberGoalAssistantMessage(message)
	r.recordGoalUsageFromAssistantMessage(message)
	if message["role"] == "assistant" {
		r.service.permission.BindSessionRoute(r.sessionKey, permissionctx.RouteContext{
			DispatchSessionKey: r.sessionKey,
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
		r.dispatchGoalContinuation(ctx)
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
