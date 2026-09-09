// INPUT: 当前 DM round actor identity 与 Execution Orchestration provider。
// OUTPUT: 每轮 query 前重新读取的 actor-specific hidden execution context。
// POS: DM runtime 不缓存 WorkGraph snapshot 的 fail-closed 注入边界。
package dm

import (
	"context"
	"strings"

	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	orchestrationruntimehook "github.com/nexus-research-lab/nexus/internal/service/orchestration/runtimehook"
)

func goalContextualInputs(contextText string, goalID string, sessionKey string) []runtimectx.ContextualInputBlock {
	contextText = strings.TrimSpace(contextText)
	if contextText == "" {
		return nil
	}
	metadata := map[string]string{}
	if goalID = strings.TrimSpace(goalID); goalID != "" {
		metadata["goal_id"] = goalID
	}
	if sessionKey = strings.TrimSpace(sessionKey); sessionKey != "" {
		metadata["session_key"] = sessionKey
	}
	return []runtimectx.ContextualInputBlock{
		runtimectx.NewContextualInputBlock(
			runtimectx.ContextualInputNameGoal,
			contextText,
			runtimectx.ContextualInputPriorityGoal,
			metadata,
		),
	}
}

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

type executionContextProvider interface {
	RuntimeContext(context.Context, orchestrationsvc.ActorContext) (string, error)
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

func (s *Service) executionObserver() orchestrationruntimehook.Observer {
	return orchestrationruntimehook.Observer{Provider: s.executionContext, Logger: s.logger}
}
