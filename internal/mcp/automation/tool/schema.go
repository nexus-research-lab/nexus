package tool

import (
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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

// executionModeSchema 仅用于 owner main 的兼容高级契约；普通对话使用 context_mode。
var executionModeSchema = map[string]any{
	"type":        "string",
	"enum":        []string{"main", "existing", "temporary", "dedicated"},
	"description": "main=使用主会话 / existing=使用现有会话 / temporary=每次新建临时会话 / dedicated=使用专用长期会话",
}

// replyModeSchema 对齐 UI「结果回传」按钮，并按可信调用上下文收窄模型可选项。
func replyModeSchema(sctx contract.ServerContext) map[string]any {
	modes := []string{"none", "execution", "selected"}
	description := "none=不回传 / execution=回到执行会话 / selected=回到指定真实会话"
	if channel, chatType, ok := currentExternalIMSummary(sctx); ok {
		modes = append(modes, "channel")
		description += fmt.Sprintf(
			" / channel=回到当前已授权的 %s %s；Nexus 自动绑定真实通道路由，不要填写 session key、账号或目标 ID",
			channel,
			chatType,
		)
	} else if hasMainAgentSchema(sctx) {
		modes = append(modes, "channel")
		description += " / channel=投递到显式 IM/外部通道目标"
	}
	return map[string]any{
		"type":        "string",
		"enum":        modes,
		"description": description,
	}
}

func createSchema(sctx contract.ServerContext) map[string]any {
	properties := map[string]any{
		"request_id":      map[string]any{"type": "string", "description": "本次创建意图的稳定幂等键；同一工具调用重试必须复用，不能换值"},
		"name":            map[string]any{"type": "string", "description": "任务名称"},
		"agent_id":        map[string]any{"type": "string", "description": "目标智能体；缺省=当前智能体。普通 Agent/Room 成员只能创建自身任务，owner main 仅在自己的可信私有 DM 可指定其他 Agent"},
		"instruction":     map[string]any{"type": "string", "description": "任务指令（Agent 到点要执行的内容）"},
		"execution_kind":  map[string]any{"type": "string", "enum": []string{"agent"}, "description": "对话入口只允许交给 Agent 会话执行；宿主脚本任务是人类控制面能力"},
		"permission_mode": map[string]any{"type": "string", "enum": []string{"default", "plan", "acceptEdits", "bypassPermissions", "dontAsk"}, "description": "任务每次运行使用的 SDK 权限模式；省略时在创建瞬间复制当前 Session/Agent 的有效模式与工具权限，之后独立保存。bypassPermissions 会明确跳过 SDK 权限检查；SDK 发出的额外权限请求由 Nexus 持久审批"},
		"schedule":        scheduleSchema,
		"context_mode":    map[string]any{"type": "string", "enum": []string{"isolated", "current"}, "description": "可选。isolated=每次独立运行（默认）；current=任务需要读取当前聊天历史时复用当前会话"},
		"deliver_result":  map[string]any{"type": "boolean", "description": "可选。是否把结果送回当前可信会话；有当前会话时默认 true，无当前会话时默认 false。外部 IM 的真实路由由 Nexus 自动绑定"},
		"overlap_policy":  map[string]any{"type": "string", "enum": []string{"skip", "allow"}, "description": "重叠触发策略：skip=有运行中任务时跳过；allow=允许并发执行。缺省 skip"},
		"expires_at":      map[string]any{"type": "string", "description": "可选。任务生命周期截止时间，RFC3339；到期后停止后续触发，但不中断正在执行的任务"},
		"enabled":         map[string]any{"type": "boolean", "description": "创建后立即启用，缺省 true"},
	}
	addAdvancedRoutingSchema(properties, sctx)
	return map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   []string{"request_id", "name", "instruction", "schedule"},
	}
}

func addAdvancedRoutingSchema(properties map[string]any, sctx contract.ServerContext) {
	if properties == nil || !hasMainAgentSchema(sctx) {
		return
	}
	properties["execution_mode"] = executionModeSchema
	properties["reply_mode"] = replyModeSchema(sctx)
	properties["selected_session_key"] = map[string]any{"type": "string", "description": "高级兼容：复用的会话 key"}
	properties["named_session_key"] = map[string]any{"type": "string", "description": "高级兼容：专用长期会话名称"}
	properties["selected_reply_session_key"] = map[string]any{"type": "string", "description": "高级兼容：接收结果的会话 key"}
	addExplicitChannelTargetSchema(properties, sctx)
}

func addExplicitChannelTargetSchema(properties map[string]any, sctx contract.ServerContext) {
	if properties != nil && hasMainAgentSchema(sctx) {
		properties["reply_session_key"] = map[string]any{"type": "string", "description": "reply_mode=channel 时填写已存在、结构化且已授权的 IM/session key"}
	}
}

func hasMainAgentSchema(sctx contract.ServerContext) bool {
	return sctx.IsMainAgent &&
		(sctx.StableInteractiveSurface || strings.TrimSpace(sctx.SourceContextType) == "agent")
}

func currentExternalIMSummary(sctx contract.ServerContext) (string, string, bool) {
	parsed := protocol.ParseSessionKey(strings.TrimSpace(sctx.CurrentSessionKey))
	if !parsed.IsStructured || parsed.Kind != protocol.SessionKeyKindAgent || strings.TrimSpace(parsed.Ref) == "" {
		return "", "", false
	}
	channel := protocol.NormalizeStoredChannelType(parsed.Channel)
	switch channel {
	case protocol.SessionChannelDiscord,
		protocol.SessionChannelTelegram,
		protocol.SessionChannelDingTalk,
		protocol.SessionChannelWeChat,
		protocol.SessionChannelWeixinPersonal,
		protocol.SessionChannelFeishu:
	default:
		return "", "", false
	}
	chatType := protocol.NormalizeSessionChatType(parsed.ChatType)
	if chatType == protocol.RoomTypeDM {
		chatType = "私聊"
	} else if chatType == protocol.RoomTypeGroup {
		chatType = "群聊"
	}
	return channel, chatType, true
}

func heartbeatGetSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"agent_id": map[string]any{"type": "string", "description": "主智能体仅在自己的可信 Nexus 私有 DM 可指定 owner scope 内 Agent；Room、外部与后台来源只能读取当前 Agent"},
		},
	}
}

func heartbeatUpdateSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"agent_id":      map[string]any{"type": "string", "description": "owner main 仅在自己的可信私有 DM 可指定目标 Agent；其他上下文只能修改自身"},
			"enabled":       map[string]any{"type": "boolean"},
			"every_seconds": map[string]any{"type": "integer", "minimum": 1},
			"target_mode":   map[string]any{"type": "string", "enum": []string{"none", "last"}},
			"ack_max_chars": map[string]any{"type": "integer", "minimum": 0},
		},
	}
}

func heartbeatWakeSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"agent_id": map[string]any{"type": "string", "description": "owner main 仅在自己的可信私有 DM 可指定目标 Agent；其他上下文只能唤醒自身"},
			"mode":     map[string]any{"type": "string", "enum": []string{"now", "next-heartbeat"}, "description": "缺省 now"},
			"text":     map[string]any{"type": "string", "description": "可选，随本次唤醒交付的上下文"},
		},
	}
}

func updateSchema(sctx contract.ServerContext) map[string]any {
	properties := map[string]any{
		"job_id":             map[string]any{"type": "string", "description": "要修改的任务 id；也可改传 query 让工具在当前权限范围内定位唯一任务"},
		"query":              map[string]any{"type": "string", "description": "可选。没有 job_id 时按名称、内容、投递目标或状态定位唯一当前未删除任务；当前 DM/Room/IM 群里会优先当前会话匹配，写“这里/当前会话/这个群/当前频道”会强制限定；多候选时不会修改"},
		"agent_id":           map[string]any{"type": "string", "description": "owner main 仅在自己的可信私有 DM 可把 query 限定到其他 Agent；其余上下文强制限定为自己"},
		"name":               map[string]any{"type": "string"},
		"instruction":        map[string]any{"type": "string", "description": "完整替换任务内容；用户只是说“再加一条要求/补充细节”时优先用 instruction_append"},
		"instruction_append": map[string]any{"type": "string", "description": "追加到当前任务内容末尾，适合“再加上/补充/以后也要”这类增量修改；不要和 instruction 同时传"},
		"execution_kind":     map[string]any{"type": "string", "enum": []string{"agent"}, "description": "对话入口不能把任务切换为宿主脚本"},
		"permission_mode":    map[string]any{"type": "string", "enum": []string{"default", "plan", "acceptEdits", "bypassPermissions", "dontAsk"}, "description": "后续每次运行使用的 SDK 权限模式；bypassPermissions 会明确跳过 SDK 权限检查"},
		"schedule":           scheduleSchema,
		"context_mode":       map[string]any{"type": "string", "enum": []string{"isolated", "current"}, "description": "修改运行上下文：isolated=以后独立运行；current=以后复用当前会话"},
		"deliver_result":     map[string]any{"type": "boolean", "description": "修改是否把结果送回当前可信会话；外部 IM 的真实路由由 Nexus 自动绑定"},
		"overlap_policy":     map[string]any{"type": "string", "enum": []string{"skip", "allow"}},
		"expires_at":         map[string]any{"type": "string", "description": "设置新的任务生命周期截止时间，RFC3339"},
		"clear_expires_at":   map[string]any{"type": "boolean", "description": "清除任务生命周期截止时间；不要与 expires_at 同时使用"},
		"enabled":            map[string]any{"type": "boolean"},
		"cancel_active_run":  map[string]any{"type": "boolean", "description": "停用任务时是否同时中断当前 active run；true 会隐含 enabled=false"},
		"run_id":             map[string]any{"type": "string", "description": "配合 cancel_active_run 使用；传当前 running_run_id 可避免误取消旧 run"},
	}
	addAdvancedRoutingSchema(properties, sctx)
	return map[string]any{
		"type":       "object",
		"properties": properties,
	}
}

func jobIDSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"job_id":   map[string]any{"type": "string", "description": "任务 id；也可改传 query 让工具在当前权限范围内定位唯一当前未删除任务"},
			"query":    map[string]any{"type": "string", "description": "可选。没有 job_id 时按名称、内容、投递目标或状态定位唯一当前未删除任务；当前 DM/Room/IM 群里会优先当前会话匹配，写“这里/当前会话/这个群/当前频道”会强制限定；多候选时不会执行"},
			"agent_id": map[string]any{"type": "string", "description": "owner main 仅在自己的可信私有 DM 可把 query 限定到其他 Agent；其余上下文强制限定为自己"},
		},
	}
}

func findSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query":           map[string]any{"type": "string", "description": "按任务 id、名称、内容、投递目标、来源、状态或审计内容查询；当前会话中的查询优先匹配当前会话任务"},
			"agent_id":        map[string]any{"type": "string", "description": "owner main 仅在自己的可信私有 DM 可限定其他 Agent；其余上下文始终限定为自己"},
			"include_active":  map[string]any{"type": "boolean", "description": "是否包含当前任务，缺省 true"},
			"include_deleted": map[string]any{"type": "boolean", "description": "是否包含已删除任务，缺省 false"},
			"enabled":         map[string]any{"type": "boolean", "description": "可选，只返回匹配启用状态的当前任务；已删除任务会被排除"},
			"limit":           map[string]any{"type": "integer", "description": "返回条数，缺省 20，最大 50"},
		},
	}
}

func inspectSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"job_id":      map[string]any{"type": "string", "description": "任务 id；后台定时运行中省略时自动使用宿主绑定的当前任务，也可改传 query 定位唯一任务"},
			"query":       map[string]any{"type": "string", "description": "按名称、内容、投递目标或状态定位唯一任务；runs/events 可检查已删除任务"},
			"agent_id":    map[string]any{"type": "string", "description": "owner main 仅在自己的可信私有 DM 可限定其他 Agent；其余上下文始终限定为自己"},
			"view":        map[string]any{"type": "string", "enum": []string{"status", "runs", "events"}, "description": "status=配置与健康摘要；runs=运行历史；events=管理审计。缺省 status"},
			"run_limit":   map[string]any{"type": "integer", "description": "status/runs 返回条数，缺省 10，最大 50"},
			"event_limit": map[string]any{"type": "integer", "description": "status/events 返回条数，缺省 10，最大 50"},
		},
	}
}

func reportSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"date":     map[string]any{"type": "string", "description": "要查询的日期，YYYY-MM-DD；缺省=today。也接受 today / 今天"},
			"timezone": map[string]any{"type": "string", "description": "IANA 时区，如 Asia/Shanghai；缺省使用当前上下文默认时区"},
			"agent_id": map[string]any{"type": "string", "description": "owner main 仅在自己的可信私有 DM 可查看其他 Agent；其余上下文只能查看自身"},
			"job_id":   map[string]any{"type": "string", "description": "可选：只看某个任务；后台定时运行中省略时自动使用宿主绑定的当前任务"},
			"query":    map[string]any{"type": "string", "description": "可选：没有 job_id 时按自然语言定位唯一当前或已删除任务，再只看该任务；当前 DM/Room/IM 群里会优先当前会话匹配，写“这里/当前会话/这个群/当前频道”会强制限定；泛化的“当前会话/这个群定时任务发送情况”会聚合当前会话任务"},
		},
	}
}

func repairSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action":   map[string]any{"type": "string", "enum": []string{"recover", "retry_delivery"}, "description": "recover=释放卡住的执行；retry_delivery=只补发已完成 run 的失败投递"},
			"job_id":   map[string]any{"type": "string", "description": "任务 id；也可改传 query 定位唯一当前任务"},
			"query":    map[string]any{"type": "string", "description": "按名称、内容、投递目标或状态定位唯一当前任务"},
			"agent_id": map[string]any{"type": "string", "description": "owner main 仅在自己的可信私有 DM 可限定其他 Agent；其余上下文始终限定为自己"},
			"run_id":   map[string]any{"type": "string", "description": "可选。recover 时用于避免误释放旧 run；retry_delivery 时指定要补投递的失败 run"},
		},
		"required": []string{"action"},
	}
}
