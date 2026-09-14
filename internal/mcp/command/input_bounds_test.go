package command

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestSchemaBoundsRejectBeforeOperation(t *testing.T) {
	schema := map[string]any{"type": "object", "properties": map[string]any{
		"revision": map[string]any{"type": "integer", "minimum": 1, "maximum": 3},
		"nodes":    map[string]any{"type": "array", "minItems": 1, "maxItems": 2},
		"title":    map[string]any{"type": "string", "minLength": 2, "maxLength": 3},
	}}
	for _, test := range []struct {
		field string
		value any
	}{
		{"revision", 0}, {"revision", int64(4)}, {"revision", json.Number("-1")},
		{"nodes", []any{}}, {"nodes", []any{1, 2, 3}}, {"title", "中"}, {"title", "中文字符"},
	} {
		called := false
		op := Operation{Name: "bounded", InputSchema: schema, Handler: func(_ context.Context, _ map[string]any) (Result, error) { called = true; return Result{}, nil }}
		_, err := op.Invoke(context.Background(), map[string]any{test.field: test.value}, nil)
		if err == nil || called || !strings.Contains(err.Error(), "$."+test.field) {
			t.Fatalf("%s=%v: %v called=%v", test.field, test.value, err, called)
		}
	}
	for _, revision := range []any{1, int64(3), json.Number("2"), float64(2)} {
		if err := ValidateInput(schema, map[string]any{"revision": revision, "nodes": []any{1}, "title": "中文"}); err != nil {
			t.Fatal(err)
		}
	}
}
