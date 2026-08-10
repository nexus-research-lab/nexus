// INPUT: model-provided Channel type, optional account target, or opaque flow ID.
// OUTPUT: strict schemas with no principal/scope/lease/QR/verification-code fields.
// POS: nexus_channel_authorization model argument boundary.
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

func startSchema() map[string]any {
	return objectSchema(map[string]any{
		"channel_type": map[string]any{
			"type":        "string",
			"description": "要授权的 Channel 类型；owner 与 Agent 由服务端固定",
		},
		"account_id": map[string]any{
			"type":        "string",
			"description": "可选；要求平台最终返回的精确账号 ID，不匹配时不会保存凭据",
		},
	}, []string{"channel_type"})
}

func flowSchema() map[string]any {
	return objectSchema(map[string]any{
		"flow_id": map[string]any{
			"type":        "string",
			"description": "start 返回的不透明授权 flow_id",
		},
	}, []string{"flow_id"})
}
