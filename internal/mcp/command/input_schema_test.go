package command

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestInspectEntryCorrection(t *testing.T) {
	for domain, name := range map[string]string{"execution": "get_execution", "goal": "get_goal"} {
		t.Run(domain, func(t *testing.T) {
			calls := 0
			operations := []Operation{{Name: name, Handler: func(_ context.Context, input map[string]any) (Result, error) {
				calls++
				if input["execution_id"] != "historical" {
					t.Fatal("inspect lost input")
				}
				return Result{}, nil
			}}}
			tool := NewTool(func(ctx context.Context, request Request) (any, error) {
				return HandleSemantic(ctx, Actor{}, domain, name, operations, request)
			})
			for _, action := range []string{ActionInvoke, ActionInspect} {
				result, err := tool.Handler(context.Background(), map[string]any{
					"domain": domain, "action": action, "operation": name,
				})
				if err != nil || !result.IsError || calls != 0 {
					t.Fatalf("wrong entry executed: result=%+v err=%v calls=%d", result, err, calls)
				}
				message := result.Content[0]["text"].(string)
				start, end := strings.Index(message, "{"), strings.Index(message, "}")
				if start < 0 || end < start {
					t.Fatalf("missing correction: %s", message)
				}
				var corrected map[string]any
				if err := json.Unmarshal([]byte(message[start:end+1]), &corrected); err != nil {
					t.Fatal(err)
				}
				corrected["input"] = map[string]any{"execution_id": "historical"}
				result, err = tool.Handler(context.Background(), corrected)
				if err != nil || result.IsError || calls != 1 {
					t.Fatalf("correction failed: result=%+v err=%v calls=%d", result, err, calls)
				}
				calls = 0
			}
			for _, selected := range []string{"", name} {
				contract, err := BuildContract(domain, name, selected, operations)
				if err != nil || !strings.Contains(contract.Operations[0].Description, `"action":"inspect"`) {
					t.Fatalf("contract lacks inspect entry: %+v, %v", contract, err)
				}
			}
		})
	}
}

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

func TestValidateInputAcceptsBridgeJSONNumberIntegers(t *testing.T) {
	t.Parallel()

	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"revision", "nodes"},
		"properties": map[string]any{
			"revision": map[string]any{"type": "integer"},
			"nodes": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"position"},
					"properties": map[string]any{
						"position": map[string]any{"type": "integer"},
					},
				},
			},
		},
	}
	if err := ValidateInput(schema, map[string]any{
		"revision": json.Number("1"),
		"nodes":    []any{map[string]any{"position": json.Number("0")}},
	}); err != nil {
		t.Fatalf("ValidateInput() rejected bridge integer tokens: %v", err)
	}
	if err := ValidateInput(schema, map[string]any{
		"revision": json.Number("1.5"),
		"nodes":    []any{map[string]any{"position": json.Number("0")}},
	}); err == nil || !strings.Contains(err.Error(), "$.revision") {
		t.Fatalf("ValidateInput() error = %v, want fractional revision rejection", err)
	}
}
