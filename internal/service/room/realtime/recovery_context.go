// INPUT: Room slot 的触发来源、共享历史与 Goal 上下文。
// OUTPUT: 仅为真实用户触发的目标 Agent 注入失败恢复上下文。
// POS: Room 多 Agent 历史失败语义到 runtime contextual inputs 的薄适配层。
package realtime

import (
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
)

func (e *slotExecution) contextualInputs() []runtimectx.ContextualInputBlock {
	if e.slot == nil {
		return nil
	}
	inputs := goalContextualInputs(e.slot.goalContext(), e.slot.goalIDForUsage(), goalSessionKeyForSlot(e.slot))
	if e.round != nil {
		inputs = append(runtimectx.AutomationRunContextualInputs(e.round.AutomationRun), inputs...)
	}
	if e.round == nil || e.round.Internal {
		return inputs
	}
	switch e.slot.Trigger.TriggerType {
	case "public_chat", "room_host_default":
		return append(inputs, conversationsvc.RoundRecoveryContextualInputs(e.history, e.slot.AgentID)...)
	default:
		return inputs
	}
}
