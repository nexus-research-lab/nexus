// INPUT: 主窗口（主智能体的 WebSocket DM）的用户输入与已到点的宿主事件、PostToolUse 安全点、用户停止与 round 终态。
// OUTPUT: 先落库再 ACK 的 inbox 事件；空闲时按类型规格起轮（积压用户消息合并为一轮），运行中交给当前轮（running_input_v1 或 PostToolUse 安全点，宿主 run 除外）；
// 只有 pending 可撤回；用户开口即取消 Echo 并恢复暂停闸门，用户停止即暂停宿主恢复输入。
// POS: 主窗口输入的唯一入口；不经过 JSONL 待发送队列、投递策略与 queue admission。
// 单进程部署：并发正确性全部来自 inbox 行的条件状态迁移，不需要租约或内存登记表。
package dm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	inboxstore "github.com/nexus-research-lab/nexus/internal/storage/inbox"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
)

// ErrInboxEventNotPending 表示消息已被 agent 取走（或已撤回），不能再撤回。
var ErrInboxEventNotPending = errors.New("消息已被处理，无法撤回")

// mainInboxTarget 是一个主窗口：主智能体 + 它唯一的 WebSocket DM 会话。
type mainInboxTarget struct {
	agent      *protocol.Agent
	sessionKey string
}

type mainInboxInput struct {
	content         string
	attachments     []protocol.ChatAttachment
	clientMessageID string
	trusted         bool
}

func isMainWindowSession(agent *protocol.Agent, parsed protocol.SessionKey) bool {
	return agent != nil && agent.IsMain && dmRoomConversationID(parsed) != ""
}

// mainInboxTargetOf 在已解析出 Agent 的路径上判定主窗口；不是主窗口返回 nil。
// 每轮只在受理时算一次，之后由 dmChatExecution 与 roundRunner 持有。
func mainInboxTargetOf(agent *protocol.Agent, sessionKey string) *mainInboxTarget {
	if !isMainWindowSession(agent, protocol.ParseSessionKey(sessionKey)) {
		return nil
	}
	return &mainInboxTarget{agent: agent, sessionKey: sessionKey}
}

func (t mainInboxTarget) owner() string { return t.agent.OwnerUserID }

func (t mainInboxTarget) location() workspacestore.InputQueueLocation {
	return workspacestore.InputQueueLocation{
		OwnerUserID:   t.agent.OwnerUserID,
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: t.agent.WorkspacePath,
		SessionKey:    t.sessionKey,
	}
}

// resolveMainInboxTarget 只在会话是主窗口时返回 ok；其他 DM 继续走原有路径。
func (s *Service) resolveMainInboxTarget(ctx context.Context, sessionKey string, agentID string) (mainInboxTarget, bool, error) {
	normalized, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		return mainInboxTarget{}, false, err
	}
	parsed := protocol.ParseSessionKey(normalized)
	if dmRoomConversationID(parsed) == "" {
		return mainInboxTarget{}, false, nil
	}
	resolvedAgentID, err := s.resolveChatAgentID(ctx, parsed, agentID)
	if err != nil {
		return mainInboxTarget{}, false, err
	}
	agent, err := s.Agents.GetAgent(ctx, resolvedAgentID)
	if err != nil {
		return mainInboxTarget{}, false, err
	}
	if !isMainWindowSession(agent, parsed) {
		return mainInboxTarget{}, false, nil
	}
	if strings.TrimSpace(agent.OwnerUserID) != authctx.OwnerUserID(ctx) {
		return mainInboxTarget{}, false, errors.New("agent owner does not match request owner")
	}
	return mainInboxTarget{agent: agent, sessionKey: normalized}, true, nil
}

// acceptMainInbox 落库即受理；同一 client_message_id 重复提交返回原事件。
func (s *Service) acceptMainInbox(ctx context.Context, target mainInboxTarget, input mainInboxInput) (inboxstore.Event, bool, error) {
	attachments := s.normalizeChatAttachments(input.attachments, target.agent.AgentID)
	if !protocol.HasChatInput(input.content, attachments) {
		return inboxstore.Event{}, false, errors.New("content is required")
	}
	if len(strings.TrimSpace(input.clientMessageID)) > protocol.MaxClientMessageIDBytes {
		return inboxstore.Event{}, false, errors.New("client_message_id 过长")
	}
	eventID := newInboxEventID()
	event, duplicate, err := s.inbox.Append(ctx, inboxstore.Event{
		EventID:     eventID,
		OwnerUserID: target.owner(),
		InboxKey:    target.sessionKey,
		Kind:        inboxstore.KindUserMessage,
		DedupeKey:   textutil.FirstNonEmpty(input.clientMessageID, eventID),
		Payload: inboxstore.Payload{
			Content:         strings.TrimSpace(input.content),
			Attachments:     attachments,
			ClientMessageID: strings.TrimSpace(input.clientMessageID),
			Principal:       inboxPrincipal(ctx, target.owner(), input.trusted),
		},
	})
	if err != nil {
		return inboxstore.Event{}, false, err
	}
	if !duplicate {
		if err = s.resumeMainInbox(ctx, target); err != nil {
			return inboxstore.Event{}, false, err
		}
	}
	s.cancelMainWindowEcho(ctx, target)
	s.broadcastMainInbox(ctx, target)
	s.wakeMainInbox(target)
	return event, duplicate, nil
}

// acceptMainInboxChat 让主窗口的 chat 也进入 inbox；ACK 只确认落库，round 身份在被取走时才出现。
func (s *Service) acceptMainInboxChat(ctx context.Context, target mainInboxTarget, request Request) error {
	_, _, err := s.acceptMainInbox(ctx, target, mainInboxInput{
		content:         request.Content,
		attachments:     request.Attachments,
		clientMessageID: request.ClientMessageID,
		trusted:         request.TrustedConfigurationContext,
	})
	if err != nil {
		return err
	}
	s.broadcastEventWithTimeout(ctx, target.sessionKey, protocol.NewChatAckEvent(
		target.sessionKey, request.ClientRequestID, request.ClientMessageID, "", "", 0, false, nil,
	))
	return nil
}

// handleMainInboxControl 把主窗口的 input_queue 控制映射到 inbox：只有追加与撤回。
func (s *Service) handleMainInboxControl(ctx context.Context, target mainInboxTarget, request InputQueueRequest) (protocol.InputQueueMutationResult, error) {
	switch request.Action {
	case "", "enqueue":
		event, duplicate, err := s.acceptMainInbox(ctx, target, mainInboxInput{
			content:         request.Content,
			attachments:     request.Attachments,
			clientMessageID: request.ClientMessageID,
			trusted:         request.TrustedConfigurationContext,
		})
		return protocol.InputQueueMutationResult{Action: "enqueue", ItemID: event.EventID, Duplicate: duplicate}, err
	case "delete":
		err := s.withdrawMainInbox(ctx, target, request.ItemID)
		return protocol.InputQueueMutationResult{Action: "delete", ItemID: request.ItemID}, err
	default:
		return protocol.InputQueueMutationResult{}, fmt.Errorf("主窗口不支持 input_queue 操作: %s", request.Action)
	}
}

// inboxPrincipal 只为认证 WebSocket 入口记录真人身份；它决定取走时能否获得配置能力。
func inboxPrincipal(ctx context.Context, ownerUserID string, trusted bool) inboxstore.Principal {
	if !trusted {
		return inboxstore.Principal{}
	}
	binding, ok := authctx.DirectHumanPrincipalBindingFromContext(ctx, ownerUserID)
	if !ok {
		return inboxstore.Principal{}
	}
	return inboxstore.Principal{UserID: binding.UserID, AuthMethod: binding.AuthMethod, SessionID: binding.SessionID}
}

func (s *Service) wakeMainInbox(target mainInboxTarget) {
	s.StartSessionBackgroundTask(target.sessionKey, target.owner(), func(ctx context.Context) {
		s.drainMainInbox(ctx, target)
	})
}

// drainMainInbox 在会话空闲时把积压的 pending 合并成一轮启动；运行中交给安全点与轮末接续。
func (s *Service) drainMainInbox(ctx context.Context, target mainInboxTarget) bool {
	unlock, lockErr := s.dispatchLocks.lock(ctx, target.sessionKey)
	if lockErr != nil {
		return false
	}
	defer unlock()
	return s.dispatchMainInboxLocked(ctx, target)
}

// dispatchMainInboxLocked 要求持有该会话的派发锁：空闲时按类型规格起轮，运行中则投给当前 turn。
// 事件已收口却不必起轮时（目标陈旧、回执已处理）继续取下一批，直到起轮或无事可做。
func (s *Service) dispatchMainInboxLocked(ctx context.Context, target mainInboxTarget) bool {
	for {
		if running := s.Runtime.GetRunningRoundIDs(target.sessionKey); len(running) > 0 {
			s.deliverMainInboxRunningInput(ctx, target, running[0])
			return false
		}
		gate, err := s.inbox.Gate(ctx, target.sessionKey)
		if err != nil {
			s.LoggerFor(ctx).Error("读取主窗口暂停闸门失败", "session_key", target.sessionKey, "err", err)
			return false
		}
		batch, err := s.claimMainInbox(ctx, target, "", func(pending []inboxstore.Event) []inboxstore.Event {
			return roundBatch(dispatchableEvents(pending, gate))
		})
		if err != nil || len(batch) == 0 {
			return false
		}
		switch inboxKindOf(batch[0]).start(s, ctx, target, batch[0].RoundID, batch) {
		case inboxStartRound:
			s.broadcastMainInbox(ctx, target)
			return true
		case inboxStartNone:
			return false
		}
	}
}

// startUserInboxRound 把合并后的用户消息起成一轮；真人身份随事件持久化，取走时还原配置能力。
func (s *Service) startUserInboxRound(ctx context.Context, target mainInboxTarget, roundID string, batch []inboxstore.Event) inboxStart {
	roundCtx := ctx
	if principal := batch[0].Payload.Principal; principal.UserID != "" {
		roundCtx = authctx.WithQueuedHumanPrincipalBinding(ctx, authctx.QueuedHumanPrincipalBinding(principal))
	}
	return s.startMainInboxBatch(roundCtx, target, batch, s.mainInboxRequest(target, roundID, batch))
}

// startMainInboxBatch 起一轮并把起轮批次标为 consumed；起轮失败时整批退回 pending 并提示用户。
// 标记 consumed 失败也无妨：轮末会再补记一次，不会让已执行的批次重跑。
func (s *Service) startMainInboxBatch(ctx context.Context, target mainInboxTarget, batch []inboxstore.Event, request Request) inboxStart {
	request.inboxEventIDs = eventIDs(batch)
	if err := s.startMainInboxRound(ctx, request); err != nil {
		s.releaseMainInbox(ctx, target, request.RoundID)
		s.reportMainInboxFailure(ctx, target, err)
		return inboxStartNone
	}
	if _, err := s.inbox.Transition(ctx, target.sessionKey, request.inboxEventIDs, inboxstore.StatusDelivering, inboxstore.StatusConsumed, request.RoundID); err != nil {
		s.LoggerFor(ctx).Error("主窗口 inbox 已起轮但标记 consumed 失败", "round_id", request.RoundID, "err", err)
	}
	return inboxStartRound
}

// startMainInboxRound 是主窗口起轮的唯一出口。延迟提交的轮次（宿主 run、Echo）在这里登记为不接受运行中输入，
// 起轮失败即撤销登记，轮末由 finishMainInboxRound 移除。
func (s *Service) startMainInboxRound(ctx context.Context, request Request) error {
	if request.DeferredAssistant != nil {
		s.hostRuns.Store(request.RoundID, struct{}{})
	}
	err := s.handleChat(ctx, request, chatExecutionInline)
	if err != nil {
		s.hostRuns.Delete(request.RoundID)
	}
	return err
}

// dispatchableEvents 去掉被暂停闸门挡住的事件；闸门之后入队、未获准越过的宿主恢复要等用户再开口。
func dispatchableEvents(pending []inboxstore.Event, gate inboxstore.Gate) []inboxstore.Event {
	if !gate.Paused {
		return pending
	}
	kept := make([]inboxstore.Event, 0, len(pending))
	for _, event := range pending {
		if !inboxKindOf(event).gated || admittedPastGate(event, gate) {
			kept = append(kept, event)
		}
	}
	return kept
}

func admittedPastGate(event inboxstore.Event, gate inboxstore.Gate) bool {
	recovery := event.Payload.Recovery
	return recovery != nil && recovery.AdmittedRevision != nil && *recovery.AdmittedRevision == gate.Revision
}

// roundBatch 取 pending 的最长可合并前缀：斜杠命令与宿主事件独占一轮，不同真人身份不合并。
func roundBatch(pending []inboxstore.Event) []inboxstore.Event {
	if len(pending) == 0 {
		return nil
	}
	first := pending[0]
	if isSoloInboxEvent(first) {
		return pending[:1]
	}
	end := 1
	for end < len(pending) && !isSoloInboxEvent(pending[end]) && pending[end].Payload.Principal == first.Payload.Principal {
		end++
	}
	return pending[:end]
}

// guidanceBatch 取第一个斜杠命令之前的全部 pending；命令要等本轮结束后单独执行，不能被插队越过。
// 不能插进别的轮次的事件（Echo、宿主恢复、IM 回执）留在 inbox，等对话空闲后独占一轮。
func guidanceBatch(pending []inboxstore.Event) []inboxstore.Event {
	batch := make([]inboxstore.Event, 0, len(pending))
	for _, event := range pending {
		if isInboxCommand(event) {
			break
		}
		if joinsRunningRound(event) {
			batch = append(batch, event)
		}
	}
	return batch
}

// claimMainInbox 把选中的 pending 条件迁移为 delivering；与撤回竞争时只保留真正迁移成功的事件。
// roundID 为空时按首个事件的类型规格分配，认领结果的 RoundID 即这一轮的 ID。
func (s *Service) claimMainInbox(
	ctx context.Context,
	target mainInboxTarget,
	roundID string,
	pick func([]inboxstore.Event) []inboxstore.Event,
) ([]inboxstore.Event, error) {
	open, err := s.inbox.ListOpen(ctx, target.sessionKey, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	picked := pick(pendingEvents(open))
	if len(picked) == 0 {
		return nil, nil
	}
	if roundID == "" {
		roundID = inboxRoundID(picked[0])
	}
	moved, err := s.inbox.Transition(ctx, target.sessionKey, eventIDs(picked), inboxstore.StatusPending, inboxstore.StatusDelivering, roundID)
	if err != nil {
		return nil, err
	}
	claimed := selectEvents(picked, moved)
	for index := range claimed {
		claimed[index].Status, claimed[index].RoundID = inboxstore.StatusDelivering, roundID
	}
	return claimed, nil
}

// mainInboxRequest 把合并后的用户消息还原成一轮可见输入；inbox 来源不经过运行中路由与 JSONL 队列。
func (s *Service) mainInboxRequest(target mainInboxTarget, roundID string, batch []inboxstore.Event) Request {
	contents := make([]string, 0, len(batch))
	var attachments []protocol.ChatAttachment
	for _, event := range batch {
		contents = append(contents, event.Payload.Content)
		attachments = append(attachments, event.Payload.Attachments...)
	}
	return Request{
		SessionKey:                        target.sessionKey,
		AgentID:                           target.agent.AgentID,
		Content:                           strings.Join(contents, "\n\n"),
		Attachments:                       attachments,
		ClientMessageID:                   batch[0].Payload.ClientMessageID,
		RoundID:                           roundID,
		BroadcastUserMessage:              true,
		ExecutionOrigin:                   mainInboxExecutionOrigin,
		trustedQueuedConfigurationContext: batch[0].Payload.Principal.UserID != "",
	}
}

// deliverMainInboxRunningInput 把 pending 逐条作为运行中输入交给当前 turn（running_input_v1）。
// input_consumed 回执才算 consumed；没有回执的（turn 已收尾、被中断）随轮末退回 pending。
// 未协商的 runtime 什么都不做，由 PostToolUse hook 在安全点取走。
func (s *Service) deliverMainInboxRunningInput(ctx context.Context, target mainInboxTarget, roundID string) {
	sender := runtimectx.RunningInputSenderOf(s.Runtime.SessionClient(target.sessionKey))
	if sender == nil || !s.acceptsMidRunInput(roundID) {
		return
	}
	events, err := s.claimMainInbox(ctx, target, roundID, guidanceBatch)
	for index := 0; err == nil && index < len(events); index++ {
		if err = s.sendMainInboxRunningInput(ctx, sender, events[index]); err != nil {
			// 未送出的立即退回，已送出的仍等回执或轮末退回。
			_, _ = s.inbox.Transition(ctx, target.sessionKey, eventIDs(events[index:]), inboxstore.StatusDelivering, inboxstore.StatusPending, "")
		}
	}
	if err != nil {
		s.LoggerFor(ctx).Warn("主窗口 inbox 运行中投递失败，消息保留为待处理", "round_id", roundID, "err", err)
	}
	s.broadcastMainInbox(ctx, target)
}

func (s *Service) sendMainInboxRunningInput(ctx context.Context, sender runtimectx.RunningInputSender, event inboxstore.Event) error {
	content, err := s.mainInboxEventContent(ctx, event)
	if err != nil {
		return err
	}
	return sender.SendRunningInput(ctx, event.EventID, content.Payload())
}

// mainInboxEventContent 是插进运行中轮次的内容：用户原话带附件，宿主事件用带来源包装的提示词。
func (s *Service) mainInboxEventContent(ctx context.Context, event inboxstore.Event) (conversationsvc.RuntimeContent, error) {
	if isHostInboxEvent(event) {
		return s.renderRuntimeContentWithAttachments(ctx, hostEventPrompt(event), nil)
	}
	return s.renderRuntimeContentWithAttachments(ctx, event.Payload.Content, event.Payload.Attachments)
}

// acceptsMidRunInput 区分可插话的轮次与宿主 run（定时任务、跟进、Echo）：后者期间的输入等它收口后另起一轮。
func (s *Service) acceptsMidRunInput(roundID string) bool {
	_, hostRun := s.hostRuns.Load(roundID)
	return !hostRun
}

// confirmMainInboxInputs 处理 input_consumed：input_id 即 inbox event ID，按回执顺序确认并落盘为用户消息。
func (s *Service) confirmMainInboxInputs(target mainInboxTarget, roundID string, inputIDs []string) {
	ctx := runtimehost.ContextWithExactOwner(context.Background(), target.owner())
	open, err := s.inbox.ListOpen(ctx, target.sessionKey, time.Now().UTC())
	if err != nil {
		s.LoggerFor(ctx).Error("读取主窗口 inbox 回执事件失败", "round_id", roundID, "err", err)
		return
	}
	s.confirmMainInboxGuidance(target, roundID, selectEvents(open, inputIDs))
}

// mainInboxGuidanceHook 在 PostToolUse 安全点取走 pending，作为用户补充注入当前 round；
// runtime 确认 applied 后才算 consumed，未确认的在轮末退回 pending 并接续下一轮。
// 协商了运行中输入的 runtime 不走这条路径。
func (s *Service) mainInboxGuidanceHook(target mainInboxTarget) sdkhook.Callback {
	return func(ctx context.Context, input sdkhook.Input, _ string) (sdkhook.Output, error) {
		if !runtimehost.IsGuidanceHookEvent(input) ||
			runtimectx.RunningInputSenderOf(s.Runtime.SessionClient(target.sessionKey)) != nil {
			return sdkhook.Output{}, nil
		}
		ctx = runtimehost.ContextWithExactOwner(ctx, target.owner())
		running := s.Runtime.GetRunningRoundIDs(target.sessionKey)
		if len(running) == 0 {
			return sdkhook.Output{}, nil
		}
		roundID := running[0]
		if !s.acceptsMidRunInput(roundID) {
			return sdkhook.Output{}, nil
		}
		events, err := s.claimMainInbox(ctx, target, roundID, guidanceBatch)
		if err != nil || len(events) == 0 {
			return sdkhook.Output{}, err
		}
		inputs, err := s.mainInboxGuidedInputs(ctx, events)
		if err != nil {
			_, _ = s.inbox.Transition(ctx, target.sessionKey, eventIDs(events), inboxstore.StatusDelivering, inboxstore.StatusPending, "")
			return sdkhook.Output{}, err
		}
		confirm := func() { s.confirmMainInboxGuidance(target, roundID, events) }
		output := runtimehost.GuidanceHookOutput(inputs, confirm)
		if !s.Runtime.SupportsHookResponseAck(target.sessionKey) {
			// 旧 runtime 没有 applied ACK，只能以 hook 返回视为已送达。
			confirm()
			output.OnApplied = nil
		}
		return output, nil
	}
}

func (s *Service) mainInboxGuidedInputs(ctx context.Context, events []inboxstore.Event) ([]runtimectx.GuidedInput, error) {
	inputs := make([]runtimectx.GuidedInput, 0, len(events))
	for _, event := range events {
		content, err := s.mainInboxEventContent(ctx, event)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, runtimectx.GuidedInput{Content: content.PlainText()})
	}
	return inputs, nil
}

func (s *Service) confirmMainInboxGuidance(target mainInboxTarget, roundID string, events []inboxstore.Event) {
	ctx := runtimehost.ContextWithExactOwner(context.Background(), target.owner())
	moved, err := s.inbox.Transition(ctx, target.sessionKey, eventIDs(events), inboxstore.StatusDelivering, inboxstore.StatusConsumed, roundID)
	if err == nil {
		err = s.persistMainInboxGuidance(ctx, target, roundID, selectEvents(events, moved))
	}
	if err != nil {
		s.LoggerFor(ctx).Error("主窗口 inbox 引导确认失败", "round_id", roundID, "err", err)
	}
	s.broadcastMainInbox(ctx, target)
}

func (s *Service) persistMainInboxGuidance(ctx context.Context, target mainInboxTarget, roundID string, events []inboxstore.Event) error {
	if len(events) == 0 {
		return nil
	}
	location := target.location()
	sessionItem, err := s.currentGuidanceSession(location, target.sessionKey)
	if err != nil {
		return err
	}
	for _, event := range events {
		message := guidanceUserMessage{
			userMessageID:   "msg_user_" + event.EventID,
			sourceRoundID:   event.EventID,
			targetRoundID:   roundID,
			content:         event.Payload.Content,
			attachments:     event.Payload.Attachments,
			clientMessageID: event.Payload.ClientMessageID,
		}
		if isHostInboxEvent(event) {
			message.content = hostEventPrompt(event)
			message.metadata = hostEventMetadata(event)
		}
		sessionItem, err = s.persistGuidanceUserMessage(ctx, location, sessionItem, message)
		if err != nil {
			return err
		}
	}
	return nil
}

// finishMainInboxRound 在 round 终态后退回未确认的注入；用户主动停止时不自动接续。
// 起轮批次先补记 consumed：起轮时那次标记若失败，退回会让已执行的输入再跑一遍。
// 与运行中投递共用派发锁：投递要么先于退回完成，要么已看到本轮不再运行，不会遗留 delivering。
func (s *Service) finishMainInboxRound(ctx context.Context, target mainInboxTarget, roundID string, startEventIDs []string, resume bool) bool {
	unlock, lockErr := s.dispatchLocks.lock(ctx, target.sessionKey)
	if lockErr != nil {
		return false
	}
	defer unlock()
	s.hostRuns.Delete(roundID)
	if _, err := s.inbox.Transition(ctx, target.sessionKey, startEventIDs, inboxstore.StatusDelivering, inboxstore.StatusConsumed, roundID); err != nil {
		s.LoggerFor(ctx).Error("轮末补记主窗口起轮批次 consumed 失败", "round_id", roundID, "err", err)
	}
	s.releaseMainInbox(ctx, target, roundID)
	return resume && s.dispatchMainInboxLocked(ctx, target)
}

func (s *Service) releaseMainInbox(ctx context.Context, target mainInboxTarget, roundID string) {
	if err := s.inbox.ReleaseDelivering(ctx, target.sessionKey, roundID); err != nil {
		s.LoggerFor(ctx).Error("退回主窗口 inbox 未确认事件失败", "round_id", roundID, "err", err)
	}
	s.broadcastMainInbox(ctx, target)
}

func (s *Service) reportMainInboxFailure(ctx context.Context, target mainInboxTarget, err error) {
	// 撞上 runtime 自唤醒时由那一轮的终态接续，不是用户可见失败。
	var pending *runtimeWakePendingError
	if errors.As(err, &pending) {
		return
	}
	s.LoggerFor(ctx).Error("主窗口 inbox 起轮失败，消息保留为待处理", "session_key", target.sessionKey, "err", err)
	message := "消息处理失败，已保留，可撤回或稍后继续发送"
	if clientMessage, ok := protocol.ClientErrorMessage(err); ok {
		message = clientMessage
	}
	s.broadcastEventWithTimeout(ctx, target.sessionKey, protocol.NewErrorEvent(target.sessionKey, message))
}

func (s *Service) withdrawMainInbox(ctx context.Context, target mainInboxTarget, eventID string) error {
	moved, err := s.inbox.Transition(ctx, target.sessionKey, []string{strings.TrimSpace(eventID)}, inboxstore.StatusPending, inboxstore.StatusWithdrawn, "")
	if err != nil {
		return err
	}
	if len(moved) == 0 {
		return ErrInboxEventNotPending
	}
	s.broadcastMainInbox(ctx, target)
	return nil
}

// broadcastMainInbox 复用 input_queue 快照事件投影 pending 消息，前端以只读+撤回方式展示。
func (s *Service) broadcastMainInbox(ctx context.Context, target mainInboxTarget) {
	open, err := s.inbox.ListOpen(ctx, target.sessionKey, time.Now().UTC())
	if err != nil {
		s.LoggerFor(ctx).Warn("读取主窗口 inbox 快照失败", "session_key", target.sessionKey, "err", err)
		return
	}
	pending := pendingEvents(open)
	items := make([]protocol.InputQueueItem, 0, len(pending))
	for _, event := range pending {
		// 宿主事件不是用户写的，不进入可撤回的待发送列表。
		if isHostInboxEvent(event) {
			continue
		}
		items = append(items, protocol.InputQueueItem{
			ID:              event.EventID,
			Scope:           protocol.InputQueueScopeDM,
			SessionKey:      target.sessionKey,
			AgentID:         target.agent.AgentID,
			ClientMessageID: event.Payload.ClientMessageID,
			Source:          protocol.InputQueueSourceUser,
			Content:         event.Payload.Content,
			Attachments:     event.Payload.Attachments,
			DeliveryPolicy:  protocol.ChatDeliveryPolicyGuide,
			OwnerUserID:     event.OwnerUserID,
			CreatedAt:       event.CreatedAt.UnixMilli(),
			UpdatedAt:       event.CreatedAt.UnixMilli(),
		})
	}
	s.broadcastInputQueueSnapshot(ctx, target.sessionKey, items)
}

func pendingEvents(events []inboxstore.Event) []inboxstore.Event {
	pending := make([]inboxstore.Event, 0, len(events))
	for _, event := range events {
		if event.Status == inboxstore.StatusPending {
			pending = append(pending, event)
		}
	}
	return pending
}

func eventIDs(events []inboxstore.Event) []string {
	ids := make([]string, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.EventID)
	}
	return ids
}

func selectEvents(events []inboxstore.Event, ids []string) []inboxstore.Event {
	keep := make(map[string]bool, len(ids))
	for _, id := range ids {
		keep[id] = true
	}
	selected := make([]inboxstore.Event, 0, len(ids))
	for _, event := range events {
		if keep[event.EventID] {
			selected = append(selected, event)
		}
	}
	return selected
}
