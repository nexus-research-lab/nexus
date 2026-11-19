// INPUT: DM idle drain 收到的 SDK 消息、上一轮 roundRunner 的身份与 runtime client。
// OUTPUT: runtime 自唤醒（后台任务完成后自行开启新 turn）时，以独立 DM round 接管同一消息流；交接期间同会话普通输入等待后排队；
// 进程已切换时放弃接续，其它启动失败回退旧 idle drain。
// POS: DM idle drain 与 runtime 自唤醒 round 的交接边界；自唤醒 round 不发送 query，只承接 runtime 已开始的 turn。
package dm

import (
	"context"
	"errors"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	messagepkg "github.com/nexus-research-lab/nexus/internal/message"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
)

// runtimeWakeStart 只由 idle drain 交接时创建，外部 Request 构造者无法伪造。
type runtimeWakeStart struct {
	trigger messagepkg.RuntimeWakeTrigger
	// client 是交接时正在 drain 的进程；新 round 拿到的 client 不同时说明 turn 已无法接续。
	client  runtimectx.Client
	initial []sdkprotocol.ReceivedMessage
}

// observeIdleTaskNotification 记住最近一次后台任务通知，作为随后自唤醒 round 的触发说明。
func (r *roundRunner) observeIdleTaskNotification(incoming sdkprotocol.ReceivedMessage) {
	trigger, ok := messagepkg.RuntimeWakeTriggerFromTaskNotification(incoming)
	if !ok {
		return
	}
	r.Mu.Lock()
	r.idleWakeTrigger = trigger
	r.Mu.Unlock()
}

// handoffRuntimeWake 让 idle drain 退出，并以新 round 接管 runtime 已开始的 turn。
// 新 round 拥有完整生命周期：hook/权限路由、result 终态、round_status 与后续派发。
func (r *roundRunner) handoffRuntimeWake(incoming sdkprotocol.ReceivedMessage) {
	r.Mu.Lock()
	trigger := r.idleWakeTrigger
	r.idleWakeTrigger = messagepkg.RuntimeWakeTrigger{}
	r.Mu.Unlock()
	request := Request{
		SessionKey:      r.sessionKey,
		AgentID:         r.agent.AgentID,
		Content:         trigger.Content(),
		Internal:        true,
		ExecutionOrigin: messagepkg.RuntimeWakePurpose,
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose:   messagepkg.RuntimeWakePurpose,
			Synthetic: true,
			Metadata:  trigger.Metadata(),
		},
		runtimeWake: &runtimeWakeStart{
			trigger: trigger,
			client:  r.client,
			initial: []sdkprotocol.ReceivedMessage{incoming},
		},
	}
	// 闸门必须在 drain 退出前同步登记：之后 StartRound 才能放行的普通输入都会看到它。
	release := r.service.RuntimeWakeGates.Begin(r.sessionKey)
	r.service.StartSessionBackgroundTask(r.sessionKey, r.ownerUserID, func(ctx context.Context) {
		defer release()
		err := r.service.handleChatLocked(ctx, request, chatExecutionInline)
		if err == nil {
			return
		}
		if errors.Is(err, errRuntimeWakeLost) {
			// 原进程已被替换，自行开启的 turn 随旧进程结束；新进程没有可接续的输出，不再回退 drain。
			r.service.LoggerFor(ctx).Info("runtime 自唤醒交接时进程已切换，放弃接续",
				"session_key", r.sessionKey,
				"previous_round_id", r.roundID,
			)
			return
		}
		r.service.LoggerFor(ctx).Warn("runtime 自唤醒 round 启动失败，回退 idle drain",
			"session_key", r.sessionKey,
			"previous_round_id", r.roundID,
			"err", err,
		)
		// 回退后剩余输出仍落到上一轮，至少不丢消息。
		r.service.Runtime.StartIdleMessageDrain(r.sessionKey, r.handleIdleSubagentMessage)
	})
}

// errRuntimeWakeLost 表示交接期间 runtime 进程已被替换，原 turn 无从接续。
var errRuntimeWakeLost = errors.New("runtime 进程已切换，自唤醒 turn 无法接续")

// runtimeWakePendingError 携带交接完成信号，供持锁调用方释放派发锁后等待重试。
type runtimeWakePendingError struct {
	done <-chan struct{}
}

func (e *runtimeWakePendingError) Error() string {
	return runtimehost.ErrRuntimeWakeHandoffPending.Error()
}

func (e *runtimeWakePendingError) Unwrap() error { return runtimehost.ErrRuntimeWakeHandoffPending }

func (e *runtimeWakePendingError) wait(ctx context.Context) error {
	select {
	case <-e.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// yieldToRuntimeWake 让普通 round 给正在交接的自唤醒 turn 让路。
// 准备前与 StartRound 后各检查一次：StartRound 会停掉 drain，此后不会再出现新的交接，检查结果即为最终态。
func (e *dmChatExecution) yieldToRuntimeWake() error {
	if e.request.runtimeWake != nil {
		return nil
	}
	if done := e.service.RuntimeWakeGates.Pending(e.sessionKey); done != nil {
		return &runtimeWakePendingError{done: done}
	}
	return nil
}

// runtimeWakeClientMismatch 报告 round 准备阶段是否换掉了交接时的 runtime 进程。
func (r *roundRunner) runtimeWakeClientMismatch() bool {
	return r.runtimeWake != nil && r.runtimeWake.client != r.client
}

func (r *roundRunner) runtimeWakeInitialMessages() []sdkprotocol.ReceivedMessage {
	if r.runtimeWake == nil {
		return nil
	}
	return r.runtimeWake.initial
}
