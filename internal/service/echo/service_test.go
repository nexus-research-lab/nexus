package echo

import (
	"context"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

type stubPreferences struct{ enabled bool }

func (p stubPreferences) Get(context.Context, string) (preferencessvc.Preferences, error) {
	return preferencessvc.Preferences{EchoEnabled: p.enabled, Version: 1}, nil
}

func (p stubPreferences) SetEchoEnabled(context.Context, string, bool) (preferencessvc.Preferences, error) {
	return preferencessvc.Preferences{}, nil
}

func (p stubPreferences) SetEchoEnabledAtVersion(context.Context, string, bool, int64) (preferencessvc.Preferences, error) {
	return preferencessvc.Preferences{}, nil
}

func newPolicyTestService(enabled bool, now time.Time) *Service {
	service := NewService(config.Config{DefaultTimezone: "UTC"}, nil, nil, nil, stubPreferences{enabled: enabled})
	service.nowFn = func() time.Time { return now }
	return service
}

func TestScheduleAfterWaitsIdleDelayInsideActiveHours(t *testing.T) {
	service := newPolicyTestService(true, time.Time{})
	// 10:00 结束 + 6h = 16:00，在 09:00–22:00 内。
	dueAt, ok, err := service.ScheduleAfter(context.Background(), "owner", time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC))
	if err != nil || !ok || !dueAt.Equal(time.Date(2026, 11, 26, 16, 0, 0, 0, time.UTC)) {
		t.Fatalf("dueAt=%v ok=%v err=%v", dueAt, ok, err)
	}
	// 20:00 结束 + 6h = 次日 02:00，顺延到次日 09:00。
	dueAt, _, _ = service.ScheduleAfter(context.Background(), "owner", time.Date(2026, 11, 26, 20, 0, 0, 0, time.UTC))
	if !dueAt.Equal(time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("应顺延到活跃时段开始: %v", dueAt)
	}
}

func TestScheduleAfterSkipsWhenDisabled(t *testing.T) {
	service := newPolicyTestService(false, time.Time{})
	if _, ok, err := service.ScheduleAfter(context.Background(), "owner", time.Now()); ok || err != nil {
		t.Fatalf("关闭时不应安排 Echo: ok=%v err=%v", ok, err)
	}
}

func TestDecideStaysSilentWhenDisabledAndDefersOutsideActiveHours(t *testing.T) {
	candidate := dmsvc.EchoCandidate{OwnerUserID: "owner", AgentID: "nexus", SessionKey: "s"}
	decision, err := newPolicyTestService(false, time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)).Decide(context.Background(), candidate)
	if err != nil || decision.Prompt != "" || !decision.DeferUntil.IsZero() || decision.Reason != "disabled" {
		t.Fatalf("关闭后到点应沉默: %+v err=%v", decision, err)
	}
	decision, err = newPolicyTestService(true, time.Date(2026, 11, 26, 23, 0, 0, 0, time.UTC)).Decide(context.Background(), candidate)
	if err != nil || decision.Prompt != "" || !decision.DeferUntil.Equal(time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("活跃时段外应顺延: %+v err=%v", decision, err)
	}
}

func TestParseGateDecisionRejectsUnknownReason(t *testing.T) {
	if _, err := parseGateDecision(`{"decision":"follow_up","reason_code":"bored","focus":"x"}`); err == nil {
		t.Fatal("未知 reason_code 应拒绝")
	}
	decision, err := parseGateDecision("```json\n{\"decision\":\"follow_up\",\"reason_code\":\"awaiting_answer\",\"focus\":\"确认部署窗口\"}\n```")
	if err != nil || decision.Focus != "确认部署窗口" {
		t.Fatalf("decision=%+v err=%v", decision, err)
	}
}
