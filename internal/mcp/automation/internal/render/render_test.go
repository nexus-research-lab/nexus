package render

import "testing"

func TestDecorateTimesRecursesNestedPayloads(t *testing.T) {
	payload := map[string]any{
		"timezone": "Asia/Shanghai",
		"job": map[string]any{
			"schedule":    map[string]any{"timezone": "America/New_York"},
			"next_run_at": "2026-05-25T13:00:00Z",
		},
		"recent_runs": []any{
			map[string]any{
				"delivery_dead_letter_at": "2026-05-25T13:30:00Z",
			},
		},
		"recent_events": []any{
			map[string]any{
				"created_at": "2026-05-25T14:00:00Z",
				"detail": map[string]any{
					"delivery_next_attempt_at": "2026-05-25T14:30:00Z",
				},
			},
		},
	}

	decorated, ok := DecorateTimes(payload, "").(map[string]any)
	if !ok {
		t.Fatalf("DecorateTimes should return map, got %T", decorated)
	}
	job := decorated["job"].(map[string]any)
	if job["next_run_at_display"] != "2026-05-25 09:00:00 EDT" {
		t.Fatalf("job time should use schedule timezone, got %+v", job["next_run_at_display"])
	}
	run := decorated["recent_runs"].([]any)[0].(map[string]any)
	if run["delivery_dead_letter_at_display"] != "2026-05-25 21:30:00 CST" {
		t.Fatalf("run time should use inherited timezone, got %+v", run["delivery_dead_letter_at_display"])
	}
	event := decorated["recent_events"].([]any)[0].(map[string]any)
	if event["created_at_display"] != "2026-05-25 22:00:00 CST" {
		t.Fatalf("event created_at should be decorated, got %+v", event["created_at_display"])
	}
	detail := event["detail"].(map[string]any)
	if detail["delivery_next_attempt_at_display"] != "2026-05-25 22:30:00 CST" {
		t.Fatalf("event detail time should be decorated, got %+v", detail["delivery_next_attempt_at_display"])
	}
}
