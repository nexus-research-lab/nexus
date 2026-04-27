package automation

import (
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

var standardCronParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

// ComputeNextRunAt 计算下次触发时间。
func ComputeNextRunAt(schedule Schedule, now time.Time) (*time.Time, error) {
	normalized := schedule.Normalized()
	if err := normalized.Validate(); err != nil {
		return nil, err
	}

	utcNow := now.UTC()
	switch normalized.Kind {
	case ScheduleKindEvery:
		next := utcNow.Add(time.Duration(*normalized.IntervalSeconds) * time.Second)
		return &next, nil
	case ScheduleKindAt:
		next, err := parseRunAt(*normalized.RunAt, normalized.Timezone)
		if err != nil {
			return nil, err
		}
		if next.Before(utcNow) {
			return nil, nil
		}
		return &next, nil
	case ScheduleKindCron:
		scheduled, err := parseCronExpression(*normalized.CronExpression, normalized.Timezone)
		if err != nil {
			return nil, err
		}
		next := scheduled.Next(utcNow)
		if next.IsZero() {
			return nil, nil
		}
		return &next, nil
	default:
		return nil, fmt.Errorf("unsupported schedule kind: %s", normalized.Kind)
	}
}

func parseRunAt(raw string, timezoneName string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	location, err := time.LoadLocation(strings.TrimSpace(timezoneName))
	if err != nil {
		return time.Time{}, err
	}

	// 前端当前会提交 `YYYY-MM-DDTHH:mm` 本地时间，这里优先按本地时区解释，
	// 如果字符串自身已经带时区，则直接尊重调用方提供的偏移。
	if parsed, parseErr := time.Parse(time.RFC3339, value); parseErr == nil {
		return parsed.UTC(), nil
	}
	if parsed, parseErr := time.ParseInLocation("2006-01-02T15:04", value, location); parseErr == nil {
		return parsed.UTC(), nil
	}
	if parsed, parseErr := time.ParseInLocation("2006-01-02 15:04:05", value, location); parseErr == nil {
		return parsed.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid run_at: %s", raw)
}

func parseCronExpression(expression string, timezoneName string) (cron.Schedule, error) {
	normalized := strings.TrimSpace(expression)
	if normalized == "" {
		return nil, fmt.Errorf("cron_expression is required")
	}
	if timezoneName != "" {
		normalized = fmt.Sprintf("CRON_TZ=%s %s", strings.TrimSpace(timezoneName), normalized)
	}
	return standardCronParser.Parse(normalized)
}
