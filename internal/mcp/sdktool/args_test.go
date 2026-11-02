package sdktool

import (
	"reflect"
	"testing"
)

func TestArgsTrimOnceAndTreatWrongTypesAsMissing(t *testing.T) {
	args := map[string]any{
		"name":       "  alice  ",
		"number":     42,
		"recipients": []any{" a ", "", "b", "a", 7},
		"bad_list":   "a,b",
		"extras":     map[string]any{"k": "v", "n": 1},
		"object":     map[string]any{"x": 1},
		"whole":      float64(3),
		"fraction":   1.5,
	}
	if got := StringArg(args, "name"); got != "alice" {
		t.Fatalf("StringArg trimmed = %q", got)
	}
	if got := StringArg(args, "number"); got != "" {
		t.Fatalf("StringArg must not stringify numbers: %q", got)
	}
	if got := StringArg(nil, "name"); got != "" {
		t.Fatalf("StringArg(nil) = %q", got)
	}
	if got := StringListArg(args, "recipients"); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("StringListArg = %#v", got)
	}
	if got := StringListArg(args, "missing"); got != nil {
		t.Fatalf("missing list must be nil: %#v", got)
	}
	if got := StringListArg(args, "bad_list"); got == nil || len(got) != 0 {
		t.Fatalf("present non-list must be empty, not nil: %#v", got)
	}
	if got := StringMapArg(args, "extras"); !reflect.DeepEqual(got, map[string]string{"k": "v"}) {
		t.Fatalf("StringMapArg = %#v", got)
	}
	if got := ObjectArg(args, "object"); got["x"] != 1 {
		t.Fatalf("ObjectArg = %#v", got)
	}
	if got := IntArg(args, "whole"); got != 3 {
		t.Fatalf("IntArg whole = %d", got)
	}
	if got := IntArg(args, "fraction"); got != 0 {
		t.Fatalf("IntArg must reject fractions: %d", got)
	}
}
