package command

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestCommandRequestIDRejectsMalformedInputBeforeDispatch(t *testing.T) {
	for _, id := range []any{"", "submit", "提交-20260910", "submit work-001", "submit/work-001", strings.Repeat("x", 129), 12345678} {
		t.Run(fmt.Sprint(id), func(t *testing.T) {
			called := false
			tool := NewTool(func(context.Context, Request) (any, error) {
				called = true
				return nil, nil
			})
			result, err := tool.Handler(context.Background(), map[string]any{
				"domain": "execution", "action": "invoke", "operation": "submit_work", "request_id": id,
			})
			if err != nil || !result.IsError || called {
				t.Fatalf("invalid ID reached dispatch: result=%+v err=%v called=%v", result, err, called)
			}
			message := result.Content[0]["text"].(string)
			if !strings.Contains(message, "工具顶层") || !strings.Contains(message, "本次请求未执行") {
				t.Fatalf("missing actionable correction: %s", message)
			}
		})
	}
}

func TestSubmitWorkMissingRequestIDReturnsCorrectionBeforeOperation(t *testing.T) {
	called := false
	operations := []Operation{{Name: "submit_work", Handler: func(context.Context, map[string]any) (Result, error) {
		called = true
		return Result{}, nil
	}}}
	tool := NewTool(func(ctx context.Context, request Request) (any, error) {
		return HandleSemantic(ctx, Actor{}, "execution", "get_execution", operations, request)
	})
	result, err := tool.Handler(context.Background(), map[string]any{
		"domain": "execution", "action": "invoke", "operation": "submit_work", "input": map[string]any{},
	})
	if err != nil || !result.IsError || called || !strings.Contains(result.Content[0]["text"].(string), "补齐或修正 ID") {
		t.Fatalf("missing ID was not corrected before operation: %+v %v called=%v", result, err, called)
	}
	result, err = tool.Handler(context.Background(), map[string]any{
		"domain": "execution", "action": "invoke", "operation": "submit_work", "input": map[string]any{},
		"request_id": "submit-work-20260910-001",
	})
	if err != nil || result.IsError || !called {
		t.Fatalf("corrected ID did not reach operation: %+v %v called=%v", result, err, called)
	}
}

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

func TestCommandResultJSONShapes(t *testing.T) {
	for _, tc := range []struct {
		name       string
		value      any
		text       string
		structured string
	}{
		{"list", []map[string]any{{"id": "job-1"}}, `[{"id":"job-1"}]`, `{"result":[{"id":"job-1"}]}`},
		{"empty_list", []string{}, `[]`, `{"result":[]}`},
		{"nil_list", []string(nil), `null`, `{"result":null}`},
		{"object", struct {
			Total int `json:"total"`
		}{1}, `{"total":1}`, `{"total":1}`},
		{"scalar", true, `true`, `{"result":true}`},
		{"nil", nil, `null`, `{"result":null}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tool := NewTool(func(context.Context, Request) (any, error) { return tc.value, nil })
			result, err := tool.Handler(context.Background(), map[string]any{
				"domain": "automation", "action": "inspect", "operation": "list",
			})
			if err != nil || result.IsError {
				t.Fatalf("result = %+v, err = %v", result, err)
			}
			encoded, err := json.Marshal(result.StructuredContent)
			if err != nil || string(encoded) != tc.structured || len(result.Content) != 1 || result.Content[0]["text"] != tc.text {
				t.Fatalf("result = %+v, structured = %s, err = %v", result, encoded, err)
			}
		})
	}
}
