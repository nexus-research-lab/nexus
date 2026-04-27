package chat

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	permission3 "github.com/nexus-research-lab/nexus/internal/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

type chatRoundMapperAdapter struct {
	mapper *messageMapper
}

func (a chatRoundMapperAdapter) Map(
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

func (a chatRoundMapperAdapter) SessionID() string {
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
	client            runtimectx.Client
	runtimeProvider   string
	runtimeModel      string
	ownerUserID       string
	mapper            *messageMapper
	permissionMode    sdkprotocol.PermissionMode
	permissionHandler agentclient.PermissionHandler
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
	r.service.runtime.MarkRoundFinished(r.sessionKey, r.roundID)
	r.service.permission.BroadcastEvent(
		context.Background(),
		r.sessionKey,
		protocol.NewRoundStatusEvent(r.sessionKey, r.roundID, result.TerminalStatus, result.ResultSubtype),
	)
	r.service.broadcastSessionStatus(context.Background(), r.sessionKey)
	go r.service.dispatchNextInputQueueItem(contextWithQueueOwner(context.Background(), r.ownerUserID), r.sessionKey, r.agent.AgentID)
}

func (r *roundRunner) executeRound(
	ctx context.Context,
	logger *slog.Logger,
) (runtimectx.RoundExecutionResult, error) {
	return runtimectx.ExecuteRound(ctx, runtimectx.RoundExecutionRequest{
		Query:  r.content,
		Client: r.client,
		Mapper: chatRoundMapperAdapter{mapper: r.mapper},
		InterruptReason: func() string {
			return r.service.runtime.GetInterruptReason(r.sessionKey, r.roundID)
		},
		ObserveIncomingMessage: func(incoming sdkprotocol.ReceivedMessage) {
			logger.Debug("Agent ", runtimectx.BuildSDKMessageLogFields(incoming)...)
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
			if message["role"] == "assistant" {
				r.service.permission.BindSessionRoute(r.sessionKey, permission3.RouteContext{
					DispatchSessionKey: r.sessionKey,
					AgentID:            r.agent.AgentID,
					MessageID:          normalizeString(message["message_id"]),
					CausedBy:           r.roundID,
				})
			}
			return nil
		},
		EmitEvent: func(event protocol.EventMessage) error {
			r.service.permission.BroadcastEvent(context.Background(), r.sessionKey, event)
			return nil
		},
	})
}

func (r *roundRunner) failRound(err error) {
	if interruptReason := r.service.runtime.GetInterruptReason(r.sessionKey, r.roundID); interruptReason != "" {
		r.finishInterrupted(interruptReason)
		return
	}
	r.service.loggerFor(context.Background()).Error("DM round 执行失败",
		"session_key", r.sessionKey,
		"agent_id", r.agent.AgentID,
		"round_id", r.roundID,
		"err", err,
	)
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
		"session_id":      firstNonEmpty(r.client.SessionID(), persistedSessionID),
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
		event.MessageID = normalizeString(event.Data["message_id"])
		event.DeliveryMode = "durable"
		r.service.permission.BroadcastEvent(context.Background(), r.sessionKey, event)
	}
	errorEvent := protocol.NewErrorEvent(r.sessionKey, err.Error())
	errorEvent.AgentID = r.agent.AgentID
	errorEvent.CausedBy = r.roundID
	if messageID := strings.TrimSpace(r.mapper.CurrentMessageID()); messageID != "" {
		errorEvent.MessageID = messageID
	}
	r.service.permission.BroadcastEvent(context.Background(), r.sessionKey, errorEvent)
	r.service.permission.BroadcastEvent(
		context.Background(),
		r.sessionKey,
		protocol.NewRoundStatusEvent(r.sessionKey, r.roundID, "error", "error"),
	)
	r.service.broadcastSessionStatus(context.Background(), r.sessionKey)
	go r.service.dispatchNextInputQueueItem(contextWithQueueOwner(context.Background(), r.ownerUserID), r.sessionKey, r.agent.AgentID)
}

func (r *roundRunner) finishInterrupted(resultText string) {
	r.service.loggerFor(context.Background()).Warn("DM round 以中断状态结束",
		"session_key", r.sessionKey,
		"agent_id", r.agent.AgentID,
		"round_id", r.roundID,
		"reason", resultText,
	)
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
		"session_id":      firstNonEmpty(r.client.SessionID(), persistedSessionID),
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
		event.MessageID = normalizeString(event.Data["message_id"])
		event.DeliveryMode = "durable"
		r.service.permission.BroadcastEvent(context.Background(), r.sessionKey, event)
	}
	r.service.permission.BroadcastEvent(
		context.Background(),
		r.sessionKey,
		protocol.NewRoundStatusEvent(r.sessionKey, r.roundID, "interrupted", "interrupted"),
	)
	r.service.broadcastSessionStatus(context.Background(), r.sessionKey)
	go r.service.dispatchNextInputQueueItem(contextWithQueueOwner(context.Background(), r.ownerUserID), r.sessionKey, r.agent.AgentID)
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

func (r *roundRunner) recordUsage(message protocol.Message) {
	if r.service.usage == nil || protocol.MessageRole(message) != "result" {
		return
	}
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
