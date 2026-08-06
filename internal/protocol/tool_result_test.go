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
}

func TestParseMutationResultEnvelopeRejectsUnrecognizedOutput(t *testing.T) {
	t.Parallel()

	if result, ok := ParseMutationResultEnvelope(
		map[string]any{"outcome": "maybe", "message": "not a stable envelope"},
		"ordinary tool output",
	); ok {
		t.Fatalf("unexpected mutation result = %+v", result)
	}
}
