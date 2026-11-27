// INPUT: 当前 pending 人工交互集合与前端路由 session。
// OUTPUT: 不含工具输入与 secret 的只读待响应请求快照，按到达顺序稳定排列。
// POS: 编排层判断某个 Agent 是否正被人工交互阻塞的唯一读取入口。
package permission

import (
	"slices"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
)

// PendingInteractionSnapshot 是一个仍在等待人工响应的请求的只读身份视图。
// 刻意不暴露 ToolInput、配置 secret 或响应通道，调用方只能据此判断阻塞状态。
type PendingInteractionSnapshot struct {
	RequestID          string
	RuntimeSessionKey  string
	DispatchSessionKey string
	AgentID            string
	RoundID            string
	AgentRoundID       string
	ToolName           string
	ToolUseID          string
	CreatedAt          time.Time
}

// PendingInteractionsForDispatchSession 返回指定前端路由 session 下所有仍待响应的请求。
// Room 中所有成员 slot 共用同一个 dispatch session，因此一次读取即可覆盖整个会话。
func (c *Context) PendingInteractionsForDispatchSession(dispatchSessionKey string) []PendingInteractionSnapshot {
	dispatchSessionKey = strings.TrimSpace(dispatchSessionKey)
	if dispatchSessionKey == "" {
		return nil
	}

	c.mu.RLock()
	requests := make([]*PendingRequest, 0)
	for _, pending := range c.pendingRequests {
		if strings.TrimSpace(pending.DispatchSessionKey) == dispatchSessionKey {
			requests = append(requests, pending)
		}
	}
	c.mu.RUnlock()
	if len(requests) == 0 {
		return nil
	}

	slices.SortFunc(requests, comparePendingRequests)
	snapshots := make([]PendingInteractionSnapshot, 0, len(requests))
	for _, pending := range requests {
		snapshots = append(snapshots, PendingInteractionSnapshot{
			RequestID:          pending.RequestID,
			RuntimeSessionKey:  strings.TrimSpace(pending.SessionKey),
			DispatchSessionKey: strings.TrimSpace(pending.DispatchSessionKey),
			AgentID: strings.TrimSpace(
				textutil.FirstNonEmpty(pending.Route.AgentID, agentIDFromSessionKey(pending.SessionKey)),
			),
			RoundID:      strings.TrimSpace(pending.Route.RoundID),
			AgentRoundID: strings.TrimSpace(pending.Route.AgentRoundID),
			ToolName:     strings.TrimSpace(pending.ToolName),
			ToolUseID:    strings.TrimSpace(pending.ToolUseID),
			CreatedAt:    pending.CreatedAt,
		})
	}
	return snapshots
}
