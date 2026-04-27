package automation

import (
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestComputeNextRunAt(t *testing.T) {
	now := time.Date(2026, 4, 11, 8, 0, 0, 0, time.UTC)

	every := protocol.Schedule{
		Kind:            protocol.ScheduleKindEvery,
		IntervalSeconds: intRef(1800),
		Timezone:        "Asia/Shanghai",
	}
	nextEvery, err := ComputeNextRunAt(every, now)
	if err != nil {
		t.Fatalf("every 调度计算失败: %v", err)
	}
	if nextEvery == nil || !nextEvery.Equal(now.Add(30*time.Minute)) {
		t.Fatalf("every 下次触发时间错误: %v", nextEvery)
	}

	at := protocol.Schedule{
		Kind:     protocol.ScheduleKindAt,
		RunAt:    stringRef("2026-04-11T18:30"),
		Timezone: "Asia/Shanghai",
	}
	nextAt, err := ComputeNextRunAt(at, now)
	if err != nil {
		t.Fatalf("at 调度计算失败: %v", err)
	}
	expectedAt := time.Date(2026, 4, 11, 10, 30, 0, 0, time.UTC)
	if nextAt == nil || !nextAt.Equal(expectedAt) {
		t.Fatalf("at 下次触发时间错误: got=%v want=%v", nextAt, expectedAt)
	}

	cronExpr := "0 9 * * *"
	cronSchedule := protocol.Schedule{
		Kind:           protocol.ScheduleKindCron,
		CronExpression: &cronExpr,
		Timezone:       "Asia/Shanghai",
	}
	nextCron, err := ComputeNextRunAt(cronSchedule, now)
	if err != nil {
		t.Fatalf("cron 调度计算失败: %v", err)
	}
	expectedCron := time.Date(2026, 4, 12, 1, 0, 0, 0, time.UTC)
	if nextCron == nil || !nextCron.Equal(expectedCron) {
		t.Fatalf("cron 下次触发时间错误: got=%v want=%v", nextCron, expectedCron)
	}
}

func intRef(value int) *int {
	result := value
	return &result
}

func stringRef(value string) *string {
	result := value
	return &result
}
