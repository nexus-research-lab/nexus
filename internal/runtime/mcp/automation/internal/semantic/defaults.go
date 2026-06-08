package semantic

import (
	"strings"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/internal/argx"
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

// contextHeavyKeywords 命中后强制要求显式确认 execution/reply 字段，禁止套默认值。
// 中英双语：避免英文 instruction（"summary of today"）绕过判断走默认。
var contextHeavyKeywords = []string{
	"总结", "汇总", "简报", "报告", "跟进", "复盘", "检查", "分析", "研究", "整理", "回顾", "监控",
	"搜索", "检索", "新闻", "日报", "周报", "月报", "告警", "报警", "推送", "发送", "发到", "投递", "播报",
	"summary", "summarize", "summarise", "report", "review", "analyze", "analyse",
	"analysis", "follow up", "follow-up", "followup", "audit", "investigate",
	"monitor", "digest", "recap", "retrospective", "search", "news", "newsletter",
	"alert", "push", "send", "deliver", "broadcast",
}

func containsHeavyKeyword(instruction string) bool {
	lower := strings.ToLower(instruction)
	for _, keyword := range contextHeavyKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

// CanDefaultSimpleReminder 判断是否允许在 create 时套用短提醒默认值。
// 仅当前有会话、短文本、无重业务关键词、调度形状合法的提醒类任务允许默认。
func CanDefaultSimpleReminder(args map[string]any, sctx contract.ServerContext) bool {
	if strings.TrimSpace(sctx.CurrentSessionKey) == "" {
		return false
	}
	instruction := strings.TrimSpace(argx.String(args, "instruction"))
	if instruction == "" || utf8.RuneCountInString(instruction) > 24 {
		return false
	}
	if containsHeavyKeyword(instruction) {
		return false
	}
	return hasRunnableScheduleShape(args)
}

// ApplyConversationDefaults 按对话上下文补齐安全默认值。
// 优先处理“发到当前 IM/飞书群”的明确投递诉求，再处理复杂任务可见回传，最后处理普通短提醒。
func ApplyConversationDefaults(args map[string]any, sctx contract.ServerContext) map[string]any {
	args = ApplyDeliveryFieldDefaults(args)
	args = ApplyCurrentChannelDefaults(args, sctx)
	args = ApplyVisibleResultDefaults(args, sctx)
	return ApplySimpleDefaults(args, sctx)
}

// ApplySimpleDefaults 在允许的前提下补齐 execution_mode / reply_mode 默认值。
// 默认对齐 UI 的当前会话可见语义：existing + execution。
// 如果模型显式要求 temporary/dedicated 但没写 reply_mode，则把结果回传到当前会话。
func ApplySimpleDefaults(args map[string]any, sctx contract.ServerContext) map[string]any {
	if !CanDefaultSimpleReminder(args, sctx) {
		return args
	}
	if argx.String(args, "execution_mode") == "" {
		args["execution_mode"] = "existing"
	}
	if argx.String(args, "reply_mode") == "" {
		switch strings.TrimSpace(argx.String(args, "execution_mode")) {
		case "main":
			args["reply_mode"] = "none"
		case "temporary", "dedicated":
			args["reply_mode"] = "selected"
			if argx.String(args, "selected_reply_session_key") == "" {
				args["selected_reply_session_key"] = sctx.CurrentSessionKey
			}
		default:
			args["reply_mode"] = "execution"
		}
	}
	if strings.TrimSpace(argx.String(args, "reply_mode")) == "selected" &&
		argx.String(args, "selected_reply_session_key") == "" {
		args["selected_reply_session_key"] = sctx.CurrentSessionKey
	}
	return args
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

// RequireExplicitCreateFields 在不允许默认时强制要求 execution_mode / reply_mode 字段齐全。
// 注意：schedule.timezone 现在由 ApplyDefaultTimezone 自动补齐，这里不再强求。
func RequireExplicitCreateFields(args map[string]any, sctx contract.ServerContext) error {
	if _, ok := args["schedule"].(map[string]any); !ok {
		return missingFieldsError([]string{"schedule"})
	}
	if strings.TrimSpace(argx.String(args, "execution_kind")) == "script" {
		return nil
	}
	if CanDefaultSimpleReminder(args, sctx) {
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
	return "missing required scheduling fields: " + strings.Join(e.Missing, ", ") +
		". Ask the user in the current conversation to confirm these fields, " +
		"or, if the request already says the result should be sent/told back here and does not depend on current chat history, retry with execution_mode=temporary + reply_mode=selected. " +
		"or shorten the instruction to a short reminder (≤24 chars / 24 字) without heavy-context keywords " +
		"(summary / report / analyze / 总结 / 汇总 / 分析 …) from an active chat to qualify for the default visible reminder mode."
}
