package dm

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
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
	logger := r.service.loggerFor(ctx).With(
		"session_key", r.sessionKey,
		"agent_id", r.agent.AgentID,
		"round_id", r.roundID,
	)
	logger.Info("开始执行 DM round")
	stopTyping := r.startExternalReplyTyping(context.Background())
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
		r.deliverExternalAssistantReply(context.Background(), finalAssistant)
	}
	r.recordGoalUsage(context.Background(), result, finalAssistant)
	r.recordGoalUsageLimit(result)
	r.recordGoalContinuationProgress(result)
	if result.CompletedByAssistant {
		r.recordTerminalAssistantUsage(finalAssistant)
	}
	r.service.runtime.MarkRoundFinished(r.sessionKey, r.roundID)
	r.refreshSessionMetaAfterRoundFinished()
	r.service.broadcastEventWithTimeout(
		context.Background(),
		r.sessionKey,
		protocol.NewRoundStatusEvent(r.sessionKey, r.roundID, result.TerminalStatus, result.ResultSubtype),
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
			updatedSession, syncErr := r.service.syncSDKSessionID(
				context.Background(),
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

func (r *roundRunner) dispatchNextInputQueueItem() {
	location := workspacestore.InputQueueLocation{
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: r.workspacePath,
		SessionKey:    r.sessionKey,
	}
	go r.service.dispatchNextInputQueueItemAtLocation(
		contextWithQueueOwner(context.Background(), r.ownerUserID),
		r.sessionKey,
		r.agent.AgentID,
		location,
	)
}

func (r *roundRunner) dispatchPostRoundWork() {
	location := workspacestore.InputQueueLocation{
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: r.workspacePath,
		SessionKey:    r.sessionKey,
	}
	go func() {
		ctx := contextWithQueueOwner(context.Background(), r.ownerUserID)
		if r.service.dispatchNextInputQueueItemAtLocation(ctx, r.sessionKey, r.agent.AgentID, location) {
			return
		}
		r.dispatchGoalContinuation(ctx)
	}()
}

func (r *roundRunner) persistMessage(message protocol.Message) error {
	if err := r.service.appendRuntimeHistoryMessage(r.workspacePath, r.session, message); err != nil {
		return err
	}
	r.recordUsage(message)
	updated, err := r.service.refreshSessionMetaAfterMessage(r.workspacePath, r.session, message)
	if err != nil {
		return err
	}
	if updated != nil {
		r.session = *updated
	}
	return nil
}

func (r *roundRunner) refreshSessionMetaAfterRoundFinished() {
	updated, err := r.service.refreshSessionMetaRuntimeState(r.workspacePath, r.session)
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
