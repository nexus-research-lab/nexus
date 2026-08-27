// INPUT: Goal service 与 command context context。
// OUTPUT: 模型可见的完整 Goal 工具集合。
// POS: Goal command 操作注册入口。
package operation

import (
	command "github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/mcp/command/goal/contract"
)

// BuildAll 汇集 Codex Goal 对齐的模型可见工具。
func BuildAll(svc contract.Service, sctx contract.Context) []command.Operation {
	return []command.Operation{
		getGoal(svc, sctx),
		createGoal(svc, sctx),
		retargetGoal(svc, sctx),
		auditObjectiveAlignment(svc, sctx),
		updateGoal(svc, sctx),
	}
}

const (
	searchHintGetGoal      = "goal current status budget usage remaining tokens 当前目标 状态 预算 用量"
	searchHintCreateGoal   = "goal create start objective token_budget long running task 创建 启动 长程目标 预算"
	searchHintRetargetGoal = "goal retarget correct replace objective explicit user correction 更正 修正 替换 当前目标"
	searchHintAuditGoal    = "goal objective alignment completion criteria evidence audit verify target 对齐 完成标准 证据 审计"
	searchHintUpdateGoal   = "goal update complete blocked finish completion audit 标记完成 阻塞 审计"
)

func objectSchema(properties map[string]any, required ...string) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
		"required":             append([]string{}, required...),
	}
}

func stringProperty(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
	}
}

func integerProperty(description string) map[string]any {
	return map[string]any{
		"type":        "integer",
		"description": description,
	}
}

func enumStringProperty(description string, values ...string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
		"enum":        values,
	}
}
