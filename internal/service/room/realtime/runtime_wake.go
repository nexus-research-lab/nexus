// INPUT: Room slot idle drain 收到的 SDK 消息、上一轮 round/slot 身份与当时的 runtime client。
// OUTPUT: runtime 自唤醒（后台任务完成后自行开启新 turn）时，以只含该成员的内部 Room round 接管同一消息流；
// 公区 slot 的唤醒在共享时间线显示触发行，私域 slot 保持私域；交接期间同会话普通输入等待后再路由。
// POS: Room idle drain 与自唤醒 round 的交接边界；自唤醒 slot 不发送 query、不推进公区上下文游标。
package realtime

import (
	"context"
	"errors"
	"slices"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	messagepkg "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/exec"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
)

// errRoomRuntimeWakeLost 表示交接期间成员 runtime 进程已被替换，原 turn 无从接续。
var errRoomRuntimeWakeLost = errors.New("runtime 进程已切换，自唤醒 turn 无法接续")

// roomRuntimeWakeStart 只由 idle drain 交接时创建，外部 ChatRequest 构造者无法伪造。
type roomRuntimeWakeStart struct {
	trigger messagepkg.RuntimeWakeTrigger
	// runtimeSessionKey/client 是交接时正在 drain 的成员进程；新 slot 拿到的 client 不同说明 turn 已无法接续。
	runtimeSessionKey string
	client            runtimectx.Client
	initial           []sdkprotocol.ReceivedMessage
	// replyRoute 继承上一轮 slot 的公私域；不继承 reply source，避免重复登记定向回复。
	replyRoute protocol.RoomReplyRoute
}

func (w *roomRuntimeWakeStart) publishesPublicOutput() bool {
	return w != nil && (w.replyRoute.Mode == "" || w.replyRoute.Mode == protocol.RoomReplyRoutePublic)
}

// roomIdleWakeObserver 记住 drain 中最近一次顶层后台任务通知；drain 单协程串行调用，无需加锁。
type roomIdleWakeObserver struct {
	trigger messagepkg.RuntimeWakeTrigger
}

func (o *roomIdleWakeObserver) observe(incoming sdkprotocol.ReceivedMessage) {
	if trigger, ok := messagepkg.RuntimeWakeTriggerFromTaskNotification(incoming); ok {
		o.trigger = trigger
	}
}

func (o *roomIdleWakeObserver) take() messagepkg.RuntimeWakeTrigger {
	trigger := o.trigger
	o.trigger = messagepkg.RuntimeWakeTrigger{}
	return trigger
}

// handoffRoomRuntimeWake 让 slot drain 退出，并以新的内部 round 接管 runtime 已开始的 turn。
// 闸门在 drain 返回前同步登记，之后到达派发锁的普通输入都会先等待交接完成。
func (s *Service) handoffRoomRuntimeWake(
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	trigger messagepkg.RuntimeWakeTrigger,
	incoming sdkprotocol.ReceivedMessage,
) {
	client := s.Runtime.SessionClient(slot.RuntimeSessionKey)
	wake := &roomRuntimeWakeStart{
		trigger:           trigger,
		runtimeSessionKey: slot.RuntimeSessionKey,
		client:            client,
		initial:           []sdkprotocol.ReceivedMessage{incoming},
		// 只继承可见性：不继承唤醒策略与下一跳，自唤醒输出不再连锁唤醒其他成员。
		replyRoute: protocol.RoomReplyRoute{
			Mode:       roomSlotReplyRoute(slot).Mode,
			Recipients: slices.Clone(roomSlotReplyRoute(slot).Recipients),
		},
	}
	inputOptions := sdkprotocol.OutboundMessageOptions{
		Purpose:   messagepkg.RuntimeWakePurpose,
		Synthetic: true,
		Metadata:  trigger.Metadata(),
	}
	if !wake.publishesPublicOutput() {
		inputOptions.HiddenFromUser = true
	}
	request := ChatRequest{
		SessionKey:           roundValue.SessionKey,
		RoomID:               roundValue.RoomID,
		ConversationID:       roundValue.ConversationID,
		CoordinatorAgentID:   roundValue.CoordinatorAgentID,
		Content:              trigger.Content(),
		TargetAgentIDs:       []string{slot.AgentID},
		Internal:             true,
		BroadcastUserMessage: wake.publishesPublicOutput(),
		ExecutionOrigin:      messagepkg.RuntimeWakePurpose,
		InputOptions:         inputOptions,
		runtimeWake:          wake,
	}
	release := s.RuntimeWakeGates.Begin(roundValue.ConversationID)
	s.StartSessionBackgroundTask(roundValue.SessionKey, roundValue.OwnerUserID, func(taskCtx context.Context) {
		defer release()
		ctx := runtimehost.ContextWithExactOwner(taskCtx, roundValue.OwnerUserID)
		logger := s.LoggerFor(ctx).With(
			"session_key", roundValue.SessionKey,
			"previous_round_id", roundValue.RoundID,
			"agent_id", slot.AgentID,
		)
		if client == nil {
			logger.Info("Room runtime 自唤醒交接时进程已关闭，放弃接续")
			return
		}
		err := s.handleChat(ctx, request)
		if err == nil {
			return
		}
		if errors.Is(err, errRoomRuntimeWakeLost) {
			logger.Info("Room runtime 自唤醒交接时进程已切换，放弃接续")
			return
		}
		logger.Warn("Room runtime 自唤醒 round 启动失败，回退 idle drain", "err", err)
		// 回退后剩余输出仍落到上一轮，至少不丢消息。
		s.startIdleSubagentNotificationDrains(taskCtx, roundValue)
	})
}

// lost 报告成员 runtime 进程是否已不是交接时那一个。
func (w *roomRuntimeWakeStart) lost(current runtimectx.Client) bool {
	return w != nil && current != w.client
}

// decorateRoomRuntimeWakeUserMessage 把内部 round 的用户行改成自唤醒触发行；私域唤醒仍保持隐藏。
func decorateRoomRuntimeWakeUserMessage(request ChatRequest, message protocol.Message) protocol.Message {
	if request.runtimeWake == nil {
		return message
	}
	message = messagepkg.DecorateRuntimeWakeUserMessage(message, request.runtimeWake.trigger)
	if !request.runtimeWake.publishesPublicOutput() {
		message["hidden_from_user"] = true
	}
	return message
}

// applyRuntimeWakeSlot 让自唤醒 slot 继承上一轮公私域，并标记本 slot 只接管、不发送 query。
func applyRuntimeWakeSlot(request ChatRequest, slot *activeRoomSlot) {
	if request.runtimeWake == nil || slot == nil {
		return
	}
	slot.runtimeWake = request.runtimeWake
	slot.HiddenFromUser = !request.runtimeWake.publishesPublicOutput()
	slot.setDeliveryMetadata(request.runtimeWake.replyRoute, "", "")
}

// executeRuntimeWakeRound 接管 runtime 已开始的 turn：不读公区上下文、不发 query，只登记私有 round 边界。
func (e *slotExecution) executeRuntimeWakeRound(client runtimectx.Client, round runtimehost.AgentRound) (exec.RoundExecutionResult, error) {
	wake := e.slot.runtimeWake
	if wake.lost(client) {
		return exec.RoundExecutionResult{}, errRoomRuntimeWakeLost
	}
	if err := e.service.recordPrivateRoundMarker(e.round, e.slot, wake.trigger.Content()); err != nil {
		return exec.RoundExecutionResult{}, err
	}
	round.ResumeRuntimeTurn = true
	round.InitialMessages = wake.initial
	return e.service.ExecuteAgentRound(e.ctx, round)
}

// yieldToRoomRuntimeWake 返回同会话正在交接的自唤醒完成信号；自唤醒请求自身不等待。
func (s *Service) yieldToRoomRuntimeWake(request ChatRequest, conversationID string) <-chan struct{} {
	if request.runtimeWake != nil {
		return nil
	}
	return s.RuntimeWakeGates.Pending(conversationID)
}
