// INPUT: 当前 Room slot/round identity 与 Execution Orchestration provider。
// OUTPUT: 每轮 query 前重新读取、按当前成员裁剪的 hidden execution context。
// POS: Room runtime 不从聊天文本猜 WorkGraph 的 fail-closed 注入边界。
package realtime

import (
	"context"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"strings"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
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

type executionRuntimeGraphObserver interface {
	BeginRuntimeRound(context.Context, orchestrationsvc.ActorContext) error
	ObserveRuntimeMessage(context.Context, orchestrationsvc.ActorContext, sdkprotocol.ReceivedMessage) error
	ObserveRuntimeCommandReceipts(context.Context, orchestrationsvc.ActorContext, []nexusmcp.CommandReceipt) error
	FinishRuntimeRound(context.Context, orchestrationsvc.ActorContext, string, string) error
}

type executionRuntimeArtifactObserver interface {
	ObserveRuntimeArtifacts(context.Context, orchestrationsvc.ActorContext, protocol.Message) error
}

type executionGoalBindingProvider interface {
	RuntimeGoalBinding(
		context.Context,
		orchestrationsvc.ActorContext,
	) (orchestrationsvc.RuntimeGoalBinding, error)
}

func (s *Service) beginExecutionRuntimeGraph(actor orchestrationsvc.ActorContext) {
	observer, ok := s.executionContext.(executionRuntimeGraphObserver)
	if !ok || observer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := observer.BeginRuntimeRound(ctx, actor); err != nil {
		s.logger.Warn("记录 Room AgentRun 开始失败", "err", err)
	}
}

func (s *Service) observeExecutionRuntimeGraph(
	actor orchestrationsvc.ActorContext,
	message sdkprotocol.ReceivedMessage,
) {
	observer, ok := s.executionContext.(executionRuntimeGraphObserver)
	if !ok || observer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := observer.ObserveRuntimeMessage(ctx, actor, message); err != nil {
		s.logger.Warn("记录 Room Runtime NodeRun 失败", "err", err)
	}
}

func (s *Service) observeExecutionRuntimeCommandReceipts(
	actor orchestrationsvc.ActorContext,
	receipts []nexusmcp.CommandReceipt,
) {
	observer, ok := s.executionContext.(executionRuntimeGraphObserver)
	if !ok || observer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := observer.ObserveRuntimeCommandReceipts(ctx, actor, receipts); err != nil {
		s.logger.Warn("核验 Room Execution command NodeRun 失败", "err", err)
	}
}

func (s *Service) observeExecutionRuntimeArtifacts(
	actor orchestrationsvc.ActorContext,
	message protocol.Message,
) {
	observer, ok := s.executionContext.(executionRuntimeArtifactObserver)
	if !ok || observer == nil || message == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := observer.ObserveRuntimeArtifacts(ctx, actor, message); err != nil {
		s.logger.Warn("关联 Room Runtime Artifact 失败", "err", err)
	}
}

func (s *Service) finishExecutionRuntimeGraph(
	actor orchestrationsvc.ActorContext,
	terminalStatus string,
	failureReason string,
) {
	observer, ok := s.executionContext.(executionRuntimeGraphObserver)
	if !ok || observer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := observer.FinishRuntimeRound(ctx, actor, terminalStatus, failureReason); err != nil {
		s.logger.Warn("收口 Room AgentRun 失败", "err", err)
	}
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
