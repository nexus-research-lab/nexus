package protocol

import (
	"errors"
	"strings"
)

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
	kind := strings.TrimSpace(s.Kind)
	timezoneName := strings.TrimSpace(s.Timezone)
	if timezoneName == "" {
		timezoneName = "Asia/Shanghai"
	}
	switch kind {
	case ScheduleKindEvery:
		if s.IntervalSeconds == nil || *s.IntervalSeconds <= 0 {
			return errors.New("interval_seconds must be greater than 0 when kind is every")
		}
		if s.RunAt != nil || s.CronExpression != nil {
			return errors.New("run_at and cron_expression must be empty when kind is every")
		}
	case ScheduleKindAt:
		if s.RunAt == nil || strings.TrimSpace(*s.RunAt) == "" {
			return errors.New("run_at is required when kind is at")
		}
		if s.IntervalSeconds != nil || s.CronExpression != nil {
			return errors.New("interval_seconds and cron_expression must be empty when kind is at")
		}
	case ScheduleKindCron:
		if s.CronExpression == nil || strings.TrimSpace(*s.CronExpression) == "" {
			return errors.New("cron_expression is required when kind is cron")
		}
		if s.RunAt != nil || s.IntervalSeconds != nil {
			return errors.New("run_at and interval_seconds must be empty when kind is cron")
		}
	default:
		return errors.New("schedule.kind must be one of every, cron, at")
	}
	if strings.TrimSpace(timezoneName) == "" {
		return errors.New("timezone is required")
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
