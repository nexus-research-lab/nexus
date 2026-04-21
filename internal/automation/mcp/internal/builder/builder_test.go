package builder

import (
	"strings"
	"testing"

	automationsvc "github.com/nexus-research-lab/nexus/internal/automation"
)

func TestSchedule_CronNormalizesToDaily(t *testing.T) {
	cases := []struct {
		name     string
		expr     string
		wantCron string
	}{
		{"every day at 09:00", "0 9 * * *", "0 9 * * *"},
		{"weekdays at 09:00", "0 9 * * 1-5", "0 9 * * 1,2,3,4,5"},
		{"mon/wed/fri at 08:30", "30 8 * * 1,3,5", "30 8 * * 1,3,5"},
		{"sunday at 23:00 (7=sun)", "0 23 * * 7", "0 23 * * 0"},
		{"all weekdays via list collapses to *", "0 9 * * 0,1,2,3,4,5,6", "0 9 * * *"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Schedule(map[string]any{"kind": "cron", "expr": tc.expr}, "Asia/Shanghai")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Kind != automationsvc.ScheduleKindCron {
				t.Fatalf("kind = %q, want cron", got.Kind)
			}
			if got.CronExpression == nil || *got.CronExpression != tc.wantCron {
				t.Fatalf("cron expr = %v, want %q", got.CronExpression, tc.wantCron)
			}
			if got.Timezone != "Asia/Shanghai" {
				t.Fatalf("timezone = %q, want Asia/Shanghai", got.Timezone)
			}
		})
	}
}

func TestSchedule_CronRejectsUneditableExpressions(t *testing.T) {
	cases := []struct {
		name string
		expr string
		hint string
	}{
		{"monthly first day", "0 9 1 * *", "day-of-month and month must both be '*'"},
		{"monthly march", "0 9 * 3 *", "day-of-month and month must both be '*'"},
		{"step minutes", "*/15 * * * *", "minute field must be a single integer"},
		{"step hours", "0 */2 * * *", "hour field must be a single integer"},
		{"weekday step", "0 9 * * */2", "day-of-week step"},
		{"too few fields", "0 9 * *", "expected standard 5-field cron expression"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Schedule(map[string]any{"kind": "cron", "expr": tc.expr}, "Asia/Shanghai")
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.hint) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.hint)
			}
		})
	}
}

func TestSchedule_CronImpliedFromExprAlias(t *testing.T) {
	got, err := Schedule(map[string]any{"cron": "0 9 * * 1-5"}, "Asia/Shanghai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CronExpression == nil || *got.CronExpression != "0 9 * * 1,2,3,4,5" {
		t.Fatalf("cron expr = %v, want weekdays daily form", got.CronExpression)
	}
}
