// INPUT: Nexus Automation command 的 UI 对齐 schedule。
// OUTPUT: 可由 scheduler 持久化且仍可被页面编辑的 Schedule。
// POS: Automation command plan/apply 共用的唯一调度翻译入口。
package automation

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

var automationCommandWeekday = map[string]int{
	"sun": 0, "su": 0,
	"mon": 1, "mo": 1,
	"tue": 2, "tu": 2,
	"wed": 3, "we": 3,
	"thu": 4, "th": 4,
	"fri": 5, "fr": 5,
	"sat": 6, "sa": 6,
}

func automationCommandSchedule(
	input *automationdomain.AutomationCommandSchedule,
	defaultTimezone string,
) (automationdomain.Schedule, error) {
	if input == nil {
		return automationdomain.Schedule{}, errors.New("schedule is required")
	}
	timezone := strings.TrimSpace(input.Timezone)
	if timezone == "" {
		timezone = strings.TrimSpace(defaultTimezone)
	}
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	switch strings.ToLower(strings.TrimSpace(input.Kind)) {
	case "single":
		runAt := strings.TrimSpace(input.RunAt)
		if runAt == "" {
			return automationdomain.Schedule{}, errors.New("schedule.run_at is required when kind=single")
		}
		return validateAutomationCommandSchedule(automationdomain.Schedule{
			Kind: automationdomain.ScheduleKindAt, RunAt: &runAt, Timezone: timezone,
		})
	case "daily":
		expression, err := automationCommandDailyCron(input.DailyTime, input.Weekdays)
		if err != nil {
			return automationdomain.Schedule{}, err
		}
		return validateAutomationCommandSchedule(automationdomain.Schedule{
			Kind: automationdomain.ScheduleKindCron, CronExpression: &expression, Timezone: timezone,
		})
	case "interval":
		seconds, err := automationCommandIntervalSeconds(input.IntervalValue, input.IntervalUnit)
		if err != nil {
			return automationdomain.Schedule{}, err
		}
		return validateAutomationCommandSchedule(automationdomain.Schedule{
			Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: &seconds, Timezone: timezone,
		})
	case "cron":
		return automationCommandCronSchedule(input.Expr, timezone)
	default:
		return automationdomain.Schedule{}, errors.New("schedule.kind must be one of single, daily, interval, cron")
	}
}

func validateAutomationCommandSchedule(value automationdomain.Schedule) (automationdomain.Schedule, error) {
	normalized := value.Normalized()
	if err := normalized.Validate(); err != nil {
		return automationdomain.Schedule{}, err
	}
	return normalized, nil
}

func automationCommandDailyCron(dailyTime string, weekdays []string) (string, error) {
	parts := strings.Split(strings.TrimSpace(dailyTime), ":")
	if len(parts) != 2 {
		return "", errors.New("schedule.daily_time must use HH:MM when kind=daily")
	}
	hour, err := automationCommandClockPart(parts[0], 0, 23)
	if err != nil {
		return "", fmt.Errorf("schedule.daily_time hour: %w", err)
	}
	minute, err := automationCommandClockPart(parts[1], 0, 59)
	if err != nil {
		return "", fmt.Errorf("schedule.daily_time minute: %w", err)
	}
	values := make([]int, 0, len(weekdays))
	seen := map[int]struct{}{}
	for _, weekday := range weekdays {
		value, ok := automationCommandWeekday[strings.ToLower(strings.TrimSpace(weekday))]
		if !ok {
			return "", fmt.Errorf("schedule.weekdays contains unsupported value %q", weekday)
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	sort.Ints(values)
	dayOfWeek := "*"
	if len(values) > 0 && len(values) < 7 {
		parts := make([]string, len(values))
		for index, value := range values {
			parts[index] = strconv.Itoa(value)
		}
		dayOfWeek = strings.Join(parts, ",")
	}
	return fmt.Sprintf("%d %d * * %s", minute, hour, dayOfWeek), nil
}

func automationCommandClockPart(value string, minimum int, maximum int) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < minimum || parsed > maximum {
		return 0, fmt.Errorf("must be an integer in [%d,%d]", minimum, maximum)
	}
	return parsed, nil
}

func automationCommandIntervalSeconds(value int, unit string) (int, error) {
	if value <= 0 {
		return 0, errors.New("schedule.interval_value must be positive when kind=interval")
	}
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "", "seconds":
		return value, nil
	case "minutes":
		return value * 60, nil
	case "hours":
		return value * 3600, nil
	default:
		return 0, errors.New("schedule.interval_unit must be one of seconds, minutes, hours")
	}
}

func automationCommandCronSchedule(expression string, timezone string) (automationdomain.Schedule, error) {
	expression = strings.TrimSpace(expression)
	fields := strings.Fields(expression)
	if len(fields) != 5 {
		return automationdomain.Schedule{}, errors.New("schedule.expr must be a standard five-field cron expression")
	}
	schedule, err := validateAutomationCommandSchedule(automationdomain.Schedule{
		Kind: automationdomain.ScheduleKindCron, CronExpression: &expression, Timezone: timezone,
	})
	if err != nil {
		return automationdomain.Schedule{}, err
	}
	if err = validateRunnableSchedule(schedule); err != nil {
		return automationdomain.Schedule{}, fmt.Errorf("schedule.expr: %w", err)
	}
	return schedule, nil
}
