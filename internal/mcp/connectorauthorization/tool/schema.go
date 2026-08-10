// INPUT: 无。
// OUTPUT: 不接受 owner/principal/session/state/device_code/auth_code/token 的严格 schema。
// POS: Connector 授权 MCP 模型参数边界。
package tool

func startSchema() map[string]any {
	return objectSchema(map[string]any{
		"request_id": map[string]any{
			"type": "string", "minLength": 8, "maxLength": 128,
			"description": "本次授权的稳定幂等 ID；重试复用，新的授权意图换新 ID",
		},
		"connector_id": map[string]any{
			"type": "string", "description": "目标 Connector 目录 ID",
		},
		"method": map[string]any{
			"type": "string",
			"enum": []string{"oauth_browser", "device"},
		},
		"device_mode": map[string]any{
			"type":        "string",
			"enum":        []string{"official_qr", "manual_credentials"},
			"description": "仅要求多阶段应用配置的 Device Connector 使用",
		},
		"extras": map[string]any{
			"type": "object",
			"additionalProperties": map[string]any{
				"type": "string", "maxLength": 512,
			},
			"maxProperties": 16,
			"description":   "仅 OAuth provider 声明的非秘密定位参数，例如 shop",
		},
	}, []string{"request_id", "connector_id", "method"})
}

func flowRefSchema() map[string]any {
	return objectSchema(map[string]any{
		"flow_id": map[string]any{
			"type":        "string",
			"description": "start 返回的 opaque flow_id",
		},
		"connector_id": map[string]any{
			"type":        "string",
			"description": "必须与 flow 启动时 Connector 完全一致",
		},
	}, []string{"flow_id", "connector_id"})
}

func objectSchema(
	properties map[string]any,
	required []string,
) map[string]any {
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
