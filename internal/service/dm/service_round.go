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
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
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
) (runtimectx.RoundMapResult, error) {
	events, durableMessages, terminalStatus, resultSubtype, err := a.mapper.Map(incoming, interruptReason...)
	if err != nil {
		return runtimectx.RoundMapResult{}, err
	}
	return runtimectx.RoundMapResult{
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
	service           *Service
	workspacePath     string
	session           protocol.Session
	agent             *protocol.Agent
	sessionKey        string
	roundID           string
	reqID             string
	content           string
	runtimeContent    conversationsvc.RuntimeContent
	client            runtimectx.Client
	runtimeProvider   string
	runtimeModel      string
	ownerUserID       string
	mapper            *dmdomain.MessageMapper
	inputOptions      sdkprotocol.OutboundMessageOptions
	internal          bool
	goalIDForUsage    string
	goalUsage         *goalsvc.RuntimeUsageAccumulator
	goalUsageStarted  time.Time
	goalUsageMu       sync.Mutex
	goalLastAssistant protocol.Message
	permissionMode    sdkpermission.Mode
	permissionHandler sdkpermission.Handler
}

func (r *roundRunner) run(ctx context.Context) {
	logger := r.service.loggerFor(ctx).With(
		"session_key", r.sessionKey,
		"agent_id", r.agent.AgentID,
		"round_id", r.roundID,
	)
	logger.Info("开始执行 DM round")
	result, err := r.executeRound(ctx, logger)
	if err != nil {
		if errors.Is(err, runtimectx.ErrRoundInterrupted) {
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
	)
	r.recordGoalUsage(result, r.mapper.LastAssistantMessage())
	r.recordGoalUsageLimit(result)
	if result.CompletedByAssistant {
		r.recordTerminalAssistantUsage(r.mapper.LastAssistantMessage())
		go r.commitMemoryTurn()
	}
	r.service.runtime.MarkRoundFinished(r.sessionKey, r.roundID)
	r.refreshSessionMetaAfterRoundFinished()
	r.service.broadcastEventWithTimeout(
		context.Background(),
		r.sessionKey,
		protocol.NewRoundStatusEvent(r.sessionKey, r.roundID, result.TerminalStatus, result.ResultSubtype),
	)
	r.service.broadcastSessionStatus(context.Background(), r.sessionKey)
	r.dispatchPostRoundWork()
}

func (r *roundRunner) executeRound(
	ctx context.Context,
	logger *slog.Logger,
) (runtimectx.RoundExecutionResult, error) {
	return runtimectx.ExecuteRound(ctx, runtimectx.RoundExecutionRequest{
		Content:      r.runtimeContent.Payload(),
		InputOptions: r.inputOptions,
		Client:       r.client,
		Mapper:       dmRoundMapperAdapter{mapper: r.mapper},
		InterruptReason: func() string {
			return r.service.runtime.GetInterruptReason(r.sessionKey, r.roundID)
		},
		ObserveIncomingMessage: func(incoming sdkprotocol.ReceivedMessage) {
			if incoming.Type == sdkprotocol.MessageTypeStreamEvent && !r.service.config.MessageDebugStreamEvent {
				return
			}
			logger.Debug(
				"Agent ",
				runtimectx.BuildSDKMessageLogFieldsWithOptions(
					incoming,
					runtimectx.SDKMessageLogOptions{
						IncludeStreamEvent:  r.service.config.MessageDebugStreamEvent,
						IncludeSnapshotData: true,
					},
				)...,
			)
		},
		SyncSessionID: func(sessionID string) error {
			updatedSession, syncErr := r.service.syncSDKSessionID(
				context.Background(),
				r.workspacePath,
				r.session,
				sessionID,
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
			if err := r.persistMessage(message); err != nil {
				return err
			}
			r.rememberGoalAssistantMessage(message)
			r.recordGoalUsageFromAssistantMessage(message)
			if message["role"] == "assistant" {
				r.service.permission.BindSessionRoute(r.sessionKey, permissionctx.RouteContext{
					DispatchSessionKey: r.sessionKey,
					AgentID:            r.agent.AgentID,
					MessageID:          dmdomain.NormalizeString(message["message_id"]),
					CausedBy:           r.roundID,
				})
			}
			return nil
		},
		EmitEvent: func(event protocol.EventMessage) error {
			r.service.broadcastEventWithTimeout(context.Background(), r.sessionKey, event)
			return nil
		},
	})
}

func (r *roundRunner) failRound(err error) {
	if interruptReason := r.service.runtime.GetInterruptReason(r.sessionKey, r.roundID); interruptReason != "" {
		r.finishInterrupted(interruptReason)
		return
	}
	fields := []any{
		"session_key", r.sessionKey,
		"agent_id", r.agent.AgentID,
		"round_id", r.roundID,
		"err", err,
	}
	fields = append(fields, dmRoundFailureDiagnostics(err, r)...)
	r.service.loggerFor(context.Background()).Error("DM round 执行失败", fields...)
	r.service.runtime.MarkRoundFinished(r.sessionKey, r.roundID)
	persistedSessionID := ""
	if r.session.SessionID != nil {
		persistedSessionID = strings.TrimSpace(*r.session.SessionID)
	}
	resultMessage := protocol.Message{
		"message_id":      "result_" + r.roundID,
		"session_key":     r.sessionKey,
		"agent_id":        r.agent.AgentID,
		"round_id":        r.roundID,
		"session_id":      dmdomain.FirstNonEmpty(r.client.SessionID(), persistedSessionID),
		"role":            "result",
		"timestamp":       time.Now().UnixMilli(),
		"subtype":         "error",
		"duration_ms":     0,
		"duration_api_ms": 0,
		"num_turns":       0,
		"usage":           map[string]any{},
		"result":          err.Error(),
		"is_error":        true,
	}
	if persistErr := r.service.appendSyntheticHistoryMessage(r.workspacePath, r.session, resultMessage); persistErr != nil {
		r.service.loggerFor(context.Background()).Error("DM 错误结果持久化失败",
			"session_key", r.sessionKey,
			"agent_id", r.agent.AgentID,
			"round_id", r.roundID,
			"err", persistErr,
		)
	} else {
		if updated, updateErr := r.service.refreshSessionMetaAfterMessage(r.workspacePath, r.session, resultMessage); updateErr != nil {
			r.service.loggerFor(context.Background()).Error("DM 错误结果刷新 session meta 失败",
				"session_key", r.sessionKey,
				"agent_id", r.agent.AgentID,
				"round_id", r.roundID,
				"err", updateErr,
			)
		} else if updated != nil {
			r.session = *updated
		}
		event := protocol.NewEvent(protocol.EventTypeMessage, r.mapper.ProjectResultMessage(resultMessage))
		event.SessionKey = r.sessionKey
		event.AgentID = r.agent.AgentID
		event.MessageID = dmdomain.NormalizeString(event.Data["message_id"])
		event.DeliveryMode = "durable"
		r.service.broadcastEventWithTimeout(context.Background(), r.sessionKey, event)
	}
	errorEvent := protocol.NewErrorEvent(r.sessionKey, err.Error())
	r.refreshSessionMetaAfterRoundFinished()
	errorEvent.AgentID = r.agent.AgentID
	errorEvent.CausedBy = r.roundID
	if messageID := strings.TrimSpace(r.mapper.CurrentMessageID()); messageID != "" {
		errorEvent.MessageID = messageID
	}
	r.service.broadcastEventWithTimeout(context.Background(), r.sessionKey, errorEvent)
	r.service.broadcastEventWithTimeout(
		context.Background(),
		r.sessionKey,
		protocol.NewRoundStatusEvent(r.sessionKey, r.roundID, "error", "error"),
	)
	r.service.broadcastSessionStatus(context.Background(), r.sessionKey)
	r.dispatchNextInputQueueItem()
}

func dmRoundFailureDiagnostics(err error, runner *roundRunner) []any {
	fields := make([]any, 0, 16)
	var streamClosed *runtimectx.RoundStreamClosedError
	if errors.As(err, &streamClosed) {
		fields = append(fields,
			"stream_messages_seen", streamClosed.MessagesSeen,
			"stream_last_type", streamClosed.LastMessageType,
			"stream_last_summary", streamClosed.LastMessageSummary,
			"stream_last_session_id", streamClosed.LastSessionID,
			"stream_last_message_id", streamClosed.LastMessageID,
			"stream_wait_error", streamClosed.WaitError,
		)
	}
	var streamIdle *runtimectx.RoundStreamIdleTimeoutError
	if errors.As(err, &streamIdle) {
		fields = append(fields,
			"stream_idle_timeout", streamIdle.IdleTimeout.String(),
			"stream_messages_seen", streamIdle.MessagesSeen,
			"stream_last_type", streamIdle.LastMessageType,
			"stream_last_summary", streamIdle.LastMessageSummary,
			"stream_last_session_id", streamIdle.LastSessionID,
			"stream_last_message_id", streamIdle.LastMessageID,
		)
	}
	if runner != nil && runner.client != nil {
		fields = append(fields, "client_session_id", runner.client.SessionID())
	}
	return fields
}

func (r *roundRunner) finishInterrupted(resultText string) {
	r.service.loggerFor(context.Background()).Warn("DM round 以中断状态结束",
		"session_key", r.sessionKey,
		"agent_id", r.agent.AgentID,
		"round_id", r.roundID,
		"reason", resultText,
	)
	r.recordGoalUsage(runtimectx.RoundExecutionResult{}, r.lastGoalAssistantMessage())
	r.service.runtime.MarkRoundFinished(r.sessionKey, r.roundID)
	persistedSessionID := ""
	if r.session.SessionID != nil {
		persistedSessionID = strings.TrimSpace(*r.session.SessionID)
	}
	resultMessage := protocol.Message{
		"message_id":      "result_" + r.roundID,
		"session_key":     r.sessionKey,
		"agent_id":        r.agent.AgentID,
		"round_id":        r.roundID,
		"session_id":      dmdomain.FirstNonEmpty(r.client.SessionID(), persistedSessionID),
		"role":            "result",
		"timestamp":       time.Now().UnixMilli(),
		"subtype":         "interrupted",
		"duration_ms":     0,
		"duration_api_ms": 0,
		"num_turns":       0,
		"usage":           map[string]any{},
		"is_error":        false,
	}
	if trimmedResult := strings.TrimSpace(resultText); trimmedResult != "" {
		resultMessage["result"] = trimmedResult
	}
	if persistErr := r.service.appendSyntheticHistoryMessage(r.workspacePath, r.session, resultMessage); persistErr != nil {
		r.service.loggerFor(context.Background()).Error("DM interrupted 结果持久化失败",
			"session_key", r.sessionKey,
			"agent_id", r.agent.AgentID,
			"round_id", r.roundID,
			"err", persistErr,
		)
	} else {
		if updated, updateErr := r.service.refreshSessionMetaAfterMessage(r.workspacePath, r.session, resultMessage); updateErr != nil {
			r.service.loggerFor(context.Background()).Error("DM interrupted 刷新 session meta 失败",
				"session_key", r.sessionKey,
				"agent_id", r.agent.AgentID,
				"round_id", r.roundID,
				"err", updateErr,
			)
		} else if updated != nil {
			r.session = *updated
		}
		event := protocol.NewEvent(protocol.EventTypeMessage, r.mapper.ProjectResultMessage(resultMessage))
		event.SessionKey = r.sessionKey
		event.AgentID = r.agent.AgentID
		event.MessageID = dmdomain.NormalizeString(event.Data["message_id"])
		event.DeliveryMode = "durable"
		r.service.broadcastEventWithTimeout(context.Background(), r.sessionKey, event)
	}
	r.refreshSessionMetaAfterRoundFinished()
	r.service.broadcastEventWithTimeout(
		context.Background(),
		r.sessionKey,
		protocol.NewRoundStatusEvent(r.sessionKey, r.roundID, "interrupted", "interrupted"),
	)
	r.service.broadcastSessionStatus(context.Background(), r.sessionKey)
	r.dispatchNextInputQueueItem()
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
	r.writeUsage(message)
}

func (r *roundRunner) recordTerminalAssistantUsage(message protocol.Message) {
	if r.service.usage == nil || protocol.MessageRole(message) != "assistant" {
		return
	}
	r.writeUsage(message)
}

func (r *roundRunner) writeUsage(message protocol.Message) {
	input := usagesvc.MessageRecordInput(r.ownerUserID, "dm_runtime", message)
	if err := r.service.usage.RecordMessageUsage(context.Background(), input); err != nil {
		r.service.loggerFor(context.Background()).Error("DM token usage 写入失败",
			"session_key", r.sessionKey,
			"agent_id", r.agent.AgentID,
			"round_id", r.roundID,
			"err", err,
		)
	}
}
