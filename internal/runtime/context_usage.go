// INPUT: 可选的 runtime context usage 控制能力。
// OUTPUT: 归一化为 Nexus 协议的当前会话上下文占用快照。
// POS: 产品 runtime 与不同 Agent SDK 实现之间的只读能力适配层。
package runtime

import (
	"context"
	"maps"
	"math"
	"slices"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type contextUsageReader interface {
	ContextUsage(context.Context) (agentclient.ContextUsageResponse, error)
}

// ContextUsageSnapshot 表示一个前端 Session 内某个 Agent 的最后一次权威快照。
type ContextUsageSnapshot struct {
	AgentID string
	Usage   protocol.ContextUsageData
}

// RecordContextUsage 保存 runtime 已确认的最后一次上下文占用快照。
//
// Room 使用共享 Session key 并按 Agent 隔离；DM 只有一个 Agent，仍走同一路径。
// Manager 只负责当前进程的重连热缓存；跨进程恢复由 Session meta.json 负责。
func (m *Manager) RecordContextUsage(
	sessionKey string,
	agentID string,
	usage protocol.ContextUsageData,
) {
	sessionKey = strings.TrimSpace(sessionKey)
	agentID = strings.TrimSpace(agentID)
	if sessionKey == "" || agentID == "" || usage.MaxTokens <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.ensureStateLocked(sessionKey)
	if state.Closing {
		return
	}
	if state.ContextUsageByAgent == nil {
		state.ContextUsageByAgent = make(map[string]protocol.ContextUsageData)
	}
	state.ContextUsageByAgent[agentID] = usage
	m.touchStateLocked(state)
}

// ContextUsageSnapshots 返回 Session 当前可重放的上下文快照。
func (m *Manager) ContextUsageSnapshots(sessionKey string) []ContextUsageSnapshot {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return nil
	}
	m.mu.RLock()
	state := m.sessions[sessionKey]
	if state == nil || state.Closing || len(state.ContextUsageByAgent) == 0 {
		m.mu.RUnlock()
		return nil
	}
	snapshotsByAgent := maps.Clone(state.ContextUsageByAgent)
	m.mu.RUnlock()

	agentIDs := slices.Sorted(maps.Keys(snapshotsByAgent))
	snapshots := make([]ContextUsageSnapshot, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		snapshots = append(snapshots, ContextUsageSnapshot{
			AgentID: agentID,
			Usage:   snapshotsByAgent[agentID],
		})
	}
	return snapshots
}

// ReadContextUsage 在 runtime 支持该控制能力时返回归一化快照。
func ReadContextUsage(
	ctx context.Context,
	client Client,
) (protocol.ContextUsageData, bool, error) {
	reader, supported := client.(contextUsageReader)
	if !supported {
		return protocol.ContextUsageData{}, false, nil
	}
	response, err := reader.ContextUsage(ctx)
	if err != nil {
		return protocol.ContextUsageData{}, true, err
	}
	data, valid := normalizeContextUsage(response)
	return data, valid, nil
}

// ContextUsage 读取底层 SDK session 的当前上下文占用。
func (c *agentClient) ContextUsage(
	ctx context.Context,
) (agentclient.ContextUsageResponse, error) {
	session, err := c.currentSession()
	if err != nil {
		return agentclient.ContextUsageResponse{}, err
	}
	return session.Control().ContextUsage(ctx)
}

// normalizeContextUsage 统一 Claude Code 与 nxs 的窗口字段和比例口径。
func normalizeContextUsage(
	response agentclient.ContextUsageResponse,
) (protocol.ContextUsageData, bool) {
	maxTokens := response.RawMaxTokens
	if maxTokens <= 0 {
		maxTokens = response.MaxTokens
	}
	if maxTokens <= 0 {
		return protocol.ContextUsageData{}, false
	}
	totalTokens := max(0, response.TotalTokens)
	percentage := float64(totalTokens) / float64(maxTokens) * 100
	percentage = min(100, max(0, percentage))
	return protocol.ContextUsageData{
		TotalTokens: totalTokens,
		MaxTokens:   maxTokens,
		Percentage:  math.Round(percentage*10) / 10,
		Model:       strings.TrimSpace(response.Model),
	}, true
}
