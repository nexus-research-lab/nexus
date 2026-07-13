package types

import (
	"errors"
	"strings"
)

type scheduleValidator func(Schedule) error

var scheduleValidators = map[string]scheduleValidator{
	ScheduleKindEvery: validateEverySchedule,
	ScheduleKindAt:    validateAtSchedule,
	ScheduleKindCron:  validateCronSchedule,
}

// Schedule 表示结构化调度定义。
type Schedule struct {
	Kind            string  `json:"kind"`
	RunAt           *string `json:"run_at,omitempty"`
	IntervalSeconds *int    `json:"interval_seconds,omitempty"`
	CronExpression  *string `json:"cron_expression,omitempty"`
	Timezone        string  `json:"timezone,omitempty"`
}

// Validate 校验调度形状。
func (s Schedule) Validate() error {
	normalized := s.Normalized()
	validate := scheduleValidators[normalized.Kind]
	if validate == nil {
		return errors.New("schedule.kind must be one of every, cron, at")
	}
	return validate(normalized)
}

func validateEverySchedule(schedule Schedule) error {
	if schedule.IntervalSeconds == nil || *schedule.IntervalSeconds <= 0 {
		return errors.New("interval_seconds must be greater than 0 when kind is every")
	}
	if schedule.RunAt != nil || schedule.CronExpression != nil {
		return errors.New("run_at and cron_expression must be empty when kind is every")
	}
	return nil
}

func validateAtSchedule(schedule Schedule) error {
	if schedule.RunAt == nil || *schedule.RunAt == "" {
		return errors.New("run_at is required when kind is at")
	}
	if schedule.IntervalSeconds != nil || schedule.CronExpression != nil {
		return errors.New("interval_seconds and cron_expression must be empty when kind is at")
	}
	return nil
}

func validateCronSchedule(schedule Schedule) error {
	if schedule.CronExpression == nil || *schedule.CronExpression == "" {
		return errors.New("cron_expression is required when kind is cron")
	}
	if schedule.RunAt != nil || schedule.IntervalSeconds != nil {
		return errors.New("run_at and interval_seconds must be empty when kind is cron")
	}
	return nil
}

// Normalized 返回带默认值的调度副本。
func (s Schedule) Normalized() Schedule {
	result := s
	result.Kind = strings.TrimSpace(result.Kind)
	if strings.TrimSpace(result.Timezone) == "" {
		result.Timezone = "Asia/Shanghai"
	}
	if result.RunAt != nil {
		value := strings.TrimSpace(*result.RunAt)
		result.RunAt = &value
	}
	if result.CronExpression != nil {
		value := strings.TrimSpace(*result.CronExpression)
		result.CronExpression = &value
	}
	return result
}
