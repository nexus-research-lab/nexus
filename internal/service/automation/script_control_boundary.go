// INPUT: automation 管理动作上下文及任务变更前后的 execution_kind。
// OUTPUT: Agent 发起的 script 创建、变更、删除、运行与修复拒绝结果。
// POS: 人类控制面 script 能力在 service 真实动作入口的最终授权边界。
package automation

import (
	"context"
	"errors"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

var errAgentScriptControl = errors.New("script scheduled tasks are human-control-plane only and cannot be controlled through an Agent conversation")

func rejectAgentScriptControl(
	ctx context.Context,
	tasks ...automationdomain.ScheduledTask,
) error {
	if _, ok := automationexec.ActorAgentID(ctx); !ok {
		return nil
	}
	for _, task := range tasks {
		if automationdomain.NormalizeExecutionKind(task.ExecutionKind) == automationdomain.ExecutionKindScript {
			return errAgentScriptControl
		}
	}
	return nil
}

func rejectAgentScriptCreate(ctx context.Context, input automationdomain.CreateJobInput) error {
	return rejectAgentScriptControl(ctx, automationdomain.ScheduledTask{
		ExecutionKind: input.ExecutionKind,
	})
}
