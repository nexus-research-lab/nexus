package semantic

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/automation/mcp/internal/argx"
)

// contextHeavyKeywords 命中后强制要求显式确认 execution/reply 字段，禁止套默认值。
var contextHeavyKeywords = []string{
	"总结", "汇总", "简报", "报告", "跟进", "复盘", "检查", "分析", "研究", "整理", "回顾", "监控",
}

// CanDefaultToTemporaryNone 判断是否允许在 create 时默认按 temporary + none 创建。
// 仅短文本、无重业务关键词、调度形状合法的提醒类任务允许默认。
func CanDefaultToTemporaryNone(args map[string]any) bool {
	instruction := strings.TrimSpace(argx.String(args, "instruction"))
	if instruction == "" || utf8.RuneCountInString(instruction) > 24 {
		return false
	}
	for _, keyword := range contextHeavyKeywords {
		if strings.Contains(instruction, keyword) {
			return false
		}
	}
	schedule, ok := args["schedule"].(map[string]any)
	if !ok {
		return false
	}
	kind := strings.TrimSpace(argx.String(schedule, "kind"))
	switch kind {
	case "every":
		if argx.Int(schedule["interval_seconds"]) <= 0 {
			return false
		}
	case "cron":
		if strings.TrimSpace(argx.String(schedule, "cron_expression")) == "" {
			return false
		}
	case "at":
		if strings.TrimSpace(argx.String(schedule, "run_at")) == "" {
			return false
		}
	default:
		return false
	}
	return true
}

// ApplySimpleDefaults 在允许的前提下补齐 execution_mode / reply_mode 默认值。
func ApplySimpleDefaults(args map[string]any) map[string]any {
	if !CanDefaultToTemporaryNone(args) {
		return args
	}
	hasSessionTarget := argx.HasObject(args, "session_target") || argx.String(args, "execution_mode") != ""
	hasDelivery := argx.HasObject(args, "delivery") || argx.String(args, "reply_mode") != ""
	if !hasSessionTarget {
		args["execution_mode"] = "temporary"
	}
	if !hasDelivery {
		args["reply_mode"] = "none"
	}
	return args
}

// RequireExplicitCreateFields 在不允许默认时强制要求 schedule.timezone 与 session_target / delivery 字段齐全。
func RequireExplicitCreateFields(args map[string]any) error {
	schedule, ok := args["schedule"].(map[string]any)
	if !ok {
		return errors.New("schedule is required")
	}
	missing := []string{}
	allowSimple := CanDefaultToTemporaryNone(args)
	if !allowSimple {
		if !argx.HasObject(args, "session_target") && argx.String(args, "execution_mode") == "" {
			missing = append(missing, "session_target")
		}
		if !argx.HasObject(args, "delivery") && argx.String(args, "reply_mode") == "" {
			missing = append(missing, "delivery")
		}
	}
	if strings.TrimSpace(argx.String(schedule, "timezone")) == "" {
		missing = append(missing, "schedule.timezone")
	}
	if len(missing) > 0 {
		return errors.New("missing required scheduling fields: " + strings.Join(missing, ", ") +
			". Do not assume defaults; use AskUserQuestion to confirm them with the user first")
	}
	return nil
}
