// INPUT: configuration templates containing secret placeholders, direct values, and malformed slots.
// OUTPUT: proof that only human-materialized values cross the write boundary and model-visible projections stay redacted.
// POS: regression tests for the conversational configuration secret boundary.
package secretinput

import (
	"encoding/json"
	"testing"
)

func TestPrepareJSONRejectsModelVisibleSecrets(t *testing.T) {
	tests := map[string]json.RawMessage{
		"direct secret":          json.RawMessage(`{"auth_token":"model-saw-this"}`),
		"non-secret placeholder": json.RawMessage(`{"display_name":{"$secret":"display.name.slot"}}`),
		"short slot":             json.RawMessage(`{"password":{"$secret":"short"}}`),
		"reused slot": json.RawMessage(`{
			"password":{"$secret":"shared.secret.slot"},
			"client_secret":{"$secret":"shared.secret.slot"}
		}`),
		"credential URL": json.RawMessage(`{"base_url":"https://user:pass@example.test/v1"}`),
		"query secret":   json.RawMessage(`{"base_url":"https://example.test/v1?api_key=value"}`),
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if _, _, err := PrepareJSON(input); err == nil {
				t.Fatalf("PrepareJSON(%s) unexpectedly succeeded", input)
			}
		})
	}
}

func TestMaterializeJSONRequiresExactSlots(t *testing.T) {
	template := json.RawMessage(`{"password":{"$secret":"password.slot"}}`)
	tests := map[string]map[string]string{
		"missing": nil,
		"empty":   {"password.slot": " "},
		"extra":   {"password.slot": "value", "unexpected.slot": "value"},
	}
	for name, values := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := MaterializeJSON(template, values); err == nil {
				t.Fatal("MaterializeJSON() unexpectedly succeeded")
			}
		})
	}
}
