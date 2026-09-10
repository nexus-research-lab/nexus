// INPUT: Automation operation 字段目录、可信 Actor 与原始 command input。
// OUTPUT: 模型和宿主共用的 closed input schema，以及确认前的 plan 栅栏校验。
// POS: Automation command 的无状态协议校验；业务授权和写入 CAS 仍归 service。
package automation

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
)

// RuntimeCommandPlanView 仅返回当前 Actor 可以原样用于 apply 的输入字段。
func RuntimeCommandPlanView(actor command.Actor, plan automationdomain.AutomationCommandPlan) (map[string]any, error) {
	payload, err := json.Marshal(plan)
	if err != nil {
		return nil, err
	}
	var view map[string]any
	if err := json.Unmarshal(payload, &view); err != nil {
		return nil, err
	}
	definition, ok := runtimeAutomationOperationContracts(actor)[plan.Operation]
	if !ok {
		return nil, fmt.Errorf("未知或无权访问的 Automation operation %q", plan.Operation)
	}
	properties := definition.InputSchema["properties"].(map[string]any)
	input, _ := view["input"].(map[string]any)
	for field := range input {
		if _, allowed := properties[field]; !allowed {
			delete(input, field)
		}
	}
	if err := command.ValidateInput(definition.InputSchema, input); err != nil {
		return nil, err
	}
	return view, nil
}

func runtimeAutomationOperationContracts(actor command.Actor) map[string]automationdomain.AutomationCommandOperationContract {
	contracts := runtimeAutomationOperationFields(actor)
	for name, definition := range contracts {
		properties := map[string]any{}
		for _, field := range append(append([]string{}, definition.Required...), definition.Optional...) {
			properties[field] = automationCommandProperty(field)
		}
		definition.InputSchema = map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
		if len(definition.Required) > 0 {
			definition.InputSchema["required"] = definition.Required
		}
		if definition.Kind == "mutation" {
			definition.Notes = append(definition.Notes, "Use action=plan, then action=apply with identical input and top-level request_id, expected_revision=plan.current_revision, plan_digest=plan.plan_digest. Unknown results require reconciliation; do not replay with a new ID.")
		} else {
			definition.Notes = append(definition.Notes, "Use action=inspect with this operation.")
		}
		contracts[name] = definition
	}
	return contracts
}

func automationCommandProperty(field string) map[string]any {
	text := func(description string, values ...string) map[string]any {
		property := map[string]any{"type": "string", "description": description}
		if len(values) > 0 {
			property["enum"] = values
		}
		return property
	}
	switch field {
	case "enabled", "deliver_result", "clear_expires_at", "cancel_active_run", "include_active", "include_deleted":
		return map[string]any{"type": "boolean"}
	case "limit", "run_limit", "event_limit":
		return map[string]any{"type": "integer", "minimum": 0, "description": "0 使用默认数量。"}
	case "context_mode":
		return text("执行上下文；省略使用默认策略。", "current", "isolated")
	case "permission_mode":
		return text("任务执行权限模式。", "default", "plan", "acceptEdits", "bypassPermissions", "dontAsk")
	case "overlap_policy":
		return text("运行重叠策略。", "skip", "allow")
	case "execution_mode":
		return text("仅主智能体可选择的执行会话策略。", "main", "existing", "temporary", "dedicated")
	case "reply_mode":
		return text("仅主智能体可选择的投递策略。", "none", "execution", "selected", "channel")
	case "schedule":
		return map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"kind"},
			"description": "single 必须提供 run_at；daily 必须提供 daily_time；interval 必须提供 interval_value；cron 必须提供 expr。",
			"properties": map[string]any{
				"kind":           text("调度种类。", "single", "daily", "interval", "cron"),
				"run_at":         text("single 必填，RFC3339 时间。"),
				"daily_time":     text("daily 必填，HH:MM。"),
				"weekdays":       map[string]any{"type": "array", "items": text("星期。", "su", "mo", "tu", "we", "th", "fr", "sa", "sun", "mon", "tue", "wed", "thu", "fri", "sat")},
				"interval_value": map[string]any{"type": "integer", "minimum": 1, "description": "interval 必填的正整数。"},
				"interval_unit":  text("省略使用 seconds。", "seconds", "minutes", "hours"),
				"expr":           text("cron 必填，标准五段表达式，不含秒。"),
				"timezone":       text("IANA 时区，如 Asia/Shanghai；省略使用宿主默认值。"),
			},
		}
	case "name", "instruction", "instruction_append":
		return map[string]any{"type": "string", "pattern": `\S`, "description": "非空任务文本。"}
	case "expires_at":
		return text("RFC3339 到期时间；清除使用 clear_expires_at=true。")
	case "date":
		return text("报表日期 YYYY-MM-DD。")
	default:
		return text("按当前 contract 和 inspect 返回值填写；不猜测资源或会话标识。")
	}
}

// ValidateRuntimeCommandInput 在读取资源和真人确认前验证模型原始输入。
func ValidateRuntimeCommandInput(actor command.Actor, action, operation string, input map[string]any) error {
	definition, ok := runtimeAutomationOperationContracts(actor)[operation]
	if !ok {
		return fmt.Errorf("未知或无权访问的 Automation operation %q；请读取 action=contract", operation)
	}
	if definition.Kind == "query" && action != command.ActionInspect {
		return fmt.Errorf("Automation %s 是查询，请使用 action=inspect 并保留 operation", operation)
	}
	if definition.Kind == "mutation" && action != command.ActionPlan && action != command.ActionApply && action != command.ActionReplay {
		return fmt.Errorf("Automation %s 是变更，请先 action=plan，再 action=apply", operation)
	}
	return command.ValidateInput(definition.InputSchema, input)
}

// ValidateRuntimeCommandPlan 确认前预检；ApplyRuntimeCommand 在写入前重新 plan 并再次校验。
func ValidateRuntimeCommandPlan(request automationdomain.AutomationCommandRequest, plan automationdomain.AutomationCommandPlan) error {
	if err := command.ValidateRequestID(request.RequestID); err != nil {
		return err
	}
	if strings.TrimSpace(request.ExpectedRevision) == "" {
		return errors.New("apply 缺少顶层 expected_revision；请复制 plan 返回的 current_revision，本次未执行")
	}
	if strings.TrimSpace(request.PlanDigest) == "" {
		return errors.New("apply 缺少顶层 plan_digest；请复制 plan 返回的 plan_digest，本次未执行")
	}
	if request.ExpectedRevision != plan.CurrentRevision {
		return errors.New("Automation 状态已变化：expected_revision 与当前 revision 不匹配；请重新 plan")
	}
	if request.PlanDigest != plan.PlanDigest {
		return errors.New("plan_digest 与当前 Actor、输入或 revision 不匹配；请重新 plan")
	}
	return nil
}
