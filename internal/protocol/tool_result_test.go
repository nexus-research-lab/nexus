package protocol

import "testing"

func TestParseMutationResultEnvelopeAcceptsStructuredAndTextResults(t *testing.T) {
	t.Parallel()

	wantMessage := "Plan Document items must contain at least one complete Work Item"
	for name, value := range map[string]any{
		"structured": map[string]any{
			"outcome": "rejected", "reason_code": "plan_items_empty", "message": wantMessage,
		},
		"json text": `{"outcome":"rejected","reason_code":"plan_items_empty","message":"` + wantMessage + `"}`,
		"wrapped": map[string]any{"structuredContent": map[string]any{
			"outcome": "rejected", "reason_code": "plan_items_empty", "message": wantMessage,
		}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, ok := ParseMutationResultEnvelope(value)
			if !ok {
				t.Fatal("ParseMutationResultEnvelope() did not recognize the envelope")
			}
			if got.Outcome != MutationResultRejected || got.ReasonCode != "plan_items_empty" || got.Message != wantMessage {
				t.Fatalf("ParseMutationResultEnvelope() = %+v", got)
			}
		})
	}
	if result, ok := ParseMutationResultEnvelope(
		map[string]any{"outcome": "maybe", "message": "not a stable envelope"},
		"ordinary tool output",
	); ok {
		t.Fatalf("unexpected mutation result = %+v", result)
	}
}

func TestParseGoalStatusResultReadsOnlyTerminalGoalStatus(t *testing.T) {
	for _, test := range []struct {
		name   string
		value  any
		want   GoalStatus
		wantOK bool
	}{
		{name: "complete structured", value: map[string]any{"goal": map[string]any{"status": "complete"}}, want: GoalStatusComplete, wantOK: true},
		{name: "blocked text", value: `{"goal":{"status":"blocked"}}`, want: GoalStatusBlocked, wantOK: true},
		{name: "unrelated status", value: map[string]any{"status": "complete"}},
		{name: "active goal", value: map[string]any{"goal": map[string]any{"status": "active"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := ParseGoalStatusResult(test.value)
			if got != test.want || ok != test.wantOK {
				t.Fatalf("ParseGoalStatusResult() = %q/%v, want %q/%v", got, ok, test.want, test.wantOK)
			}
		})
	}
}
