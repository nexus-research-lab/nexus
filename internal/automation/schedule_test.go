package automation

import (
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/automation/types"
)

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

func intRef(value int) *int {
	result := value
	return &result
}
