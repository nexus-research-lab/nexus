package tool

// scheduleSchema 对齐前端「新建任务」对话框里的调度面板，并兼容 raw cron 表达式：
//   - kind=single   : 对应 UI「单次」
//   - kind=daily    : 对应 UI「每天」(时间 + 星期几)
//   - kind=interval : 对应 UI「间隔」(数值 + 单位)
//   - kind=cron     : 直接传标准 5 段 cron 表达式（对齐 OpenClaw 的易用写法）
var scheduleSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"kind":           map[string]any{"type": "string", "enum": []string{"single", "daily", "interval", "cron"}},
		"run_at":         map[string]any{"type": "string", "description": "single 模式使用，ISO8601 或 YYYY-MM-DDTHH:mm 本地时间"},
		"daily_time":     map[string]any{"type": "string", "description": "daily 模式使用，HH:MM（24 小时）"},
		"weekdays":       map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}}, "description": "daily 模式使用，缺省=每天"},
		"interval_value": map[string]any{"type": "integer", "description": "interval 模式使用，正整数"},
		"interval_unit":  map[string]any{"type": "string", "enum": []string{"seconds", "minutes", "hours"}, "description": "interval 模式使用"},
		"expr":           map[string]any{"type": "string", "description": "cron 模式使用，标准 5 段表达式，如 \"0 9 * * 1-5\"。也接受别名 cron / cron_expression"},
		"timezone":       map[string]any{"type": "string", "description": "IANA 时区（如 Asia/Shanghai）。缺省按服务器默认时区"},
	},
	"required": []string{"kind"},
}

// executionModeSchema 对齐 UI「执行会话」四个按钮。
var executionModeSchema = map[string]any{
	"type":        "string",
	"enum":        []string{"main", "existing", "temporary", "dedicated"},
	"description": "main=使用主会话 / existing=使用现有会话 / temporary=每次新建临时会话 / dedicated=使用专用长期会话",
}

// replyModeSchema 对齐 UI「结果回传」三个按钮。
var replyModeSchema = map[string]any{
	"type":        "string",
	"enum":        []string{"none", "execution", "selected"},
	"description": "none=不回传 / execution=回到执行会话 / selected=回到指定会话",
}

func createSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":                       map[string]any{"type": "string", "description": "任务名称"},
			"agent_id":                   map[string]any{"type": "string", "description": "目标智能体；缺省=当前智能体"},
			"instruction":                map[string]any{"type": "string", "description": "任务指令（Agent 到点要执行的内容）"},
			"schedule":                   scheduleSchema,
			"execution_mode":             executionModeSchema,
			"reply_mode":                 replyModeSchema,
			"selected_session_key":       map[string]any{"type": "string", "description": "execution_mode=existing 时填：要复用的会话 key"},
			"named_session_key":          map[string]any{"type": "string", "description": "execution_mode=dedicated 时填：专用长期会话名称"},
			"selected_reply_session_key": map[string]any{"type": "string", "description": "reply_mode=selected 时填：接收结果的会话 key"},
			"enabled":                    map[string]any{"type": "boolean", "description": "创建后立即启用，缺省 true"},
		},
		"required": []string{"name", "instruction", "schedule"},
	}
}

func updateSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"job_id":                     map[string]any{"type": "string"},
			"name":                       map[string]any{"type": "string"},
			"instruction":                map[string]any{"type": "string"},
			"schedule":                   scheduleSchema,
			"execution_mode":             executionModeSchema,
			"reply_mode":                 replyModeSchema,
			"selected_session_key":       map[string]any{"type": "string"},
			"named_session_key":          map[string]any{"type": "string"},
			"selected_reply_session_key": map[string]any{"type": "string"},
			"enabled":                    map[string]any{"type": "boolean"},
		},
		"required": []string{"job_id"},
	}
}

func jobIDSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"job_id": map[string]any{"type": "string"}},
		"required":   []string{"job_id"},
	}
}
