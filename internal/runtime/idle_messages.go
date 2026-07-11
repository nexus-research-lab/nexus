package runtime

import (
	"context"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// IdleMessageHandler 处理 round 外到达的 SDK 消息。返回 false 表示停止 drain。
type IdleMessageHandler func(context.Context, sdkprotocol.ReceivedMessage) bool

// StartIdleMessageDrain 在没有活动 round 时接管 client 消息，用于后台 task 通知。
func (m *Manager) StartIdleMessageDrain(sessionKey string, handler IdleMessageHandler) {
	if m == nil || sessionKey == "" || handler == nil {
		return
	}
	m.mu.Lock()
	state, ok := m.sessions[sessionKey]
	if !ok || state.Client == nil || len(state.RunningRounds) > 0 {
		m.mu.Unlock()
		return
	}
	if state.IdleMessageCancel != nil {
		state.IdleMessageCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	state.IdleMessageDrainID++
	drainID := state.IdleMessageDrainID
	state.IdleMessageCancel = cancel
	client := state.Client
	m.touchStateLocked(state)
	m.mu.Unlock()

	go m.runIdleMessageDrain(ctx, sessionKey, drainID, client, handler)
}

func (m *Manager) runIdleMessageDrain(
	ctx context.Context,
	sessionKey string,
	drainID int64,
	client Client,
	handler IdleMessageHandler,
) {
	defer func() {
		m.mu.Lock()
		if state := m.sessions[sessionKey]; state != nil && state.IdleMessageDrainID == drainID {
			state.IdleMessageCancel = nil
		}
		m.mu.Unlock()
	}()
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
			if state := m.sessions[sessionKey]; state != nil && state.IdleMessageDrainID == drainID {
				m.touchStateLocked(state)
			}
			m.mu.Unlock()
			if !handler(ctx, message) {
				return
			}
		}
	}
}
