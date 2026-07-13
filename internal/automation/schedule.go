package automation

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/robfig/cron/v3"
)

var standardCronParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

// ComputeNextRunAt 计算下次触发时间。
func ComputeNextRunAt(schedule types.Schedule, now time.Time) (*time.Time, error) {
	normalized := schedule.Normalized()
	if err := normalized.Validate(); err != nil {
		return nil, err
	}

	utcNow := now.UTC()
	switch normalized.Kind {
	case types.ScheduleKindEvery:
		next := utcNow.Add(time.Duration(*normalized.IntervalSeconds) * time.Second)
		return &next, nil
	case types.ScheduleKindAt:
		next, err := parseRunAt(*normalized.RunAt, normalized.Timezone)
		if err != nil {
			return nil, err
		}
		if next.Before(utcNow) {
			return nil, nil
		}
		return &next, nil
	case types.ScheduleKindCron:
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

// ComputeJitteredNextRunAt 为循环任务附加稳定延迟，分散整点触发压力。
func ComputeJitteredNextRunAt(
	schedule types.Schedule,
	now time.Time,
	stableKey string,
	maxJitter time.Duration,
) (*time.Time, error) {
	next, err := ComputeNextRunAt(schedule, now)
	if err != nil || next == nil || maxJitter <= 0 || schedule.Normalized().Kind == types.ScheduleKindAt {
		return next, err
	}
	second, err := ComputeNextRunAt(schedule, *next)
	if err != nil || second == nil {
		return next, err
	}
	window := second.Sub(*next) / 10
	if window > maxJitter {
		window = maxJitter
	}
	if window <= 0 {
		return next, nil
	}
	result := next.Add(stableJitterOffset(stableKey, window))
	return &result, nil
}

func stableJitterOffset(key string, window time.Duration) time.Duration {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(strings.TrimSpace(key)))
	return time.Duration(float64(window) * float64(hash.Sum32()) / float64(^uint32(0)))
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
