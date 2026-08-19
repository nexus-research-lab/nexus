// INPUT: 无活动 round 的 client 消息流与 idle handler。
// OUTPUT: 串行交接、可等待退出的独占消息消费租约。
// POS: idle 消费者与 round 消费者之间的消息流所有权边界。
package runtime

import (
	"context"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// IdleMessageHandler 处理 round 外到达的 SDK 消息。返回 false 表示停止 drain。
type IdleMessageHandler func(context.Context, sdkprotocol.ReceivedMessage) bool

// idleMessageDrain 表示当前 client 消息流的 idle 消费租约。
// done 只在 handler 已经返回且租约完全退出后关闭。
type idleMessageDrain struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// StartIdleMessageDrain 在没有活动 round 时接管 client 消息，用于后台 task 通知。
func (m *Manager) StartIdleMessageDrain(sessionKey string, handler IdleMessageHandler) {
	if m == nil || sessionKey == "" || handler == nil {
		return
	}
	m.mu.Lock()
	state, ok := m.sessions[sessionKey]
	if !ok || state.Closing || state.Client == nil || state.Rounds.active() {
		m.mu.Unlock()
		return
	}
	previousDrain := state.IdleMessageDrain
	if previousDrain != nil {
		previousDrain.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	drain := &idleMessageDrain{
		cancel: cancel,
		done:   make(chan struct{}),
	}
	state.IdleMessageDrain = drain
	client := state.Client
	m.touchStateLocked(state)
	m.mu.Unlock()

	go m.runIdleMessageDrain(ctx, sessionKey, state, drain, previousDrain, client, handler)
}

func (m *Manager) runIdleMessageDrain(
	ctx context.Context,
	sessionKey string,
	expectedState *sessionState,
	drain *idleMessageDrain,
	previousDrain *idleMessageDrain,
	client Client,
	handler IdleMessageHandler,
) {
	defer func() {
		m.mu.Lock()
		if state := m.sessions[sessionKey]; state == expectedState && state.IdleMessageDrain == drain {
			state.IdleMessageDrain = nil
			m.removeClientlessSessionIfIdleLocked(sessionKey, expectedState, nil)
		}
		m.mu.Unlock()
		close(drain.done)
	}()
	if previousDrain != nil {
		// 新租约必须等旧 handler 完全退出。即使自身已经取消，也不能提前
		// 关闭 done，否则 round 会在旧消费者仍存活时接管同一消息流。
		<-previousDrain.done
	}
	if ctx.Err() != nil {
		return
	}
	messageCh := client.ReceiveMessages(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		select {
		case <-ctx.Done():
			return
		case message, ok := <-messageCh:
			if !ok {
				return
			}
			m.mu.Lock()
			if state := m.sessions[sessionKey]; state == expectedState && state.IdleMessageDrain == drain {
				m.touchStateLocked(state)
			}
			m.mu.Unlock()
			if !handler(ctx, message) {
				return
			}
		}
	}
}

func waitIdleMessageDrain(ctx context.Context, drain *idleMessageDrain) error {
	if drain == nil {
		return nil
	}
	select {
	case <-drain.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
