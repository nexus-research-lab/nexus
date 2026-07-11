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
		"weekdays":       map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []string{"mo", "tu", "we", "th", "fr", "sa", "su", "mon", "tue", "wed", "thu", "fri", "sat", "sun"}}, "description": "daily 模式使用，缺省=每天；兼容 UI 短值 mo/tu/... 与英文值 mon/tue/..."},
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

// replyModeSchema 对齐 UI「结果回传」按钮，并补 Agent/IM 长程投递入口。
var replyModeSchema = map[string]any{
	"type":        "string",
	"enum":        []string{"none", "execution", "selected", "agent", "channel"},
	"description": "none=不回传 / execution=回到执行会话 / selected=回到指定会话 / agent=投递到智能体定时任务收件箱 / channel=投递到显式通道或 IM 会话",
}

func createSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":                       map[string]any{"type": "string", "description": "任务名称"},
			"agent_id":                   map[string]any{"type": "string", "description": "目标智能体；缺省=当前智能体"},
			"instruction":                map[string]any{"type": "string", "description": "任务指令（Agent 到点要执行的内容）"},
			"execution_kind":             map[string]any{"type": "string", "enum": []string{"agent", "script"}, "description": "agent=交给 Agent 会话执行；script=直接在目标 Agent workspace 中执行 instruction 脚本"},
			"schedule":                   scheduleSchema,
			"execution_mode":             executionModeSchema,
			"reply_mode":                 replyModeSchema,
			"overlap_policy":             map[string]any{"type": "string", "enum": []string{"skip", "allow"}, "description": "重叠触发策略：skip=有运行中任务时跳过；allow=允许并发执行。缺省 skip"},
			"expires_at":                 map[string]any{"type": "string", "description": "可选。任务生命周期截止时间，RFC3339；到期后停止后续触发，但不中断正在执行的任务"},
			"selected_session_key":       map[string]any{"type": "string", "description": "execution_mode=existing 时填：要复用的会话 key"},
			"named_session_key":          map[string]any{"type": "string", "description": "execution_mode=dedicated 时填：专用长期会话名称"},
			"selected_reply_session_key": map[string]any{"type": "string", "description": "reply_mode=selected 时填：接收结果的会话 key"},
			"reply_agent_id":             map[string]any{"type": "string", "description": "reply_mode=agent 时可填：接收结果的智能体；缺省=任务目标智能体"},
			"reply_session_key":          map[string]any{"type": "string", "description": "reply_mode=channel 时可填：结构化 IM/session key，如 agent:<agent_id>:fs:group:<chat_id>；当前会话就是结构化外部 IM 群时可省略"},
			"reply_channel":              map[string]any{"type": "string", "description": "reply_mode=channel 时填：websocket/internal/telegram/discord/dingtalk/wechat/weixin-personal/feishu 等通道"},
			"reply_to":                   map[string]any{"type": "string", "description": "reply_mode=channel 时填：目标会话 key、外部群/频道 id 或 chat_id"},
			"reply_account_id":           map[string]any{"type": "string", "description": "reply_mode=channel 时可填：多账号通道账号 id"},
			"reply_thread_id":            map[string]any{"type": "string", "description": "reply_mode=channel 时可填：话题/线程 id"},
			"enabled":                    map[string]any{"type": "boolean", "description": "创建后立即启用，缺省 true"},
		},
		"required": []string{"name", "instruction", "schedule"},
	}
}

func updateSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"job_id":                     map[string]any{"type": "string", "description": "要修改的任务 id；也可改传 query 让工具在当前权限范围内定位唯一任务"},
			"query":                      map[string]any{"type": "string", "description": "可选。没有 job_id 时按名称、内容、投递目标或状态定位唯一当前未删除任务；当前 DM/Room/IM 群里会优先当前会话匹配，写“这里/当前会话/这个群/当前频道”会强制限定；多候选时不会修改"},
			"agent_id":                   map[string]any{"type": "string", "description": "主智能体可填：把 query 限定到某个智能体；普通 agent 会被强制限定为自己"},
			"name":                       map[string]any{"type": "string"},
			"instruction":                map[string]any{"type": "string", "description": "完整替换任务内容；用户只是说“再加一条要求/补充细节”时优先用 instruction_append"},
			"instruction_append":         map[string]any{"type": "string", "description": "追加到当前任务内容末尾，适合“再加上/补充/以后也要”这类增量修改；不要和 instruction 同时传"},
			"execution_kind":             map[string]any{"type": "string", "enum": []string{"agent", "script"}},
			"schedule":                   scheduleSchema,
			"execution_mode":             executionModeSchema,
			"reply_mode":                 replyModeSchema,
			"overlap_policy":             map[string]any{"type": "string", "enum": []string{"skip", "allow"}},
			"expires_at":                 map[string]any{"type": "string", "description": "设置新的任务生命周期截止时间，RFC3339"},
			"clear_expires_at":           map[string]any{"type": "boolean", "description": "清除任务生命周期截止时间；不要与 expires_at 同时使用"},
			"selected_session_key":       map[string]any{"type": "string"},
			"named_session_key":          map[string]any{"type": "string"},
			"selected_reply_session_key": map[string]any{"type": "string"},
			"reply_agent_id":             map[string]any{"type": "string"},
			"reply_session_key":          map[string]any{"type": "string"},
			"reply_channel":              map[string]any{"type": "string"},
			"reply_to":                   map[string]any{"type": "string"},
			"reply_account_id":           map[string]any{"type": "string"},
			"reply_thread_id":            map[string]any{"type": "string"},
			"enabled":                    map[string]any{"type": "boolean"},
		},
	}
}

func jobIDSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"job_id":   map[string]any{"type": "string", "description": "任务 id；也可改传 query 让工具在当前权限范围内定位唯一当前未删除任务"},
			"query":    map[string]any{"type": "string", "description": "可选。没有 job_id 时按名称、内容、投递目标或状态定位唯一当前未删除任务；当前 DM/Room/IM 群里会优先当前会话匹配，写“这里/当前会话/这个群/当前频道”会强制限定；多候选时不会执行"},
			"agent_id": map[string]any{"type": "string", "description": "主智能体可填：把 query 限定到某个智能体；普通 agent 会被强制限定为自己"},
		},
	}
}

func taskHistoryJobIDSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"job_id":   map[string]any{"type": "string", "description": "任务 id；也可改传 query 定位唯一当前或已删除任务"},
			"query":    map[string]any{"type": "string", "description": "可选。没有 job_id 时按名称、内容、投递目标或审计 detail 定位唯一当前或已删除任务；当前 DM/Room/IM 群里会优先当前会话匹配，写“这里/当前会话/这个群/当前频道”会强制限定"},
			"agent_id": map[string]any{"type": "string", "description": "主智能体可填：把 query 限定到某个智能体；普通 agent 会被强制限定为自己"},
		},
	}
}

func disableSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"job_id":   map[string]any{"type": "string", "description": "任务 id；也可改传 query 让工具在当前权限范围内定位唯一当前未删除任务"},
			"query":    map[string]any{"type": "string", "description": "可选。没有 job_id 时按名称、内容、投递目标或状态定位唯一当前未删除任务；当前 DM/Room/IM 群里会优先当前会话匹配，写“这里/当前会话/这个群/当前频道”会强制限定；多候选时不会停用"},
			"agent_id": map[string]any{"type": "string", "description": "主智能体可填：把 query 限定到某个智能体；普通 agent 会被强制限定为自己"},
			"cancel_active_run": map[string]any{
				"type":        "boolean",
				"description": "可选。true 表示停用后同时中断并取消当前 running_run_id；false 只阻止后续触发。",
			},
			"run_id": map[string]any{
				"type":        "string",
				"description": "可选。配合 cancel_active_run 使用，传入当前 running_run_id 可避免误取消刷新前看到的旧 run。",
			},
		},
	}
}

func dailyReportSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"date":     map[string]any{"type": "string", "description": "要查询的日期，YYYY-MM-DD；缺省=today。也接受 today / 今天"},
			"timezone": map[string]any{"type": "string", "description": "IANA 时区，如 Asia/Shanghai；缺省使用当前上下文默认时区"},
			"agent_id": map[string]any{"type": "string", "description": "主智能体可填：只看某个智能体的任务"},
			"job_id":   map[string]any{"type": "string", "description": "可选：只看某个任务"},
			"query":    map[string]any{"type": "string", "description": "可选：没有 job_id 时按自然语言定位唯一当前或已删除任务，再只看该任务；当前 DM/Room/IM 群里会优先当前会话匹配，写“这里/当前会话/这个群/当前频道”会强制限定；泛化的“当前会话/这个群定时任务发送情况”会聚合当前会话任务"},
		},
	}
}

func recoverSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"job_id": map[string]any{"type": "string", "description": "任务 id；也可改传 query 让工具定位唯一当前未删除任务"},
			"query":  map[string]any{"type": "string", "description": "可选。没有 job_id 时按名称、内容、投递目标或状态定位唯一当前未删除任务；当前 DM/Room/IM 群里会优先当前会话匹配，写“这里/当前会话/这个群/当前频道”会强制限定；多候选时不会恢复"},
			"agent_id": map[string]any{
				"type":        "string",
				"description": "主智能体可填：把 query 限定到某个智能体；普通 agent 会被强制限定为自己",
			},
			"run_id": map[string]any{
				"type":        "string",
				"description": "可选。传入当前 running_run_id 可避免误释放刷新前看到的旧 run。",
			},
		},
	}
}

func runIDSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"job_id":   map[string]any{"type": "string", "description": "任务 id；也可改传 query 让工具定位唯一当前未删除任务"},
			"query":    map[string]any{"type": "string", "description": "可选。没有 job_id 时按名称、内容、投递目标或状态定位唯一当前未删除任务；当前 DM/Room/IM 群里会优先当前会话匹配，写“这里/当前会话/这个群/当前频道”会强制限定"},
			"agent_id": map[string]any{"type": "string", "description": "主智能体可填：把 query 限定到某个智能体；普通 agent 会被强制限定为自己"},
			"run_id":   map[string]any{"type": "string", "description": "可选。要补投递的失败 run；不传时会自动选择唯一可手动补投递的失败 run，多候选会要求确认"},
		},
	}
}

func taskEventsSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"job_id":   map[string]any{"type": "string", "description": "任务 id；也可改传 query 定位唯一当前或已删除任务"},
			"query":    map[string]any{"type": "string", "description": "可选。没有 job_id 时按名称、内容、投递目标或审计 detail 定位唯一当前或已删除任务；当前 DM/Room/IM 群里会优先当前会话匹配，写“这里/当前会话/这个群/当前频道”会强制限定"},
			"agent_id": map[string]any{"type": "string", "description": "主智能体可填：把 query 限定到某个智能体；普通 agent 会被强制限定为自己"},
			"limit":    map[string]any{"type": "integer", "description": "返回条数，缺省 50，最大 100"},
		},
	}
}

func taskStatusSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"job_id":      map[string]any{"type": "string", "description": "任务 id；也可改传 query 让工具在当前权限范围内定位唯一当前未删除任务"},
			"query":       map[string]any{"type": "string", "description": "可选。没有 job_id 时按名称、内容、投递目标或状态定位唯一当前未删除任务；当前 DM/Room/IM 群里会优先当前会话匹配，写“这里/当前会话/这个群/当前频道”会强制限定；多候选时不会继续查询"},
			"agent_id":    map[string]any{"type": "string", "description": "主智能体可填：把 query 限定到某个智能体；普通 agent 会被强制限定为自己"},
			"run_limit":   map[string]any{"type": "integer", "description": "recent_runs 返回条数，缺省 10，最大 50"},
			"event_limit": map[string]any{"type": "integer", "description": "recent_events 返回条数，缺省 10，最大 50"},
		},
	}
}
