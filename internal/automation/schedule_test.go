package automation

import (
	"github.com/nexus-research-lab/nexus/internal/automation/types"
	"testing"
	"time"
)

func TestComputeNextRunAt(t *testing.T) {
	now := time.Date(2026, 4, 11, 8, 0, 0, 0, time.UTC)

	every := types.Schedule{
		Kind:            types.ScheduleKindEvery,
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

	at := types.Schedule{
		Kind:     types.ScheduleKindAt,
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
	cronSchedule := types.Schedule{
		Kind:           types.ScheduleKindCron,
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

func TestComputeJitteredNextRunAtIsStableAndBounded(t *testing.T) {
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	schedule := types.Schedule{
		Kind:            types.ScheduleKindEvery,
		IntervalSeconds: intRef(3600),
		Timezone:        "Asia/Shanghai",
	}
	first, err := ComputeJitteredNextRunAt(schedule, now, "task-a", 15*time.Minute)
	if err != nil {
		t.Fatalf("ComputeJitteredNextRunAt 失败: %v", err)
	}
	second, err := ComputeJitteredNextRunAt(schedule, now, "task-a", 15*time.Minute)
	if err != nil {
		t.Fatalf("ComputeJitteredNextRunAt 再次计算失败: %v", err)
	}
	base := now.Add(time.Hour)
	if first == nil || second == nil || !first.Equal(*second) {
		t.Fatalf("同一任务的 jitter 应稳定: first=%v second=%v", first, second)
	}
	if first.Before(base) || !first.Before(base.Add(6*time.Minute)) {
		t.Fatalf("每小时任务应落在 10%% 窗口内: base=%s got=%s", base, first)
	}
}

func TestComputeJitteredNextRunAtKeepsOneShotExact(t *testing.T) {
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	runAt := "2026-06-11T11:00:00Z"
	schedule := types.Schedule{Kind: types.ScheduleKindAt, RunAt: &runAt, Timezone: "UTC"}
	next, err := ComputeJitteredNextRunAt(schedule, now, "task-a", 15*time.Minute)
	if err != nil {
		t.Fatalf("ComputeJitteredNextRunAt 失败: %v", err)
	}
	want := time.Date(2026, 6, 11, 11, 0, 0, 0, time.UTC)
	if next == nil || !next.Equal(want) {
		t.Fatalf("单次任务不应添加 jitter: got=%v want=%s", next, want)
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
