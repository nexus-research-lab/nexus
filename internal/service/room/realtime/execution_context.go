// INPUT: 当前 Room slot/round identity 与 Execution Orchestration provider。
// OUTPUT: 每轮 query 前重新读取、按当前成员裁剪的 hidden execution context。
// POS: Room runtime 不从聊天文本猜 WorkGraph 的 fail-closed 注入边界。
package realtime

import (
	"context"
	"strings"

	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	orchestrationruntimehook "github.com/nexus-research-lab/nexus/internal/service/orchestration/runtimehook"
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

type executionContextProvider interface {
	RuntimeContext(context.Context, orchestrationsvc.ActorContext) (string, error)
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

// SetExecutionContextProvider 注入每轮权威 WorkGraph 上下文读取器。
func (s *Service) SetExecutionContextProvider(provider executionContextProvider) {
	s.executionContext = provider
}

func (s *Service) executionContextualInputs(
	ctx context.Context,
	actor orchestrationsvc.ActorContext,
) ([]runtimectx.ContextualInputBlock, error) {
	if s.executionContext == nil {
		return nil, nil
	}
	content, err := s.executionContext.RuntimeContext(ctx, actor)
	if err != nil {
		return nil, err
	}
	if content = strings.TrimSpace(content); content == "" {
		return nil, nil
	}
	return []runtimectx.ContextualInputBlock{
		runtimectx.NewContextualInputBlock(
			runtimectx.ContextualInputNameExecution,
			content,
			runtimectx.ContextualInputPriorityExecution,
			nil,
		),
	}, nil
}

func (s *Service) executionGoalBinding(
	ctx context.Context,
	actor orchestrationsvc.ActorContext,
) (orchestrationsvc.RuntimeGoalBinding, error) {
	provider, ok := s.executionContext.(executionGoalBindingProvider)
	if !ok || provider == nil {
		return orchestrationsvc.RuntimeGoalBinding{}, nil
	}
	return provider.RuntimeGoalBinding(ctx, actor)
}

func (s *Service) releaseExecutionCoordination(
	actor orchestrationsvc.ActorContext,
) {
	provider, ok := s.executionContext.(executionCoordinationLifecycle)
	if !ok || provider == nil {
		return
	}
	provider.ReleaseRuntimeCoordination(actor)
}

func (s *Service) executionObserver() orchestrationruntimehook.Observer {
	return orchestrationruntimehook.Observer{Provider: s.executionContext, Logger: s.logger}
}
