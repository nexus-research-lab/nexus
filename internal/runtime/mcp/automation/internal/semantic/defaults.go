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
// 已经显式传 args["schedule"] 的请求不动。
func ReassembleFlatSchedule(args map[string]any) {
	if args == nil {
		return
	}
	if existing, ok := args["schedule"].(map[string]any); ok && len(existing) > 0 {
		return
	}
	synthetic := map[string]any{}
	hasSignal := false
	for _, key := range flatScheduleKeys {
		value, exists := args[key]
		if !exists || value == nil {
			continue
		}
		switch key {
		case "at":
			synthetic["run_at"] = value
		case "cron", "cron_expression":
			synthetic["expr"] = value
		default:
			synthetic[key] = value
		}
		if key != "kind" && key != "timezone" {
			hasSignal = true
		}
	}
	if !hasSignal {
		return
	}
	args["schedule"] = synthetic
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
	"summary", "summarize", "summarise", "report", "review", "analyze", "analyse",
	"analysis", "follow up", "follow-up", "followup", "audit", "investigate",
	"monitor", "digest", "recap", "retrospective",
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

// CanDefaultToTemporaryNone 判断是否允许在 create 时默认按 temporary + none 创建。
// 仅短文本、无重业务关键词、调度形状合法的提醒类任务允许默认。
func CanDefaultToTemporaryNone(args map[string]any) bool {
	instruction := strings.TrimSpace(argx.String(args, "instruction"))
	if instruction == "" || utf8.RuneCountInString(instruction) > 24 {
		return false
	}
	if containsHeavyKeyword(instruction) {
		return false
	}
	schedule, ok := args["schedule"].(map[string]any)
	if !ok {
		return false
	}
	kind := strings.TrimSpace(argx.String(schedule, "kind"))
	switch kind {
	case "interval":
		if argx.Int(schedule["interval_value"]) <= 0 {
			return false
		}
	case "daily":
		if strings.TrimSpace(argx.String(schedule, "daily_time")) == "" {
			return false
		}
	case "single":
		if strings.TrimSpace(argx.String(schedule, "run_at")) == "" {
			return false
		}
	case "cron":
		if strings.TrimSpace(argx.FirstNonEmpty(argx.String(schedule, "expr"), argx.String(schedule, "cron"))) == "" {
			return false
		}
	default:
		return false
	}
	return true
}

// ApplySimpleDefaults 在允许的前提下补齐 execution_mode / reply_mode 默认值。
// 默认走 temporary + none：
//   - reply_mode=execution 在 agent 上下文下会被 executionReply 强制改写为 none，
//     与其让 agent 误以为"会回到我这"，不如直接默认 none，语义诚实。
//   - 想让结果回到当前会话，agent 必须显式 execution_mode=existing + reply_mode=execution。
func ApplySimpleDefaults(args map[string]any) map[string]any {
	if !CanDefaultToTemporaryNone(args) {
		return args
	}
	if argx.String(args, "execution_mode") == "" {
		args["execution_mode"] = "temporary"
	}
	if argx.String(args, "reply_mode") == "" {
		args["reply_mode"] = "none"
	}
	return args
}

// RequireExplicitCreateFields 在不允许默认时强制要求 execution_mode / reply_mode 字段齐全。
// 注意：schedule.timezone 现在由 ApplyDefaultTimezone 自动补齐，这里不再强求。
func RequireExplicitCreateFields(args map[string]any) error {
	if _, ok := args["schedule"].(map[string]any); !ok {
		return missingFieldsError([]string{"schedule"})
	}
	if CanDefaultToTemporaryNone(args) {
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
		". Either ask the user to confirm these fields (e.g. via AskUserQuestion), " +
		"or shorten the instruction to a short reminder (≤24 chars / 24 字) without heavy-context keywords " +
		"(summary / report / analyze / 总结 / 汇总 / 分析 …) to qualify for the default temporary+none mode."
}
