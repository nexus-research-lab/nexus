package argx

import (
	"encoding/json"
	"testing"
)

func TestIntAcceptsJSONNumber(t *testing.T) {
	if got := Int(json.Number("1")); got != 1 {
		t.Fatalf("Int(json.Number(\"1\")) = %d, want 1", got)
	}
	if got := Int(json.Number("1.0")); got != 1 {
		t.Fatalf("Int(json.Number(\"1.0\")) = %d, want 1", got)
	}
	if got := Int(json.Number("1.5")); got != 0 {
		t.Fatalf("Int(json.Number(\"1.5\")) = %d, want 0", got)
	}
}
