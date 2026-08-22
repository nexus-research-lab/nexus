package echo

import (
	"strings"
	"testing"
	"time"

	echodomain "github.com/nexus-research-lab/nexus/internal/echo"
)

func TestActiveWindow(t *testing.T) {
	t.Parallel()
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		start      string
		end        string
		now        time.Time
		active     bool
		nextHour   int
		nextDayAdd int
	}{
		{name: "daytime", start: "09:00", end: "22:00", now: time.Date(2026, 8, 21, 10, 0, 0, 0, location), active: true},
		{name: "before daytime", start: "09:00", end: "22:00", now: time.Date(2026, 8, 21, 8, 0, 0, 0, location), nextHour: 9},
		{name: "after daytime", start: "09:00", end: "22:00", now: time.Date(2026, 8, 21, 23, 0, 0, 0, location), nextHour: 9, nextDayAdd: 1},
		{name: "overnight", start: "22:00", end: "06:00", now: time.Date(2026, 8, 21, 23, 0, 0, 0, location), active: true},
		{name: "before overnight", start: "22:00", end: "06:00", now: time.Date(2026, 8, 21, 12, 0, 0, 0, location), nextHour: 22},
		{name: "all day", start: "09:00", end: "09:00", now: time.Date(2026, 8, 21, 3, 0, 0, 0, location), active: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := echodomain.DefaultPolicy("Asia/Shanghai")
			policy.ActiveStart = test.start
			policy.ActiveEnd = test.end
			active, next, windowErr := activeWindow(test.now, policy)
			if windowErr != nil {
				t.Fatal(windowErr)
			}
			if active != test.active {
				t.Fatalf("active = %v, want %v", active, test.active)
			}
			localNext := next.In(location)
			wantDay := test.now.In(location).AddDate(0, 0, test.nextDayAdd).Day()
			if !test.active && (localNext.Hour() != test.nextHour || localNext.Day() != wantDay) {
				t.Fatalf("next = %s, want day %d hour %d", localNext, wantDay, test.nextHour)
			}
		})
	}
}

func TestParseGateDecision(t *testing.T) {
	t.Parallel()
	decision, err := parseGateDecision(`{"decision":"follow_up","reason_code":"awaiting_answer","focus":"确认部署窗口"}`)
	if err != nil || decision.Decision != gateFollowUp || decision.Focus != "确认部署窗口" {
		t.Fatalf("decision = %+v, err = %v", decision, err)
	}
	decision, err = parseGateDecision("```json\n{\"decision\":\"skip\",\"reason_code\":\"concluded\",\"focus\":\"\"}\n```")
	if err != nil || decision.Decision != gateSkip || decision.ReasonCode != "concluded" {
		t.Fatalf("fenced decision = %+v, err = %v", decision, err)
	}
	invalid := []string{
		"before\n```json\n{\"decision\":\"skip\",\"reason_code\":\"concluded\",\"focus\":\"\"}\n```",
		`{"decision":"follow_up","reason_code":"awaiting_answer","focus":""}`,
		`{"decision":"follow_up","reason_code":"concluded","focus":"继续"}`,
		`{"decision":"skip","reason_code":"concluded","focus":"","extra":true}`,
		`{"decision":"skip","reason_code":"concluded","focus":""} {}`,
	}
	for _, raw := range invalid {
		if _, err = parseGateDecision(raw); err == nil {
			t.Fatalf("invalid decision was accepted: %s", raw)
		}
	}
}

func TestBuildFollowUpPromptRequiresNaturalReentry(t *testing.T) {
	t.Parallel()
	prompt := buildFollowUpPrompt("确认部署窗口")
	for _, want := range []string{
		"确认部署窗口",
		"自然承接上文",
		"具体的新价值",
		"不要固定套用模板",
		echoNoReplyMarker,
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("Echo prompt 缺少 %q: %s", want, prompt)
		}
	}
}
