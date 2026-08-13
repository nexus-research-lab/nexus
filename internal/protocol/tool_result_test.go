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

func TestParseMutationResultEnvelopeAcceptsSupersededOutcome(t *testing.T) {
	got, ok := ParseMutationResultEnvelope(map[string]any{
		"outcome":     "superseded",
		"reason_code": "execution_terminal",
		"message":     "the bound Room work was replaced",
	})
	if !ok || got.Outcome != MutationResultSuperseded ||
		got.ReasonCode != "execution_terminal" {
		t.Fatalf("ParseMutationResultEnvelope() = %+v/%t", got, ok)
	}
}

func TestParseMutationResultChangedReadsOnlyExplicitEnvelopeRefs(t *testing.T) {
	t.Parallel()

	got := ParseMutationResultChanged(map[string]any{
		"structuredContent": map[string]any{
			"outcome":      "applied",
			"execution_id": "execution-1",
			"changed": []any{
				" assignment:assignment-1 ",
				"attempt:attempt-1",
				"assignment:assignment-1",
			},
		},
	})
	if len(got) != 2 || got[0] != "assignment:assignment-1" ||
		got[1] != "attempt:attempt-1" {
		t.Fatalf("ParseMutationResultChanged() = %#v", got)
	}
	envelope, ok := ParseMutationResultEnvelope(map[string]any{
		"outcome":      "applied",
		"execution_id": "execution-1",
		"changed":      []string{"assignment:assignment-1"},
	})
	if !ok || envelope.ExecutionID != "execution-1" ||
		len(envelope.Changed) != 1 || envelope.Changed[0] != "assignment:assignment-1" {
		t.Fatalf("mutation identity envelope = %+v/%t", envelope, ok)
	}
	if refs := ParseMutationResultChanged(map[string]any{
		"changed": []any{"assignment:untrusted"},
	}); len(refs) != 0 {
		t.Fatalf("non-envelope changed refs = %#v", refs)
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

func TestParseGoalIDResultReadsOnlyExplicitGoalIdentity(t *testing.T) {
	for _, test := range []struct {
		name   string
		value  any
		want   string
		wantOK bool
	}{
		{name: "camel case", value: map[string]any{"goalId": " goal-1 "}, want: "goal-1", wantOK: true},
		{name: "snake case text", value: `{"goal_id":"goal-2"}`, want: "goal-2", wantOK: true},
		{name: "nested goal identity", value: map[string]any{"goal": map[string]any{"id": "goal-3"}}, want: "goal-3", wantOK: true},
		{name: "wrapped", value: map[string]any{"structuredContent": map[string]any{"goalId": "goal-4"}}, want: "goal-4", wantOK: true},
		{name: "unrelated id", value: map[string]any{"id": "tool-1"}},
		{name: "null goal id", value: map[string]any{"goalId": nil}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := ParseGoalIDResult(test.value)
			if got != test.want || ok != test.wantOK {
				t.Fatalf("ParseGoalIDResult() = %q/%v, want %q/%v", got, ok, test.want, test.wantOK)
			}
		})
	}
}
