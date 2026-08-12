// INPUT: 当前 DM 的 durable history、用户请求类型与 Goal 上下文。
// OUTPUT: 仅在真实用户新一轮注入的失败恢复上下文集合。
// POS: DM 历史失败语义到 runtime contextual inputs 的薄适配层。
package dm

import (
	"strings"

	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
)

func (e *dmChatExecution) recoveryContextualInputs() []runtimectx.ContextualInputBlock {
	if e.request.Internal || strings.TrimSpace(e.request.RewriteTargetRoundID) != "" {
		return nil
	}
	history, err := e.service.history.ReadMessages(e.agent.WorkspacePath, e.session, nil)
	if err != nil {
		e.service.loggerFor(e.ctx).Warn(
			"读取 DM 上一轮失败上下文失败",
			"session_key", e.sessionKey,
			"agent_id", e.agent.AgentID,
			"err", err,
		)
		return nil
	}
	// AgentHistoryStore 已按当前 DM Agent 隔离，因此不再要求历史行携带 agent_id。
	inputs := conversationsvc.AutomationDeliveryContextualInputs(history, e.request.RoundID)
	return append(inputs, conversationsvc.RoundRecoveryContextualInputs(history, "")...)
}

func (r *roundRunner) contextualInputs() []runtimectx.ContextualInputBlock {
	if r.atomicInput {
		return nil
	}
	inputs := r.transportContextualInputs()
	inputs = append(inputs, runtimectx.AutomationRunContextualInputs(r.automationRun)...)
	inputs = append(inputs, goalContextualInputs(r.goalContext, r.goalIDForUsage, r.sessionKey)...)
	return append(inputs, r.recoveryContext...)
}
