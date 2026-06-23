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

	m.mu.RLock()
	state := m.sessions[sessionKey]
	var client Client
	if state != nil {
		client = state.Client
	}
	m.mu.RUnlock()
	if client == nil {
		return agentclient.ErrNotConnected
	}
	return client.StopTask(ctx, taskID)
}
