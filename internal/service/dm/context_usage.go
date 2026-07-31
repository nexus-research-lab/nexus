// INPUT: 已结束一轮的 DM runtime client 与 Agent 会话身份。
// OUTPUT: 向当前 session 广播一次权威上下文占用快照。
// POS: DM round 终态与前端 Composer 指标之间的事件桥。
package dm

import (
	"context"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

const contextUsageReadTimeout = 1500 * time.Millisecond

// broadcastContextUsage 在 runtime 支持时广播本轮结束后的上下文快照。
func (r *roundRunner) broadcastContextUsage() {
	if r == nil || r.service == nil || r.client == nil || r.agent == nil {
		return
	}
	ctx, cancel := context.WithTimeout(
		context.Background(),
		contextUsageReadTimeout,
	)
	defer cancel()
	usage, available, err := runtimectx.ReadContextUsage(ctx, r.client)
	if err != nil {
		r.service.loggerFor(context.Background()).Debug(
			"DM context usage 读取失败",
			"session_key", r.sessionKey,
			"agent_id", r.agent.AgentID,
			"round_id", r.roundID,
			"err", err,
		)
		return
	}
	if !available {
		return
	}
	usageSnapshot := usage
	r.session.ContextUsage = &usageSnapshot
	r.service.runtime.RecordContextUsage(
		r.sessionKey,
		r.agent.AgentID,
		usage,
	)
	event := protocol.NewContextUsageEvent(
		r.sessionKey,
		r.agent.AgentID,
		usage,
	)
	event.RoundID = r.roundID
	event.AgentRoundID = r.agentRoundID
	r.service.broadcastEventWithTimeout(
		context.Background(),
		r.sessionKey,
		event,
	)
}
