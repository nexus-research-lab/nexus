// INPUT: 已授权的配置操作定义与经过结构校验的变更请求。
// OUTPUT: 稳定风险等级，以及是否必须消费一次真实交互式人工批准。
// POS: configuration 计划阶段的安全分级边界；不得把模型文本当作批准。
package configuration

import (
	"encoding/json"
	"strings"
)

func classifyChangeRisk(
	operation OperationDefinition,
	request ChangeRequest,
) (string, bool) {
	sensitive := containsSensitiveInput(request.Input)
	requiresConfirmation := operation.RequiresConfirmation ||
		sensitive ||
		changeHasConditionalHumanApproval(request)

	switch {
	case isDestructiveChange(request):
		return "destructive", requiresConfirmation
	case sensitive:
		return "sensitive", requiresConfirmation
	case requiresConfirmation:
		return "high_risk", true
	default:
		return "normal", false
	}
}

func changeHasConditionalHumanApproval(request ChangeRequest) bool {
	if request.Domain != DomainPreferences || request.Operation != "update" {
		return false
	}
	fields := topLevelInputFields(request.Input)
	for _, field := range []string{
		"agent_runtime_kind",
		"runtime_settings",
		"web_search",
		"default_agent_options",
		"default_image_model_selection",
		"default_vision_model_selection",
		"default_background_model_selection",
	} {
		if fields[field] {
			return true
		}
	}
	return false
}

func topLevelInputFields(input json.RawMessage) map[string]bool {
	fields := make(map[string]bool)
	if len(input) == 0 || !json.Valid(input) {
		return fields
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(input, &object) != nil {
		return fields
	}
	for field := range object {
		fields[field] = true
	}
	return fields
}

func isDestructiveChange(request ChangeRequest) bool {
	switch request.Operation {
	case "delete", "delete_config", "delete_account", "delete_pairing",
		"disconnect", "delete_oauth_client", "uninstall", "uninstall_self",
		"remove_member":
		return true
	}
	return strings.HasPrefix(request.Operation, "delete_")
}
