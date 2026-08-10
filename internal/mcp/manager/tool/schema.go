// INPUT: 当前工具是否由 owner-main 调用。
// OUTPUT: 不允许额外字段、且不接受 owner/actor/round/scope 覆盖的 JSON Schema。
// POS: nexus_manager 模型参数契约。
package tool

func emptySchema() map[string]any {
	return objectSchema(map[string]any{}, nil)
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	result := map[string]any{
		"type": "object", "properties": properties, "additionalProperties": false,
	}
	if len(required) > 0 {
		result["required"] = required
	}
	return result
}

func listSchema(withAgentID bool) map[string]any {
	properties := map[string]any{
		"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 1000},
	}
	if withAgentID {
		properties["agent_id"] = map[string]any{
			"type": "string", "description": "当前 owner 下的目标 agent_id",
		}
	}
	return objectSchema(properties, nil)
}

func readFileSchema(withAgentID bool) map[string]any {
	properties := map[string]any{
		"path": map[string]any{
			"type": "string", "description": "workspace 内相对路径；绝对路径和越界路径会被拒绝",
		},
		"max_bytes": map[string]any{
			"type": "integer", "minimum": 1, "maximum": 131072,
			"description": "最多返回的 UTF-8 内容字节数；默认 32768，最大 131072",
		},
	}
	required := []string{"path"}
	if withAgentID {
		properties["agent_id"] = map[string]any{
			"type": "string", "description": "当前 owner 下的目标 agent_id",
		}
		required = append([]string{"agent_id"}, required...)
	}
	return objectSchema(properties, required)
}

func idSchema(key string, description string) map[string]any {
	return objectSchema(map[string]any{
		key: map[string]any{"type": "string", "description": description},
	}, []string{key})
}

func listRoomContextsSchema() map[string]any {
	return objectSchema(map[string]any{
		"room_id": map[string]any{"type": "string", "description": "当前 owner 下的 room_id"},
		"limit":   map[string]any{"type": "integer", "minimum": 1, "maximum": 1000},
	}, []string{"room_id"})
}

func listSessionsSchema() map[string]any {
	return objectSchema(map[string]any{
		"agent_id": map[string]any{
			"type": "string", "description": "可选；限定到当前 owner 下的一个 Agent",
		},
		"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 1000},
	}, nil)
}
