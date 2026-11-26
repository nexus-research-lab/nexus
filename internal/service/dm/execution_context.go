// INPUT: 当前 DM round actor identity 与 Execution Orchestration provider。
// OUTPUT: 每轮 query 前重新读取的 actor-specific hidden execution context。
// POS: DM runtime 不缓存 WorkGraph snapshot 的 fail-closed 注入边界；恢复上下文只读历史尾页，成本不随会话长度增长。
package dm

import (
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// recoveryHistoryRounds 覆盖恢复上下文需要的全部尾部轮次：当前轮、上一轮终态，
// 以及上一条可见用户消息之后最多 5 条 automation 投递（每条投递各自成轮）。
const recoveryHistoryRounds = 8

func (e *dmChatExecution) recoveryContextualInputs() []runtimectx.ContextualInputBlock {
	if e.request.Internal || e.request.RewriteTargetRoundID != "" {
		return runtimehost.QueueRecoveryInputs(e.ctx)
	}
	page, err := e.service.History.ReadMessagesPageContext(
		e.ctx,
		e.agent.WorkspacePath,
		e.session,
		nil,
		workspacestore.HistoryPageQuery{Limit: recoveryHistoryRounds},
	)
	history := page.Items
	if err != nil && e.ctx.Err() == nil {
		// 读模型不可用时退回完整历史，恢复上下文不能因分页索引故障而静默丢失。
		history, err = e.service.History.ReadMessages(e.agent.WorkspacePath, e.session, nil)
	}
	if err != nil {
		e.service.LoggerFor(e.ctx).Warn(
			"读取 DM 上一轮失败上下文失败",
			"session_key", e.sessionKey,
			"agent_id", e.agent.AgentID,
			"err", err,
		)
		return nil
	}
	// AgentHistoryStore 已按当前 DM Agent 隔离，因此不再要求历史行携带 agent_id。
	inputs := append(runtimehost.QueueRecoveryInputs(e.ctx), conversationsvc.AutomationDeliveryContextualInputs(history, e.request.RoundID)...)
	if e.service.imReplies != nil {
		deliveries, readErr := e.service.imReplies.List(e.ctx, e.agent.OwnerUserID, e.sessionKey, "", 0, 5)
		if readErr == nil {
			inputs = append(inputs, conversationsvc.IMDeliveryContextualInputs(deliveries)...)
		}
	}

	return append(inputs, conversationsvc.RoundRecoveryContextualInputs(history, "", e.request.RoundID)...)
}

func (r *roundRunner) contextualInputs() []runtimectx.ContextualInputBlock {
	if r.atomicInput {
		return nil
	}
	inputs := r.transportContextualInputs()
	inputs = append(inputs, r.connectorContextualInputs()...)
	inputs = append(inputs, r.followUpContextualInputs()...)
	inputs = append(inputs, runtimectx.AutomationRunContextualInputs(r.automationRun)...)
	inputs = append(inputs, runtimectx.GoalContextualInputs(r.Context, r.IDForUsage, r.sessionKey)...)
	return append(inputs, r.recoveryContext...)
}
