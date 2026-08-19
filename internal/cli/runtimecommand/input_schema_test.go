package runtimecommand

import (
	"context"
	"strings"
	"testing"
)

func TestOperationInvokeRejectsMissingAndInvalidEnumBeforeHandler(t *testing.T) {
	invoked := false
	operation := Operation{
		Name: "review_work",
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"decision"},
			"properties": map[string]any{
				"decision": map[string]any{
					"type": "string",
					"enum": []string{"accepted", "rejected", "changes_requested"},
				},
			},
		},
		Handler: func(context.Context, map[string]any) (Result, error) {
			invoked = true
			return Result{}, nil
		},
	}

	for name, input := range map[string]map[string]any{
		"missing": {},
		"invalid": {"decision": "accept"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := operation.Invoke(context.Background(), input, nil)
			if err == nil || !strings.Contains(err.Error(), "$.decision") {
				t.Fatalf("Invoke() error = %v, want decision schema error", err)
			}
		})
	}
	if invoked {
		t.Fatal("invalid input reached domain handler")
	}
}

func TestValidateInputCoversPortableNestedSchema(t *testing.T) {
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"items"},
		"properties": map[string]any{
			"items": map[string]any{
				"type":     "array",
				"maxItems": 1,
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"label", "passed"},
					"properties": map[string]any{
						"label":  map[string]any{"type": "string", "pattern": `\S`},
						"passed": map[string]any{"type": "boolean"},
					},
				},
			},
		},
	}
	if err := ValidateInput(schema, map[string]any{
		"items": []any{map[string]any{"label": "ok", "passed": true}},
	}); err != nil {
		t.Fatalf("ValidateInput() error = %v", err)
	}
	if err := ValidateInput(schema, map[string]any{
		"items": []any{map[string]any{"label": "", "passed": true}},
	}); err == nil || !strings.Contains(err.Error(), "$[0]") && !strings.Contains(err.Error(), "$.items[0].label") {
		t.Fatalf("ValidateInput() error = %v, want nested path", err)
	}
	if err := ValidateInput(schema, map[string]any{"items": []any{}, "extra": true}); err == nil || !strings.Contains(err.Error(), "$.extra") {
		t.Fatalf("ValidateInput() error = %v, want unknown property", err)
	}
}

func TestValidateInputPreservesJSONSchemaAdditionalPropertiesDefault(t *testing.T) {
	t.Parallel()

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"known": map[string]any{"type": "string"},
		},
	}
	if err := ValidateInput(schema, map[string]any{
		"known": "value",
		"extra": true,
	}); err != nil {
		t.Fatalf("ValidateInput() error = %v, omitted additionalProperties must stay permissive", err)
	}
}
