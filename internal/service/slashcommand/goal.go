// INPUT: `/goal` host invocation、可信 UI Goal options 与 DM/Room Goal command executor。
// OUTPUT: 不进入模型 runtime 的 Goal 设置事务、durable 控制记录 ACK 与响应后空闲续跑。
// POS: Goal 按钮和 Slash 文本共用的唯一 host command 定义。
package slashcommand

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	goalCommandName         = "goal"
	goalCommandDescription  = "Set or replace the Goal for this Session"
	goalCommandArgumentHint = "<objective>"
)

// GoalCommandExecutor 把 host command 路由到 owner-scoped DM 或 Room 领域。
// DispatchGoalContinuation 只会在 ACK/terminal control event 已尝试发送后调用。
type GoalCommandExecutor interface {
	ExecuteGoalCommand(context.Context, protocol.GoalCommandRequest) (protocol.GoalCommandResult, error)
	DispatchGoalContinuation(context.Context, protocol.Goal)
}

// GoalCommandDependencies 是 `/goal` 注册依赖。
type GoalCommandDependencies struct {
	Executor GoalCommandExecutor
}

// RegisterGoalCommand 注册 DM/Room 共用的 Nexus Goal 控制命令。
func RegisterGoalCommand(registry *Registry, dependencies GoalCommandDependencies) error {
	if registry == nil {
		return errors.New("slash command registry is nil")
	}
	if dependencies.Executor == nil {
		return errors.New("goal command executor is required")
	}
	return registry.Register(Definition{
		Name:         goalCommandName,
		Description:  goalCommandDescription,
		ArgumentHint: goalCommandArgumentHint,
		Scopes:       []Scope{ScopeDM, ScopeRoom},
		Enabled:      true,
		Handler: func(ctx context.Context, invocation Invocation) (Result, error) {
			return executeGoalCommand(ctx, dependencies.Executor, invocation)
		},
	})
}

func executeGoalCommand(
	ctx context.Context,
	executor GoalCommandExecutor,
	invocation Invocation,
) (Result, error) {
	objective := strings.TrimSpace(invocation.Arguments)
	if objective == "" {
		return Result{}, commandInputError{message: "用法：/goal <objective>"}
	}
	execution, err := executor.ExecuteGoalCommand(ctx, protocol.GoalCommandRequest{
		SessionKey:      strings.TrimSpace(invocation.SessionKey),
		AgentID:         strings.TrimSpace(invocation.AgentID),
		Objective:       objective,
		CommandContent:  strings.TrimSpace(invocation.Content),
		RoundID:         strings.TrimSpace(invocation.RoundID),
		UserMessageID:   strings.TrimSpace(invocation.UserMessageID),
		ClientRequestID: strings.TrimSpace(invocation.ClientRequestID),
		ClientMessageID: strings.TrimSpace(invocation.ClientMessageID),
		TargetAgentIDs:  append([]string(nil), invocation.TargetAgentIDs...),
		Options:         invocation.GoalOptions,
	})
	if err != nil {
		return Result{}, err
	}
	goal := execution.Goal
	return Result{
		UserMessageCommitted: execution.UserMessageCommitted,
		AfterResponseAttempted: func(afterCtx context.Context) {
			executor.DispatchGoalContinuation(afterCtx, goal)
		},
	}, nil
}
