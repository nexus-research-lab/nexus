// INPUT: 无。
// OUTPUT: 四个配置工具的稳定 JSON Schema。
// POS: nexus_config 模型调用参数契约。
package tool

var domainEnum = []string{
	"preferences", "providers", "agents", "emotion", "channels", "connectors", "skills",
	"host", "automation", "sessions", "rooms", "workspaces", "goals",
}

var changeProperties = map[string]any{
	"domain": map[string]any{
		"type": "string", "enum": domainEnum,
		"description": "配置域；实际可读域和操作由当前可信 owner/Agent/Room 权限动态过滤",
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
		"description":          "领域服务 JSON 输入。敏感字段必须写成 {\"$secret\":\"8-64位opaque_slot_id\"}；真人只在 Nexus 批准卡的密码框填写，严禁把明文 secret/token/password 放进 tool input",
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
	properties["plan_digest"] = map[string]any{
		"type":        "string",
		"description": "必须原样使用 plan 返回的 plan_digest；它绑定 actor、scope、输入和当前 revision",
	}
	return map[string]any{
		"type": "object", "properties": properties,
		"required": []string{"request_id", "domain", "operation", "input", "expected_revision", "plan_digest"},
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
