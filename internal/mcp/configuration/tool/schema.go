// INPUT: 无。
// OUTPUT: 四个配置工具的稳定 JSON Schema。
// POS: nexus_config 模型调用参数契约。
package tool

var domainEnum = []string{
	"preferences", "providers", "agents", "channels", "connectors", "skills",
	"host", "automation", "rooms", "workspaces", "goals",
}

var changeProperties = map[string]any{
	"domain": map[string]any{
		"type": "string", "enum": domainEnum,
		"description": "配置域；automation/rooms/workspaces/goals 会指向专用工具，不接受通用写入",
	},
	"operation": map[string]any{
		"type":        "string",
		"description": "inspect 返回的 definition.operations[].name；不要猜测操作名",
	},
	"target": map[string]any{
		"type":        "string",
		"description": "资源标识。例：provider 名、agent_id、channel_type、connector_id、skill 名、pairing_id",
	},
	"input": map[string]any{
		"type":                 "object",
		"description":          "领域服务原生 JSON 输入。敏感值只在此传入，结果与审计永不回显明文",
		"additionalProperties": true,
	},
}

func inspectSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"domains": map[string]any{
				"type": "array", "items": map[string]any{"type": "string", "enum": domainEnum},
				"description": "要读取的域；空数组或省略表示全部",
			},
			"verify": map[string]any{
				"type":        "boolean",
				"description": "执行本地确定性核对。不会发起 Provider 网络请求；远端检查需显式 test 操作",
			},
		},
	}
}

func planSchema() map[string]any {
	return map[string]any{
		"type": "object", "properties": changeProperties,
		"required": []string{"domain", "operation", "input"},
	}
}

func applySchema() map[string]any {
	properties := make(map[string]any, len(changeProperties)+3)
	for key, value := range changeProperties {
		properties[key] = value
	}
	properties["request_id"] = map[string]any{
		"type":        "string",
		"description": "本次变更的唯一幂等 ID，8-128 位；网络重试必须复用，新的修正操作必须换新 ID",
	}
	properties["expected_revision"] = map[string]any{
		"type":        "string",
		"description": "必须等于 plan 返回的 current_revision；不同则拒绝覆盖新配置",
	}
	properties["confirm"] = map[string]any{
		"type":        "boolean",
		"description": "仅在 plan.requires_confirmation=true 且用户明确确认后传 true",
	}
	return map[string]any{
		"type": "object", "properties": properties,
		"required": []string{"request_id", "domain", "operation", "input", "expected_revision"},
	}
}

func historySchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"domain": map[string]any{"type": "string", "enum": domainEnum},
			"limit":  map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
		},
	}
}
