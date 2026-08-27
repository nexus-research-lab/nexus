// INPUT: 模型提供的 action、Channel 类型、可选账号目标或 opaque flow ID。
// OUTPUT: 不含 principal/scope/lease/QR/验证码字段的严格 schema。
// POS: Nexus Channel 授权工具的模型参数边界。
package tool

func objectSchema(properties map[string]any, required []string) map[string]any {
	result := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		result["required"] = required
	}
	return result
}

func authorizationSchema() map[string]any {
	return objectSchema(map[string]any{
		"action": map[string]any{
			"type": "string",
			"enum": []string{
				actionStart, actionStatus, actionCancel, actionRequestVerificationCode,
			},
		},
		"channel_type": map[string]any{
			"type":        "string",
			"description": "start 必填；要授权的 Channel 类型，owner 与 Agent 由服务端固定",
		},
		"account_id": map[string]any{
			"type":        "string",
			"description": "可选；要求平台最终返回的精确账号 ID，不匹配时不会保存凭据",
		},
		"flow_id": map[string]any{
			"type":        "string",
			"description": "status/cancel/request_verification_code 必填；start 返回的不透明 flow_id",
		},
	}, []string{"action"})
}
