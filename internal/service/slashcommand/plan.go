// INPUT: 用户 Slash 原文、内部来源标记与当前请求权限覆盖。
// OUTPUT: /plan 的目录、规划提示和仅限当前请求的 Plan Mode。
// POS: 显式规划入口；复用 runtime 的 ExitPlanMode 确认，不修改 Agent 或 Session 设置。
package slashcommand

import (
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const planCommandName = "plan"

// PlanCommandDescriptor 返回显式规划入口。
func PlanCommandDescriptor() protocol.CommandDescriptor {
	return newRuntimeCommand(planCommandName, "Plan a task before executing it", "<optional request>")
}

// PlanRequestPermissionMode 在用户显式 /plan 请求上收紧权限；内部输入不改变权限。
func PlanRequestPermissionMode(content string, internal bool, current sdkpermission.Mode) sdkpermission.Mode {
	name, _, ok := parseInvocation(content)
	if !internal && ok && name == planCommandName {
		return sdkpermission.ModePlan
	}
	return current
}

func expandPlanPrompt(arguments string) string {
	if arguments == "" {
		arguments = "Use the actionable request already established in this conversation. If no task has been established, ask the user what they want to plan."
	}
	return "Plan the following request in Plan Mode. Explore and clarify requirements, then write a concrete plan to the runtime plan file. Do not implement the task before approval. Use ExitPlanMode to present the plan for user approval; only continue implementation after that approval succeeds.\n\nRequest:\n" + arguments
}
