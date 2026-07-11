package runtime

import (
	"context"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

// StopTask 停止指定 session 中的后台任务。
func (m *Manager) StopTask(ctx context.Context, sessionKey string, taskID string) error {
	sessionKey = strings.TrimSpace(sessionKey)
	taskID = strings.TrimSpace(taskID)
	if sessionKey == "" || taskID == "" {
		return agentclient.ErrNotConnected
	}

	m.mu.Lock()
	state := m.sessions[sessionKey]
	var client Client
	if state != nil {
		client = state.Client
		if client != nil {
			m.touchStateLocked(state)
		}
	}
	m.mu.Unlock()
	if client == nil {
		return agentclient.ErrNotConnected
	}
	return client.StopTask(ctx, taskID)
}

// SendTaskMessage 向指定 session 的后台任务投递一条后续消息。
func (m *Manager) SendTaskMessage(ctx context.Context, sessionKey string, taskID string, message string, summary string) error {
	sessionKey = strings.TrimSpace(sessionKey)
	taskID = strings.TrimSpace(taskID)
	message = strings.TrimSpace(message)
	if sessionKey == "" || taskID == "" || message == "" {
		return agentclient.ErrNotConnected
	}

	m.mu.Lock()
	state := m.sessions[sessionKey]
	var client Client
	if state != nil {
		client = state.Client
		if client != nil {
			m.touchStateLocked(state)
		}
	}
	m.mu.Unlock()
	if client == nil {
		return agentclient.ErrNotConnected
	}
	return client.SendTaskMessage(ctx, taskID, message, strings.TrimSpace(summary))
}
