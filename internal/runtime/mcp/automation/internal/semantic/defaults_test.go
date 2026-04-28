package semantic

import (
	"encoding/json"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/contract"
)

func TestReassembleFlatScheduleMergesDottedAndTopLevelFields(t *testing.T) {
	args := map[string]any{
		"name":                    "test",
		"instruction":             "提醒我喝水",
		"schedule":                map[string]any{"kind": "interval"},
		"schedule.interval_value": json.Number("1"),
		"interval_unit":           "minutes",
	}

	ReassembleFlatSchedule(args)

	schedule, ok := args["schedule"].(map[string]any)
	if !ok {
		t.Fatalf("schedule 未重组为对象: %+v", args["schedule"])
	}
	if schedule["kind"] != "interval" {
		t.Fatalf("schedule.kind = %v, want interval", schedule["kind"])
	}
	if schedule["interval_value"] != json.Number("1") {
		t.Fatalf("schedule.interval_value = %#v, want json.Number(1)", schedule["interval_value"])
	}
	if schedule["interval_unit"] != "minutes" {
		t.Fatalf("schedule.interval_unit = %v, want minutes", schedule["interval_unit"])
	}
	sctx := contract.ServerContext{CurrentSessionKey: "agent:agent-1:dm:dm-user:main:"}
	if !CanDefaultSimpleReminder(args, sctx) {
		t.Fatalf("短提醒应允许当前会话可见默认: %+v", args)
	}
}

func TestReassembleFlatScheduleBuildsScheduleFromDottedFields(t *testing.T) {
	args := map[string]any{
		"name":                    "test",
		"instruction":             "提醒我喝水",
		"schedule.kind":           "interval",
		"schedule.interval_value": json.Number("1"),
		"schedule.interval_unit":  "minutes",
	}

	ReassembleFlatSchedule(args)

	schedule, ok := args["schedule"].(map[string]any)
	if !ok {
		t.Fatalf("schedule 未从 dotted 字段重组: %+v", args["schedule"])
	}
	if schedule["kind"] != "interval" || schedule["interval_value"] != json.Number("1") || schedule["interval_unit"] != "minutes" {
		t.Fatalf("schedule 字段不正确: %+v", schedule)
	}
}
