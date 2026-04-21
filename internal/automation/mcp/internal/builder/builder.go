// Package builder 把 MCP 工具入参里的对象翻译成 automation 底层结构，
// 并复用底层的 Normalize + Validate。
package builder

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	automationsvc "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/internal/argx"
)

// weekdayCronValue 把 UI 三字母简写映射到 cron 的 day-of-week 数值。
var weekdayCronValue = map[string]int{
	"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6,
}

var allWeekdayKeys = []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

// Schedule 把 UI 对齐的 schedule 对象翻译成底层 Schedule。
// 支持 kind=single/daily/interval/cron，其中 cron 允许直接传 raw cron 表达式（对齐 OpenClaw 的易用写法）。
// 入参里若 timezone 为空，使用 defaultTimezone 回退（通常来自 cfg.DefaultTimezone=Asia/Shanghai）。
func Schedule(raw any, defaultTimezone string) (automationsvc.Schedule, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return automationsvc.Schedule{}, errors.New("schedule must be an object")
	}
	kind := strings.TrimSpace(argx.String(m, "kind"))
	timezone := strings.TrimSpace(argx.String(m, "timezone"))
	if timezone == "" {
		timezone = strings.TrimSpace(defaultTimezone)
	}
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}

	// 允许用户直接传 cron 字符串到 schedule.expr / schedule.cron，自动推导 kind=cron。
	exprAlias := strings.TrimSpace(argx.FirstNonEmpty(argx.String(m, "expr"), argx.String(m, "cron"), argx.String(m, "cron_expression")))
	if kind == "" && exprAlias != "" {
		kind = "cron"
	}

	switch kind {
	case "single":
		runAt := strings.TrimSpace(argx.String(m, "run_at"))
		if runAt == "" {
			return automationsvc.Schedule{}, errors.New("schedule.run_at is required when kind=single")
		}
		schedule := automationsvc.Schedule{Kind: automationsvc.ScheduleKindAt, RunAt: &runAt, Timezone: timezone}
		return validateAndNormalize(schedule)
	case "daily":
		dailyTime := strings.TrimSpace(argx.String(m, "daily_time"))
		weekdays, err := normalizeWeekdays(m["weekdays"])
		if err != nil {
			return automationsvc.Schedule{}, err
		}
		cronExpr, err := buildDailyCron(dailyTime, weekdays)
		if err != nil {
			return automationsvc.Schedule{}, err
		}
		schedule := automationsvc.Schedule{Kind: automationsvc.ScheduleKindCron, CronExpression: &cronExpr, Timezone: timezone}
		return validateAndNormalize(schedule)
	case "interval":
		value := argx.Int(m["interval_value"])
		if value <= 0 {
			return automationsvc.Schedule{}, errors.New("schedule.interval_value must be a positive integer when kind=interval")
		}
		unit := strings.TrimSpace(argx.String(m, "interval_unit"))
		seconds, err := intervalSeconds(value, unit)
		if err != nil {
			return automationsvc.Schedule{}, err
		}
		schedule := automationsvc.Schedule{Kind: automationsvc.ScheduleKindEvery, IntervalSeconds: &seconds, Timezone: timezone}
		return validateAndNormalize(schedule)
	case "cron":
		if exprAlias == "" {
			return automationsvc.Schedule{}, errors.New("schedule.expr is required when kind=cron (standard 5-field cron expression, e.g. \"0 9 * * 1-5\")")
		}
		schedule := automationsvc.Schedule{Kind: automationsvc.ScheduleKindCron, CronExpression: &exprAlias, Timezone: timezone}
		return validateAndNormalize(schedule)
	case "":
		return automationsvc.Schedule{}, errors.New("schedule.kind is required (single / daily / interval / cron)")
	default:
		return automationsvc.Schedule{}, fmt.Errorf("schedule.kind must be one of single, daily, interval, cron (got %q)", kind)
	}
}

func validateAndNormalize(schedule automationsvc.Schedule) (automationsvc.Schedule, error) {
	normalized := schedule.Normalized()
	if err := normalized.Validate(); err != nil {
		return automationsvc.Schedule{}, err
	}
	return normalized, nil
}

func normalizeWeekdays(raw any) ([]string, error) {
	if raw == nil {
		return nil, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, errors.New("schedule.weekdays must be an array of weekday strings")
	}
	if len(list) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(list))
	result := make([]string, 0, len(list))
	for _, item := range list {
		key := strings.ToLower(strings.TrimSpace(argx.StringOf(item)))
		if key == "" {
			continue
		}
		if _, exists := weekdayCronValue[key]; !exists {
			return nil, fmt.Errorf("schedule.weekdays contains unsupported value %q (allowed: mon/tue/wed/thu/fri/sat/sun)", key)
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result, nil
}

func buildDailyCron(dailyTime string, weekdays []string) (string, error) {
	if dailyTime == "" {
		return "", errors.New("schedule.daily_time is required when kind=daily (HH:MM)")
	}
	parts := strings.Split(dailyTime, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("schedule.daily_time must be HH:MM (got %q)", dailyTime)
	}
	hour, err := parseTwoDigit(parts[0], 0, 23)
	if err != nil {
		return "", fmt.Errorf("schedule.daily_time hour invalid: %w", err)
	}
	minute, err := parseTwoDigit(parts[1], 0, 59)
	if err != nil {
		return "", fmt.Errorf("schedule.daily_time minute invalid: %w", err)
	}
	dow := "*"
	if len(weekdays) > 0 && len(weekdays) < len(allWeekdayKeys) {
		values := make([]int, 0, len(weekdays))
		for _, key := range weekdays {
			values = append(values, weekdayCronValue[key])
		}
		sort.Ints(values)
		strs := make([]string, len(values))
		for i, v := range values {
			strs[i] = fmt.Sprintf("%d", v)
		}
		dow = strings.Join(strs, ",")
	}
	return fmt.Sprintf("%d %d * * %s", minute, hour, dow), nil
}

func parseTwoDigit(s string, min, max int) (int, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 || len(s) > 2 {
		return 0, fmt.Errorf("expected 1-2 digit number, got %q", s)
	}
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return 0, err
	}
	if n < min || n > max {
		return 0, fmt.Errorf("value %d out of range [%d,%d]", n, min, max)
	}
	return n, nil
}

func intervalSeconds(value int, unit string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "", "seconds":
		return value, nil
	case "minutes":
		return value * 60, nil
	case "hours":
		return value * 3600, nil
	default:
		return 0, fmt.Errorf("schedule.interval_unit must be one of seconds, minutes, hours (got %q)", unit)
	}
}

// SessionTarget 把 session_target 对象翻译成底层 SessionTarget。
// 当 kind=bound 且未填 bound_session_key 时，使用当前会话 fallback。
func SessionTarget(raw any, currentSessionKey string) (automationsvc.SessionTarget, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return automationsvc.SessionTarget{}, errors.New("session_target must be an object")
	}
	target := automationsvc.SessionTarget{
		Kind:            argx.String(m, "kind"),
		BoundSessionKey: argx.String(m, "bound_session_key"),
		NamedSessionKey: argx.String(m, "named_session_key"),
		WakeMode:        argx.String(m, "wake_mode"),
	}
	if target.Kind == automationsvc.SessionTargetBound && target.BoundSessionKey == "" && currentSessionKey != "" {
		target.BoundSessionKey = currentSessionKey
	}
	normalized := target.Normalized()
	if err := normalized.Validate(); err != nil {
		return automationsvc.SessionTarget{}, err
	}
	return normalized, nil
}

// Delivery 把 delivery 对象翻译成底层 DeliveryTarget。
// 当 mode=explicit 且未填 to 时，使用当前会话 fallback 并补默认 channel=websocket。
func Delivery(raw any, currentSessionKey string) (automationsvc.DeliveryTarget, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return automationsvc.DeliveryTarget{}, errors.New("delivery must be an object")
	}
	delivery := automationsvc.DeliveryTarget{
		Mode:      argx.String(m, "mode"),
		Channel:   argx.String(m, "channel"),
		To:        argx.String(m, "to"),
		AccountID: argx.String(m, "account_id"),
		ThreadID:  argx.String(m, "thread_id"),
	}
	if delivery.Mode == automationsvc.DeliveryModeExplicit && delivery.To == "" && currentSessionKey != "" {
		if delivery.Channel == "" {
			delivery.Channel = "websocket"
		}
		delivery.To = currentSessionKey
	}
	normalized := delivery.Normalized()
	if err := normalized.Validate(); err != nil {
		return automationsvc.DeliveryTarget{}, err
	}
	return normalized, nil
}

// Source 把 source 对象翻译成底层 Source。
func Source(raw any) (automationsvc.Source, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return automationsvc.Source{}, errors.New("source must be an object")
	}
	source := automationsvc.Source{
		Kind:           argx.String(m, "kind"),
		CreatorAgentID: argx.String(m, "creator_agent_id"),
		ContextType:    argx.String(m, "context_type"),
		ContextID:      argx.String(m, "context_id"),
		ContextLabel:   argx.String(m, "context_label"),
		SessionKey:     argx.String(m, "session_key"),
		SessionLabel:   argx.String(m, "session_label"),
	}
	normalized := source.Normalized()
	if err := normalized.Validate(); err != nil {
		return automationsvc.Source{}, err
	}
	return normalized, nil
}
