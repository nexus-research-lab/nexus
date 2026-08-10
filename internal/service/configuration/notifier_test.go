// INPUT: configuration 目录失效通知。
// OUTPUT: 并发安全的 Agent 通知记录与断言查询。
// POS: configuration 跨域集成测试共享通知替身。
package configuration_test

import (
	"context"
	"slices"
	"sync"
)

type recordingConfigurationNotifier struct {
	mu       sync.Mutex
	agentIDs []string
}

func (n *recordingConfigurationNotifier) AgentChanged(_ context.Context, agentID, _ string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.agentIDs = append(n.agentIDs, agentID)
}

func (*recordingConfigurationNotifier) RoomChanged(context.Context, string, string, string) {}

func (*recordingConfigurationNotifier) RoomMemberChanged(context.Context, string, string, bool) {}

func (n *recordingConfigurationNotifier) hasAgent(agentID string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return slices.Contains(n.agentIDs, agentID)
}
