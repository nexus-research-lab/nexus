package semantic

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/argx"
)

// flatScheduleKeys 列出可能被 LLM 平铺到顶层的 schedule 字段。
// 部分模型（典型如 Grok / 国产模型）不喜欢嵌套对象，会把这些键直接放到 args 顶层。
// 这里参考 OpenClaw 的 flat-params recovery 思路（cron-tool.ts:293-344），
// 当 args.schedule 缺失或为空时，自动把这些字段重新组装成嵌套 schedule 对象。
var flatScheduleKeys = []string{
	"kind", "timezone",
	"run_at", "at",
	"daily_time", "weekdays",
	"interval_value", "interval_unit",
	"expr", "cron", "cron_expression",
}

// ReassembleFlatSchedule 检测顶层平铺的 schedule 字段，缺失时回补成 schedule 对象。
// 已经显式传 args["schedule"] 的请求也会合并缺失字段，兼容模型把一部分参数写成顶层或 schedule.xxx。
func ReassembleFlatSchedule(args map[string]any) {
	if args == nil {
		return
	}
	schedule, hasSchedule := args["schedule"].(map[string]any)
	if !hasSchedule || schedule == nil {
		schedule = map[string]any{}
	}
	hasSignal := false
	for _, key := range flatScheduleKeys {
		value, exists := firstScheduleAliasValue(args, key)
		if !exists || value == nil {
			continue
		}
		targetKey := normalizeScheduleKey(key)
		if _, exists = schedule[targetKey]; !exists {
			schedule[targetKey] = value
		}
		if key != "kind" && key != "timezone" {
			hasSignal = true
		}
	}
	if !hasSchedule && !hasSignal {
		return
	}
	args["schedule"] = schedule
}

func firstScheduleAliasValue(args map[string]any, key string) (any, bool) {
	if value, exists := args[key]; exists {
		return value, true
	}
	if value, exists := args["schedule."+key]; exists {
		return value, true
	}
	return nil, false
}

func normalizeScheduleKey(key string) string {
	switch key {
	case "at":
		return "run_at"
	case "cron", "cron_expression":
		return "expr"
	default:
		return key
	}
}

// ApplyDefaultTimezone 如果 schedule.timezone 缺失，写入 sctx.DefaultTimezone（兜底 Asia/Shanghai）。
func ApplyDefaultTimezone(args map[string]any, sctx contract.ServerContext) {
	schedule, ok := args["schedule"].(map[string]any)
	if !ok {
		return
	}
	if strings.TrimSpace(argx.String(schedule, "timezone")) != "" {
		return
	}
	tz := strings.TrimSpace(sctx.DefaultTimezone)
	if tz == "" {
		tz = "Asia/Shanghai"
	}
	schedule["timezone"] = tz
}

// ApplyConversationDefaults 按对话上下文补齐安全默认值。
// 首先把面向模型的高层意图翻译为页面兼容字段，再让可信当前会话绑定真实投递目标。
func ApplyConversationDefaults(args map[string]any, sctx contract.ServerContext) map[string]any {
	args = ApplyIntentDefaults(args, sctx)
	args = ApplyDeliveryFieldDefaults(args)
	args = ApplyCurrentChannelDefaults(args, sctx)
	args = ApplyVisibleResultDefaults(args, sctx)
	args = ApplySafeConversationFallback(args, sctx)
	return BindCurrentExternalChannelReply(args, sctx)
}

// ApplyIntentDefaults 把普通 Agent 需要表达的两个产品意图翻译为旧页面字段。
// context_mode 只决定是否复用当前上下文；deliver_result 只决定是否回到当前会话。
// channel/account/chat/thread/session 始终来自 ServerContext，模型无法覆盖。
func ApplyIntentDefaults(args map[string]any, sctx contract.ServerContext) map[string]any {
	if args == nil {
		return args
	}
	contextMode := strings.TrimSpace(argx.String(args, "context_mode"))
	if strings.TrimSpace(argx.String(args, "execution_mode")) == "" {
		switch contextMode {
		case "current":
			args["execution_mode"] = "existing"
		case "isolated":
			args["execution_mode"] = "temporary"
		}
	}
	if _, exists := args["deliver_result"]; !exists || strings.TrimSpace(argx.String(args, "reply_mode")) != "" {
		return args
	}
	if !argx.ParseBool(args["deliver_result"]) {
		args["reply_mode"] = "none"
		return args
	}
	bindCurrentConversationDelivery(args, sctx)
	return args
}

// ApplySafeConversationFallback 让常规创建不再要求模型理解底层 session/delivery 枚举。
// 有当前会话时默认独立执行并把结果送回当前会话；明确依赖当前聊天历史时复用当前会话。
// 没有可信当前会话时默认独立执行、仅保存在运行记录中。
func ApplySafeConversationFallback(args map[string]any, sctx contract.ServerContext) map[string]any {
	if args == nil || strings.TrimSpace(argx.String(args, "execution_kind")) == "script" || !hasRunnableScheduleShape(args) {
		return args
	}
	executionMode := strings.TrimSpace(argx.String(args, "execution_mode"))
	if executionMode == "" {
		if strings.TrimSpace(sctx.CurrentSessionKey) != "" &&
			containsAnyKeyword(defaultIntentText(args), currentConversationDependencyKeywords) {
			executionMode = "existing"
		} else {
			executionMode = "temporary"
		}
		args["execution_mode"] = executionMode
	}
	if strings.TrimSpace(argx.String(args, "reply_mode")) != "" {
		return args
	}
	if strings.TrimSpace(sctx.CurrentSessionKey) == "" {
		args["reply_mode"] = "none"
		return args
	}
	if containsAnyKeyword(defaultIntentText(args), append(visibleResultOptOutKeywords, currentChannelDeliveryOptOutKeywords...)) {
		args["reply_mode"] = "none"
		return args
	}
	bindCurrentConversationDelivery(args, sctx)
	return args
}

func bindCurrentConversationDelivery(args map[string]any, sctx contract.ServerContext) {
	if currentSessionKeyCanDeliverToExternalChannel(sctx.CurrentSessionKey) {
		args["reply_mode"] = "channel"
		return
	}
	if strings.TrimSpace(argx.String(args, "execution_mode")) == "existing" {
		args["reply_mode"] = "execution"
		return
	}
	args["reply_mode"] = "selected"
	args["selected_reply_session_key"] = sctx.CurrentSessionKey
}

func hasRunnableScheduleShape(args map[string]any) bool {
	schedule, ok := args["schedule"].(map[string]any)
	if !ok {
		return false
	}
	kind := strings.TrimSpace(argx.String(schedule, "kind"))
	switch kind {
	case "interval":
		return argx.Int(schedule["interval_value"]) > 0
	case "daily":
		return strings.TrimSpace(argx.String(schedule, "daily_time")) != ""
	case "single":
		return strings.TrimSpace(argx.String(schedule, "run_at")) != ""
	case "cron":
		return strings.TrimSpace(argx.FirstNonEmpty(argx.String(schedule, "expr"), argx.String(schedule, "cron"))) != ""
	default:
		return false
	}
}

// RequireExplicitCreateFields 验证默认值归一化已产生完整的底层页面语义。
// 模型不需要自己提供 execution_mode/reply_mode；它们缺失表示宿主默认值链存在缺口。
func RequireExplicitCreateFields(args map[string]any, sctx contract.ServerContext) error {
	if _, ok := args["schedule"].(map[string]any); !ok {
		return missingFieldsError([]string{"schedule"})
	}
	if strings.TrimSpace(argx.String(args, "execution_kind")) == "script" {
		return nil
	}
	missing := []string{}
	if argx.String(args, "execution_mode") == "" {
		missing = append(missing, "execution_mode")
	}
	if argx.String(args, "reply_mode") == "" {
		missing = append(missing, "reply_mode")
	}
	if len(missing) > 0 {
		return missingFieldsError(missing)
	}
	return nil
}

func missingFieldsError(missing []string) error {
	return &requiredFieldError{Missing: missing}
}

type requiredFieldError struct {
	Missing []string
}

func (e *requiredFieldError) Error() string {
	return "missing normalized scheduling fields: " + strings.Join(e.Missing, ", ")
}
