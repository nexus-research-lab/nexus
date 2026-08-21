// INPUT: 普通 DM 用户活动/成功终态与静默后台 round 的最终 assistant。
// OUTPUT: Echo 取消、调度回调和最终准入后的单条 durable 消息。
// POS: DM 与 Echo 领域之间的窄生命周期边界。
package dm

import (
	"context"
	"errors"
	"strings"
	"time"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
)

const (
	echoDeferredStatusDelivered = "delivered"
	echoDeferredStatusFailed    = "failed"
	echoDeferredStatusCancelled = "cancelled"
)

func (e *dmChatExecution) cancelEchoForUserActivity() {
	if e == nil || e.request.Internal || e.service.echoHooks.OnUserActivity == nil ||
		protocol.NormalizeSessionKeyChannelSegment(e.parsed.Channel) != protocol.SessionChannelWebSocketSegment ||
		strings.TrimSpace(e.parsed.ChatType) != protocol.RoomTypeDM {
		return
	}
	roundIDs, err := e.service.echoHooks.OnUserActivity(
		e.ctx,
		strings.TrimSpace(e.agent.OwnerUserID),
		e.sessionKey,
	)
	if err != nil {
		e.service.loggerFor(e.ctx).Warn("取消 Echo 尝试失败",
			"session_key", e.sessionKey,
			"err", err,
		)
		return
	}
	for _, roundID := range roundIDs {
		if err = e.service.interruptExactRound(e.ctx, e.sessionKey, roundID); err != nil &&
			!errors.Is(err, ErrTargetDMRoundNotRunning) {
			e.service.loggerFor(e.ctx).Warn("中断 Echo round 失败",
				"session_key", e.sessionKey,
				"round_id", roundID,
				"err", err,
			)
		}
	}
}

func (r *roundRunner) scheduleEchoAfterTerminal(
	result exec.RoundExecutionResult,
	assistant protocol.Message,
) {
	if r == nil || r.service.echoHooks.OnTerminal == nil || r.internal ||
		strings.TrimSpace(r.executionOrigin) != "" || !result.CompletedByAssistant ||
		result.TerminalStatus != "finished" ||
		(result.ResultSubtype != "" && result.ResultSubtype != "success") ||
		protocol.NormalizeSessionKeyChannelSegment(protocol.ParseSessionKey(r.sessionKey).Channel) != protocol.SessionChannelWebSocketSegment ||
		strings.TrimSpace(protocol.ParseSessionKey(r.sessionKey).ChatType) != protocol.RoomTypeDM {
		return
	}
	terminal := EchoTerminalRound{
		OwnerUserID: strings.TrimSpace(r.ownerUserID),
		AgentID:     strings.TrimSpace(r.agent.AgentID),
		SessionKey:  r.sessionKey,
		RoundID:     r.roundID,
		AssistantID: dmdomain.NormalizeString(assistant["message_id"]),
		FinishedAt:  time.Now().UTC(),
	}
	r.service.startSessionBackgroundTask(r.sessionKey, r.ownerUserID, func(ctx context.Context) {
		r.service.echoHooks.OnTerminal(ctx, terminal)
	})
}

func (r *roundRunner) finishDeferredAssistant(result exec.RoundExecutionResult) {
	hooks := r.deferredAssistant
	if hooks == nil || hooks.Admit == nil {
		r.finishDeferredFailure(echoDeferredStatusFailed, errors.New("后台 assistant 准入器未装配"))
		return
	}
	candidate := DeferredAssistantCandidate{
		Message:        r.mapper.LastAssistantMessage(),
		TerminalStatus: result.TerminalStatus,
		ResultSubtype:  result.ResultSubtype,
		ErrorMessage:   strings.TrimSpace(result.ErrorMessage),
	}
	message, admitted, err := hooks.Admit(context.Background(), candidate)
	if err != nil {
		r.completeDeferredAssistant(DeferredAssistantOutcome{Status: echoDeferredStatusFailed, Error: err})
		r.finishDeferredRuntime()
		return
	}
	if !admitted {
		r.finishDeferredRuntime()
		return
	}
	if protocol.MessageRole(message) != "assistant" {
		err = errors.New("后台准入结果不是 assistant 消息")
	} else {
		err = r.persistMessage(message)
	}
	if err == nil {
		r.recordTerminalAssistantUsage(message)
		r.broadcastDeferredAssistant(message)
		r.completeDeferredAssistant(DeferredAssistantOutcome{Status: echoDeferredStatusDelivered})
	} else {
		r.completeDeferredAssistant(DeferredAssistantOutcome{Status: echoDeferredStatusFailed, Error: err})
	}
	r.finishDeferredRuntime()
}

func (r *roundRunner) finishDeferredFailure(status string, err error) {
	r.completeDeferredAssistant(DeferredAssistantOutcome{Status: status, Error: err})
	r.finishDeferredRuntime()
}

func (r *roundRunner) completeDeferredAssistant(outcome DeferredAssistantOutcome) {
	if r.deferredAssistant == nil || r.deferredAssistant.Complete == nil {
		return
	}
	r.deferredAssistant.Complete(context.Background(), outcome)
}

func (r *roundRunner) finishDeferredRuntime() {
	r.service.runtime.MarkRoundTerminal(r.sessionKey, r.roundID)
	r.refreshSessionMetaAfterRoundFinished()
	r.dispatchNextInputQueueItem()
}

func (r *roundRunner) broadcastDeferredAssistant(message protocol.Message) {
	event := protocol.NewEvent(protocol.EventTypeMessage, protocol.Clone(message))
	event.SessionKey = r.sessionKey
	event.AgentID = r.agent.AgentID
	event.RoundID = r.roundID
	event.AgentRoundID = r.agentRoundID
	event.MessageID = dmdomain.NormalizeString(message["message_id"])
	event.DeliveryMode = protocol.DeliveryModeDurable
	r.service.broadcastEventWithTimeout(context.Background(), r.sessionKey, event)
}
