package dm

import (
	"context"
	"errors"
	"strings"
	"time"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

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
	r.recordGoalContinuationProgress(runtimectx.RoundExecutionResult{
		TerminalStatus: "error",
		ErrorMessage:   err.Error(),
	})
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
	if persistErr := r.service.history.AppendOverlayMessage(
		r.workspacePath,
		r.session.SessionKey,
		resultMessage,
	); persistErr != nil {
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
	errorEvent.RoundID = r.roundID
	errorEvent.AgentRoundID = r.agentRoundID
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
		fields = append(fields, runtimectx.RoundStreamStopDiagnosticLogFields(streamClosed.LastStreamStop)...)
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
		fields = append(fields, runtimectx.RoundStreamStopDiagnosticLogFields(streamIdle.LastStreamStop)...)
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
	r.recordGoalUsage(context.Background(), runtimectx.RoundExecutionResult{}, r.lastGoalAssistantMessage())
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
	if persistErr := r.service.history.AppendOverlayMessage(
		r.workspacePath,
		r.session.SessionKey,
		resultMessage,
	); persistErr != nil {
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
