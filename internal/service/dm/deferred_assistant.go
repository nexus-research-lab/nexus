// INPUT: 带最终准入器的延迟提交 round（Echo 与主窗口宿主 run）的终态与最后一条 assistant。
// OUTPUT: 通过准入的单条 overlay assistant 消息与一次结局回调；未提交时按需删除本轮 runtime 历史。
// POS: 延迟提交 round 的收口；准入与结局记录由发起方提供，这里只负责落盘、广播与清理。
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
	// deferredStatusQuiet 表示本轮正常结束但没有可提交的回复（沉默标记或空输出）。
	deferredStatusQuiet     = "quiet"
	deferredStatusFailed    = "failed"
	deferredStatusCancelled = "cancelled"
)

// silent 标出不公告起止的延迟提交轮（Echo）：用户在它提交前看不到任何痕迹。
func (h *DeferredAssistantHooks) silent() bool {
	return h != nil && !h.Announce
}

func (r *roundRunner) finishDeferredAssistant(result exec.RoundExecutionResult) {
	hooks := r.deferredAssistant
	if hooks == nil || hooks.Admit == nil {
		r.finishDeferredFailure(result, deferredStatusFailed, errors.New("后台 assistant 准入器未装配"))
		return
	}
	switch {
	case result.TerminalStatus == "interrupted":
		r.finishDeferredFailure(result, deferredStatusCancelled, errors.New(strings.TrimSpace(result.ErrorMessage)))
		return
	case result.TerminalStatus != "finished" || (result.ResultSubtype != "" && result.ResultSubtype != "success"):
		r.finishDeferredFailure(result, deferredStatusFailed, errors.New(strings.TrimSpace(result.ErrorMessage)))
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
		r.finishDeferredFailure(result, deferredStatusFailed, err)
		return
	}
	if !admitted {
		r.completeDeferredAssistant(DeferredAssistantOutcome{Status: deferredStatusQuiet})
		r.finishDeferredRuntime(result, false)
		return
	}
	if protocol.MessageRole(message) != "assistant" {
		err = errors.New("后台准入结果不是 assistant 消息")
	} else {
		err = r.persistDeferredMessage(message)
	}
	if err == nil {
		r.RecordTerminalAssistantUsage(message, r.writeUsage)
		r.broadcastDeferredAssistant(message)
		r.completeDeferredAssistant(DeferredAssistantOutcome{Status: deferredStatusDelivered})
	} else {
		r.completeDeferredAssistant(DeferredAssistantOutcome{Status: deferredStatusFailed, Error: err})
	}
	r.finishDeferredRuntime(result, err == nil)
}

func (r *roundRunner) finishDeferredFailure(result exec.RoundExecutionResult, status string, err error) {
	r.completeDeferredAssistant(DeferredAssistantOutcome{Status: status, Error: err})
	// 失败与中断可能来自执行之外（启动、持久化），终态事件按结局补齐。
	if status == deferredStatusCancelled {
		result.TerminalStatus, result.ResultSubtype = "interrupted", "interrupted"
	} else if result.TerminalStatus != "error" {
		result.TerminalStatus, result.ResultSubtype = "error", "error"
		if err != nil && result.ErrorMessage == "" {
			result.ErrorMessage = err.Error()
		}
	}
	r.finishDeferredRuntime(result, false)
}

func (r *roundRunner) completeDeferredAssistant(outcome DeferredAssistantOutcome) {
	if r.deferredAssistant == nil || r.deferredAssistant.Complete == nil {
		return
	}
	r.deferredAssistant.Complete(context.Background(), outcome)
}

// persistDeferredMessage 把准入后的 assistant 写进 overlay：延迟提交轮的 transcript 过程不进入用户历史，
// overlay 是这条回复唯一的可见来源。
func (r *roundRunner) persistDeferredMessage(message protocol.Message) error {
	if err := r.service.History.ForOwner(r.ownerUserID).AppendOverlayMessage(
		r.workspacePath,
		r.session.SessionKey,
		message,
	); err != nil {
		return err
	}
	return r.refreshSessionMetaAfterMessage(message)
}

func (r *roundRunner) finishDeferredRuntime(result exec.RoundExecutionResult, committed bool) {
	hooks := r.deferredAssistant
	if !committed && (hooks == nil || !hooks.KeepTranscript) {
		if err := r.discardDeferredRuntimeMessages(); err != nil {
			r.service.LoggerFor(context.Background()).Error("删除未投递后台 assistant runtime 历史失败",
				"session_key", r.sessionKey,
				"round_id", r.roundID,
				"err", err,
			)
		}
	}
	r.service.Runtime.MarkRoundTerminal(r.sessionKey, r.roundID)
	if hooks != nil && hooks.Announce {
		// 公告过起点的轮次必须同样公告终态，否则客户端的运行态停不下来。
		r.service.broadcastEventWithTimeout(context.Background(), r.sessionKey, terminalRoundStatusEvent(r, result))
		r.service.broadcastSessionStatus(context.Background(), r.sessionKey)
	}
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
