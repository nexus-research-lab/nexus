// INPUT: 主窗口一轮可见用户输入的成功终态、新的用户输入，以及到点的 echo 事件。
// OUTPUT: 安静等待后才出现的 echo 事件；到点由 Echo gate 判断，开口时起一轮隐藏、无工具、延迟提交的 round。
// POS: Echo 在主窗口里的全部生命周期；Echo 域只给判断（EchoPolicy），状态全在 inbox 行上。
package dm

import (
	"context"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	inboxstore "github.com/nexus-research-lab/nexus/internal/storage/inbox"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

const (
	echoPurpose         = "echo_followup"
	echoMaxOutputTokens = 400
)

// scheduleMainWindowEcho 只在主窗口一轮可见用户输入成功结束后安排 Echo；宿主事件与 Echo 自身的轮次不递归产生。
func (r *roundRunner) scheduleMainWindowEcho(result exec.RoundExecutionResult) {
	if r == nil || r.service.echo == nil || r.service.inbox == nil || r.internal ||
		!result.CompletedByAssistant || result.TerminalStatus != "finished" ||
		(result.ResultSubtype != "" && result.ResultSubtype != "success") ||
		!isMainWindowSession(r.agent, protocol.ParseSessionKey(r.sessionKey)) {
		return
	}
	target := mainInboxTarget{agent: r.agent, sessionKey: r.sessionKey}
	anchorRoundID, finishedAt := r.roundID, time.Now().UTC()
	r.service.StartSessionBackgroundTask(r.sessionKey, r.ownerUserID, func(ctx context.Context) {
		r.service.scheduleEcho(ctx, target, anchorRoundID, finishedAt)
	})
}

// scheduleEcho 让新的 echo 取代旧的：同一主窗口最多一条待触发 Echo，锚定最近一轮对话。
func (s *Service) scheduleEcho(ctx context.Context, target mainInboxTarget, anchorRoundID string, finishedAt time.Time) {
	availableAt, ok, err := s.echo.ScheduleAfter(ctx, target.owner(), finishedAt)
	if err != nil {
		s.LoggerFor(ctx).Warn("Echo 计算到点时间失败", "session_key", target.sessionKey, "err", err)
		return
	}
	if !ok {
		return
	}
	if _, err = s.inbox.CancelKind(ctx, target.sessionKey, inboxstore.KindEcho); err != nil {
		s.LoggerFor(ctx).Warn("取消旧 Echo 失败", "session_key", target.sessionKey, "err", err)
		return
	}
	if _, _, err = s.inbox.Append(ctx, inboxstore.Event{
		EventID:     newInboxEventID(),
		OwnerUserID: target.owner(),
		InboxKey:    target.sessionKey,
		Kind:        inboxstore.KindEcho,
		DedupeKey:   "echo:" + anchorRoundID,
		AvailableAt: availableAt,
	}); err != nil {
		s.LoggerFor(ctx).Warn("写入 Echo 失败", "session_key", target.sessionKey, "err", err)
		return
	}
	s.notifyMainInboxTimer()
}

// cancelMainWindowEcho 在用户开口时让 Echo 让路：pending 与正在 gate 的 delivering 一并失效。
func (s *Service) cancelMainWindowEcho(ctx context.Context, target mainInboxTarget) {
	if _, err := s.inbox.CancelKind(ctx, target.sessionKey, inboxstore.KindEcho); err != nil {
		s.LoggerFor(ctx).Warn("取消 Echo 失败", "session_key", target.sessionKey, "err", err)
	}
}

// runMainWindowEcho 处理已认领（delivering）的 echo：gate 是一次模型调用，必须在派发锁外进行；
// 开口前在锁内用条件迁移确认事件仍归自己，期间到来的用户输入已把它标为 cancelled。
func (s *Service) runMainWindowEcho(ctx context.Context, target mainInboxTarget, roundID string, event inboxstore.Event) {
	decision, err := s.echo.Decide(ctx, EchoCandidate{
		OwnerUserID: target.owner(),
		AgentID:     target.agent.AgentID,
		SessionKey:  target.sessionKey,
	})
	if err != nil || decision.Prompt == "" {
		if err != nil {
			s.LoggerFor(ctx).Warn("Echo 判断失败，本次保持沉默", "session_key", target.sessionKey, "err", err)
		}
		s.settleEcho(ctx, target, roundID, event, decision)
		s.drainMainInbox(ctx, target)
		return
	}
	if !s.startEchoRound(ctx, target, roundID, event, decision.Prompt) {
		s.drainMainInbox(ctx, target)
	}
}

// settleEcho 收口不开口的 echo：顺延的退回 pending 等下次到点，其余记为已处理。
func (s *Service) settleEcho(ctx context.Context, target mainInboxTarget, roundID string, event inboxstore.Event, decision EchoDecision) {
	var err error
	if !decision.DeferUntil.IsZero() {
		if _, err = s.inbox.Defer(ctx, target.sessionKey, event.EventID, decision.DeferUntil); err == nil {
			s.notifyMainInboxTimer()
		}
	} else {
		_, err = s.inbox.Transition(ctx, target.sessionKey, []string{event.EventID}, inboxstore.StatusDelivering, inboxstore.StatusConsumed, roundID)
	}
	if err != nil {
		s.LoggerFor(ctx).Warn("Echo 收口失败", "session_key", target.sessionKey, "reason", decision.Reason, "err", err)
	}
}

func (s *Service) startEchoRound(ctx context.Context, target mainInboxTarget, roundID string, event inboxstore.Event, prompt string) bool {
	if err := s.inputQueueDispatchMu.LockContext(ctx); err != nil {
		return false
	}
	defer s.inputQueueDispatchMu.Unlock()
	// gate 期间若有别的轮次起跑，对话已不安静，本次作废。
	if len(s.Runtime.GetRunningRoundIDs(target.sessionKey)) > 0 {
		_, _ = s.inbox.Transition(ctx, target.sessionKey, []string{event.EventID}, inboxstore.StatusDelivering, inboxstore.StatusCancelled, "")
		return false
	}
	moved, err := s.inbox.Transition(ctx, target.sessionKey, []string{event.EventID}, inboxstore.StatusDelivering, inboxstore.StatusConsumed, roundID)
	if err != nil || len(moved) == 0 {
		return false
	}
	if err = s.handleChat(ctx, s.echoRequest(target, roundID, event, prompt), chatExecutionInline); err != nil {
		s.LoggerFor(ctx).Warn("Echo 起轮失败", "session_key", target.sessionKey, "err", err)
		return false
	}
	return true
}

func (s *Service) echoRequest(target mainInboxTarget, roundID string, event inboxstore.Event, prompt string) Request {
	return Request{
		SessionKey:      target.sessionKey,
		AgentID:         target.agent.AgentID,
		Content:         prompt,
		RoundID:         roundID,
		Internal:        true,
		ExecutionOrigin: "echo",
		InputOptions: sdkprotocol.OutboundMessageOptions{
			HiddenFromUser:  true,
			Synthetic:       true,
			Purpose:         echoPurpose,
			ToolAccess:      "none",
			MaxOutputTokens: echoMaxOutputTokens,
			Metadata:        map[string]string{"echo_event_id": event.EventID},
		},
		DeferredAssistant: &DeferredAssistantHooks{
			Admit: func(admitCtx context.Context, candidate DeferredAssistantCandidate) (protocol.Message, bool, error) {
				return s.admitEcho(admitCtx, target, event, candidate)
			},
		},
	}
}

// admitEcho 是提交前复核：沉默，或生成期间用户已经开口，都丢弃本轮且不留痕迹（失败在准入前已分流）。
func (s *Service) admitEcho(ctx context.Context, target mainInboxTarget, event inboxstore.Event, candidate DeferredAssistantCandidate) (protocol.Message, bool, error) {
	if isHostNoReply(candidate.Message) {
		return nil, false, nil
	}
	ctx = runtimehost.ContextWithExactOwner(ctx, target.owner())
	open, err := s.inbox.ListOpen(ctx, target.sessionKey, time.Now().UTC())
	if err != nil {
		return nil, false, err
	}
	for _, pending := range open {
		if pending.Kind == inboxstore.KindUserMessage {
			return nil, false, nil
		}
	}
	message := protocol.Clone(candidate.Message)
	if textutil.AnyString(message["message_id"]) == "" {
		message["message_id"] = protocol.NewAssistantMessageID()
	}
	metadata := map[string]any{}
	if current, ok := message["metadata"].(map[string]any); ok {
		for key, value := range current {
			metadata[key] = value
		}
	}
	metadata["source"] = "echo"
	metadata["echo_event_id"] = event.EventID
	message["metadata"] = metadata
	return message, true, nil
}
