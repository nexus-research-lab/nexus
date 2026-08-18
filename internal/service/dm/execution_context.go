// INPUT: 当前 DM round actor identity 与 Execution Orchestration provider。
// OUTPUT: 每轮 query 前重新读取的 actor-specific hidden execution context。
// POS: DM runtime 不缓存 WorkGraph snapshot 的 fail-closed 注入边界。
package dm

import (
	"context"
	"strings"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtimecommand"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type executionContextProvider interface {
	RuntimeContext(context.Context, orchestrationsvc.ActorContext) (string, error)
}

type executionRuntimeGraphObserver interface {
	BeginRuntimeRound(context.Context, orchestrationsvc.ActorContext) error
	ObserveRuntimeMessage(context.Context, orchestrationsvc.ActorContext, sdkprotocol.ReceivedMessage) error
	ObserveRuntimeCommandReceipts(context.Context, orchestrationsvc.ActorContext, []runtimecommand.Receipt) error
	FinishRuntimeRound(context.Context, orchestrationsvc.ActorContext, string, string) error
}

type executionRuntimeArtifactObserver interface {
	ObserveRuntimeArtifacts(context.Context, orchestrationsvc.ActorContext, protocol.Message) error
}

// SetExecutionContextProvider 注入每轮权威 WorkGraph 上下文读取器。
func (s *Service) SetExecutionContextProvider(provider executionContextProvider) {
	s.executionContext = provider
}

func (s *Service) beginExecutionRuntimeGraph(actor orchestrationsvc.ActorContext) {
	observer, ok := s.executionContext.(executionRuntimeGraphObserver)
	if !ok || observer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := observer.BeginRuntimeRound(ctx, actor); err != nil {
		s.logger.Warn("记录 DM AgentRun 开始失败", "err", err)
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
		s.logger.Warn("记录 DM Runtime NodeRun 失败", "err", err)
	}
}

func (s *Service) observeExecutionRuntimeCommandReceipts(
	actor orchestrationsvc.ActorContext,
	receipts []runtimecommand.Receipt,
) {
	observer, ok := s.executionContext.(executionRuntimeGraphObserver)
	if !ok || observer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := observer.ObserveRuntimeCommandReceipts(ctx, actor, receipts); err != nil {
		s.logger.Warn("核验 DM Execution command NodeRun 失败", "err", err)
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
		s.logger.Warn("关联 DM Runtime Artifact 失败", "err", err)
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
		s.logger.Warn("收口 DM AgentRun 失败", "err", err)
	}
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
			0,
			nil,
		),
	}, nil
}
