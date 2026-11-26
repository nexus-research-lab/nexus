// INPUT: 带最终准入器的后台 round（Echo）的终态与最后一条 assistant。
// OUTPUT: 通过准入的单条 durable assistant 消息；未通过时删除本轮 runtime 历史，不留痕迹。
// POS: 延迟提交 round 的收口；准入判断由发起方提供，这里只负责落盘、广播与清理。
package dm

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
)

const (
	deferredStatusDelivered = "delivered"
	deferredStatusFailed    = "failed"
	deferredStatusCancelled = "cancelled"
)

func (r *roundRunner) finishDeferredAssistant(result exec.RoundExecutionResult) {
	hooks := r.deferredAssistant
	if hooks == nil || hooks.Admit == nil {
		r.finishDeferredFailure(deferredStatusFailed, errors.New("后台 assistant 准入器未装配"))
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
		r.completeDeferredAssistant(DeferredAssistantOutcome{Status: deferredStatusFailed, Error: err})
		r.finishDeferredRuntime(false)
		return
	}
	if !admitted {
		r.finishDeferredRuntime(false)
		return
	}
	if protocol.MessageRole(message) != "assistant" {
		err = errors.New("后台准入结果不是 assistant 消息")
	} else {
		err = r.persistMessage(message)
	}
	if err == nil {
		r.RecordTerminalAssistantUsage(message, r.writeUsage)
		r.broadcastDeferredAssistant(message)
		r.completeDeferredAssistant(DeferredAssistantOutcome{Status: deferredStatusDelivered})
	} else {
		r.completeDeferredAssistant(DeferredAssistantOutcome{Status: deferredStatusFailed, Error: err})
	}
	r.finishDeferredRuntime(err == nil)
}

func (r *roundRunner) finishDeferredFailure(status string, err error) {
	r.completeDeferredAssistant(DeferredAssistantOutcome{Status: status, Error: err})
	r.finishDeferredRuntime(false)
}

func (r *roundRunner) completeDeferredAssistant(outcome DeferredAssistantOutcome) {
	if r.deferredAssistant == nil || r.deferredAssistant.Complete == nil {
		return
	}
	r.deferredAssistant.Complete(context.Background(), outcome)
}

func (r *roundRunner) finishDeferredRuntime(preserveTranscript bool) {
	if !preserveTranscript {
		if err := r.discardDeferredRuntimeMessages(); err != nil {
			r.service.LoggerFor(context.Background()).Error("删除未投递后台 assistant runtime 历史失败",
				"session_key", r.sessionKey,
				"round_id", r.roundID,
				"err", err,
			)
		}
	}
	r.service.Runtime.MarkRoundTerminal(r.sessionKey, r.roundID)
	r.refreshSessionMetaAfterRoundFinished()
	r.dispatchNextInputQueueItem()
}

func (r *roundRunner) discardDeferredRuntimeMessages() error {
	if r == nil || r.client == nil {
		return nil
	}
	messageUUIDs := make([]string, 0, len(r.deferredRuntimeMessageUUIDs)+1)
	if uuid := strings.TrimSpace(r.userMessageID); uuid != "" {
		messageUUIDs = append(messageUUIDs, uuid)
	}
	messageUUIDs = append(messageUUIDs, r.deferredRuntimeMessageUUIDs...)
	if len(messageUUIDs) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), runtimectx.RoundIdleAbortTimeout)
	defer cancel()
	return r.client.RemoveMessages(ctx, messageUUIDs)
}

func (r *roundRunner) broadcastDeferredAssistant(message protocol.Message) {
	event := protocol.NewEvent(protocol.EventTypeMessage, protocol.Clone(message))
	event.SessionKey = r.sessionKey
	event.AgentID = r.agent.AgentID
	event.RoundID = r.roundID
	event.AgentRoundID = r.agentRoundID
	event.MessageID = textutil.AnyString(message["message_id"])
	event.DeliveryMode = protocol.DeliveryModeDurable
	r.service.broadcastEventWithTimeout(context.Background(), r.sessionKey, event)
}
