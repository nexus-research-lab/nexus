package tool

var replyRouteSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"mode": map[string]any{
			"type":        "string",
			"enum":        []string{"public", "private", "none"},
			"description": "public=回复进公区；private=回复进指定成员私域；none=不投递 final reply",
		},
		"recipients": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"description": "mode=private 时必填，目标 Room 成员 agent_id 列表",
		},
		"wake_policy": map[string]any{
			"type":        "string",
			"enum":        []string{"none", "immediate"},
			"description": "mode=private 时有效；immediate 会唤醒 route recipients",
		},
		"next_reply_route": map[string]any{
			"type":        "object",
			"description": "只在 mode=private 且 wake_policy=immediate 时使用，表示 route recipient 被唤醒后的下一跳回复路线",
		},
	},
	"required": []string{"mode"},
}

func sendDirectedMessageSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"recipients": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "接收私域消息的 Room 成员 agent_id 列表；单人私聊和小范围讨论都用这个字段",
			},
			"wake_targets": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "需要唤醒的 recipients 子集；不填时唤醒全部 recipients",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "私域消息正文；不会进入 public feed",
			},
			"wake_policy": map[string]any{
				"type":        "string",
				"enum":        []string{"none", "immediate", "delayed"},
				"description": "none=只记录；immediate=立即唤醒 wake_targets；delayed=延迟唤醒",
			},
			"delay_seconds": map[string]any{
				"type":        "integer",
				"description": "wake_policy=delayed 时必填，延迟秒数",
			},
			"reply_route": map[string]any{
				"type":        "object",
				"description": "recipient 被唤醒后 final reply 的投递路线",
				"properties":  replyRouteSchema["properties"],
				"required":    []string{"mode"},
			},
			"correlation_id": map[string]any{
				"type":        "string",
				"description": "可选，仅用于日志/UI 关联分组",
			},
		},
		"required": []string{"recipients", "content", "reply_route"},
	}
}

func publishPublicMessageSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"content": map[string]any{
				"type":        "string",
				"description": "要发布到 Room 公区的事实；普通公区回复不要使用此工具。",
			},
			"correlation_id": map[string]any{
				"type":        "string",
				"description": "可选，仅用于日志/UI 关联分组",
			},
		},
		"required": []string{"content"},
	}
}
