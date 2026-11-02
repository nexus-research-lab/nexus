// INPUT: 当前 Room slot/round identity、Execution Orchestration provider 与 IM 入站轮次的回信说明。
// OUTPUT: 每轮 query 前重新读取、按当前成员裁剪的 hidden execution context，以及本轮 IM 回信上下文。
// POS: Room runtime 不从聊天文本猜 WorkGraph 的 fail-closed 注入边界。
package realtime

import (
	"context"

	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func (e *slotExecution) contextualInputs() []runtimectx.ContextualInputBlock {
	if e.slot == nil {
		return nil
	}
	inputs := runtimectx.GoalContextualInputs(e.slot.goalContext(), e.slot.mutable.goal.UsageGoalID(), goalSessionKeyForSlot(e.slot))
	if e.round != nil {
		inputs = append(runtimectx.AutomationRunContextualInputs(e.round.AutomationRun), inputs...)
	}
	if e.externalReplyContext != "" {
		inputs = append(inputs, runtimectx.NewContextualInputBlock(
			"im_delivery",
			e.externalReplyContext,
			runtimectx.ContextualInputPriorityAutomationDelivery,
			nil,
		))
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

type executionGoalBindingProvider interface {
	RuntimeGoalBinding(
		context.Context,
		orchestrationsvc.ActorContext,
	) (orchestrationsvc.RuntimeGoalBinding, error)
}

type executionCoordinationLifecycle interface {
	ReleaseRuntimeCoordination(orchestrationsvc.ActorContext)
}

func (s *Service) executionGoalBinding(
	ctx context.Context,
	actor orchestrationsvc.ActorContext,
) (orchestrationsvc.RuntimeGoalBinding, error) {
	provider, ok := s.ExecutionContext.(executionGoalBindingProvider)
	if !ok || provider == nil {
		return orchestrationsvc.RuntimeGoalBinding{}, nil
	}
	return provider.RuntimeGoalBinding(ctx, actor)
}

func (s *Service) releaseExecutionCoordination(
	actor orchestrationsvc.ActorContext,
) {
	provider, ok := s.ExecutionContext.(executionCoordinationLifecycle)
	if !ok || provider == nil {
		return
	}
	provider.ReleaseRuntimeCoordination(actor)
}
