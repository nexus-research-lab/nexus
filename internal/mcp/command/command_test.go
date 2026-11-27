// command_test.go 覆盖 MCP 命令 Actor、诊断、输入边界与输入 Schema 测试。
package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

func TestActorSubagentCommandAuthorityFollowsSourceBoundary(t *testing.T) {
	base := Actor{
		OwnerUserID: "owner", AgentID: "agent", SessionKey: "session", RoundID: "round",
		LeaseSessionKey: "session", LeaseRoundID: "round",
	}
	for _, test := range []struct {
		name   string
		source string
		want   bool
	}{
		{name: "ordinary agent", source: "agent", want: true},
		{name: "paired agent", source: "agent_paired", want: true},
		{name: "untrusted source", source: "agent_untrusted", want: false},
		{name: "ordinary room handoff", source: "room_handoff", want: false},
		{name: "hidden graph editor", source: "workgraph_editor", want: false},
		{name: "scheduled automation", source: "automation_run", want: false},
		{name: "workgraph execution", source: "workgraph_execution", want: true},
		{name: "room without physical lease context", source: "room", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			actor := base
			actor.SourceContextType = test.source
			if got := actor.SubagentCommandAuthority(); got != test.want {
				t.Fatalf("source %q authority = %t, want %t", test.source, got, test.want)
			}
		})
	}
}

func TestCommandDiagnosticsCoverResultsWithoutLoggingPayloads(t *testing.T) {
	for _, tc := range []struct {
		name, action, operation, requestID string
		outcome, reason, status, label     string
		err                                error
		isError, invalidInput              bool
	}{
		{name: "contract", action: ActionContract, status: "ok", label: "catalog", outcome: "unknown", reason: "not_reported"},
		{name: "inspect", action: ActionInspect, status: "ok", label: "get_execution", outcome: "unknown", reason: "not_reported"},
		{name: "applied", outcome: "applied", status: "ok", reason: "not_reported"},
		{name: "no_op", outcome: "no_op", status: "ok", reason: "not_reported"},
		{name: "rejected", outcome: "rejected", status: "ok", reason: "stale_execution"},
		{name: "superseded", outcome: "superseded", status: "ok", reason: "work_binding_mismatch"},
		{name: "missing_outcome", outcome: "unknown", status: "ok", reason: "other"},
		{name: "bad_envelope", requestID: "sensitive invalid request", outcome: "unknown", status: "error", label: "unknown", reason: "invalid_envelope"},
		{name: "missing_id", requestID: "<missing>", outcome: "unknown", status: "error", reason: "invalid_request_id"},
		{name: "unknown_operation", operation: "sensitive-operation-body", outcome: "unknown", status: "error", label: "unknown", reason: "handler_error"},
		{name: "operation_schema", invalidInput: true, outcome: "unknown", status: "error", reason: "invalid_input"},
		{name: "cancelled", err: fmt.Errorf("sensitive error: %w", context.Canceled), outcome: "unknown", status: "cancelled", reason: "cancelled"},
		{name: "deadline", err: context.DeadlineExceeded, outcome: "unknown", status: "deadline_exceeded", reason: "deadline_exceeded"},
		{name: "handler_failure", err: errors.New("sensitive-handler-error"), outcome: "unknown", status: "error", reason: "handler_error"},
		{name: "result_error", isError: true, outcome: "unknown", status: "error", reason: "result_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			actor := Actor{OwnerUserID: "sensitive-owner", AgentID: "sensitive-agent", SessionKey: "sensitive-session", LeaseRoundID: "sensitive-round"}
			actor.Round.CommandReceipts = nexusmcp.NewCommandReceiptState()
			operations := []Operation{{Name: "get_execution", Handler: func(context.Context, map[string]any) (Result, error) { return Result{}, nil }}, {
				Name: "submit_work", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"evidence": map[string]any{"type": "string"}}, "required": []string{"evidence"}},
				Handler: func(context.Context, map[string]any) (Result, error) {
					reason := tc.reason
					if reason == "other" {
						reason = "sensitive-reason-body"
					} else if reason == "not_reported" {
						reason = ""
					}
					return Result{IsError: tc.isError, StructuredContent: map[string]any{"outcome": tc.outcome, "reason_code": reason, "message": "sensitive-result-body"}}, tc.err
				},
			}}
			handler := func(ctx context.Context, request Request) (any, error) {
				return HandleSemantic(ctx, actor, DomainExecution, "get_execution", operations, request)
			}
			tool := NewTool(handler, NewDiagnostics(slog.New(slog.NewJSONHandler(&output, nil)), actor))
			action, operation, requestID := tc.action, tc.operation, tc.requestID
			if action == "" {
				action = ActionInvoke
				if operation == "" {
					operation = "submit_work"
				}
			}
			if requestID == "" {
				requestID = "sensitive-request-001"
			}
			input := map[string]any{"domain": DomainExecution, "action": action, "operation": operation, "input": map[string]any{"evidence": "sensitive-input-body"}}
			if requestID != "<missing>" {
				input["request_id"] = requestID
			}
			if tc.invalidInput {
				input["input"] = map[string]any{"evidence": 42}
			}
			result, err := tool.Handler(context.Background(), input)
			if err != nil || result.IsError != (tc.status != "ok") {
				t.Fatalf("diagnostics changed tool result: %+v, %v", result, err)
			}
			records := decodeDiagnosticLog(t, &output)
			if len(records) != 2 || records[0]["phase"] != "start" || records[1]["phase"] != "finish" || records[0]["call_id"] != records[1]["call_id"] {
				t.Fatalf("start/finish = %+v", records)
			}
			finish := records[1]
			label := tc.label
			if label == "" {
				label = "submit_work"
			}
			for key, want := range map[string]string{"status": tc.status, "outcome": tc.outcome, "reason_code": tc.reason, "operation": label} {
				if finish[key] != want {
					t.Fatalf("%s = %v, want %s", key, finish[key], want)
				}
			}
			if duration, ok := finish["host_duration_ms"].(float64); !ok || duration < 0 {
				t.Fatalf("duration = %v", finish["host_duration_ms"])
			}
			if strings.Contains(output.String(), "sensitive-") || strings.Contains(output.String(), "sensitive invalid") {
				t.Fatalf("diagnostic leaked raw identity or payload: %s", output.String())
			}
			receipts, _ := actor.Round.CommandReceipts.Since(0)
			// Every invoke that resolves an operation records one trusted receipt,
			// including failures, so indeterminate mutations remain auditable.
			// Envelope, request-identity and unknown-operation rejections never
			// reach an operation and record none.
			wantReceipts := 0
			if action == ActionInvoke && label != "unknown" && tc.reason != "invalid_request_id" {
				wantReceipts = 1
			}
			if len(receipts) != wantReceipts {
				t.Fatalf("diagnostics changed authoritative receipt count: %d, want %d", len(receipts), wantReceipts)
			}
		})
	}
}

func TestCommandDiagnosticsIncludeSDKEnvelopeRejection(t *testing.T) {
	var output bytes.Buffer
	called := false
	tool := NewTool(func(context.Context, Request) (any, error) { called = true; return nil, nil }, NewDiagnostics(slog.New(slog.NewJSONHandler(&output, nil)), Actor{}))
	server := sdktool.NewSimpleSDKMCPServer("nexus", "1", []sdktool.Tool{tool})
	_, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "command", "arguments": map[string]any{"domain": DomainExecution, "action": ActionInvoke, "request_id": 123}},
	})
	if err != nil || called {
		t.Fatalf("SDK envelope validation: dispatched=%t err=%v", called, err)
	}
	records := decodeDiagnosticLog(t, &output)
	if len(records) != 2 || records[1]["reason_code"] != "invalid_envelope" {
		t.Fatalf("SDK rejection missing from diagnostics: %+v", records)
	}
}

func TestCommandDiagnosticCorrelationIsScopedAndConcurrent(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	actor := Actor{OwnerUserID: "owner", AgentID: "agent", SessionKey: "session", LeaseRoundID: "round-1"}
	handler := func(context.Context, Request) (any, error) { return nil, nil }
	tool := NewTool(handler, NewDiagnostics(logger, actor))
	input := map[string]any{"domain": DomainExecution, "action": ActionInvoke, "request_id": "request-001"}
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() { _, _ = tool.Handler(context.Background(), input) })
	}
	wg.Wait()
	actor.LeaseRoundID = "round-2"
	_, _ = NewTool(handler, NewDiagnostics(logger, actor)).Handler(context.Background(), input)
	actor.SessionKey = "different-session"
	_, _ = NewTool(handler, NewDiagnostics(logger, actor)).Handler(context.Background(), input)
	records := decodeDiagnosticLog(t, &output)
	if len(records) != 44 {
		t.Fatalf("records = %d, want 44", len(records))
	}
	counts := map[string]int{}
	for _, record := range records {
		counts[record["call_id"].(string)]++
	}
	if len(counts) != 22 {
		t.Fatalf("distinct calls = %d, want 22", len(counts))
	}
	for id, count := range counts {
		if count != 2 {
			t.Fatalf("call %s has %d records", id, count)
		}
	}
	if records[0]["request_hash"] != records[40]["request_hash"] || records[0]["round_hash"] == records[40]["round_hash"] || records[40]["request_hash"] == records[42]["request_hash"] {
		t.Fatal("request correlation must span rounds but isolate sessions")
	}
}

type failingDiagnosticWriter struct{ panicOnWrite bool }

func (w failingDiagnosticWriter) Write([]byte) (int, error) {
	if w.panicOnWrite {
		panic("diagnostic sink failed")
	}
	return 0, errors.New("diagnostic sink failed")
}

func TestCommandDiagnosticFailureCannotChangeDomainResult(t *testing.T) {
	for _, panicOnWrite := range []bool{false, true} {
		tool := NewTool(func(context.Context, Request) (any, error) {
			return Result{StructuredContent: map[string]any{"outcome": "applied"}}, nil
		}, NewDiagnostics(slog.New(slog.NewJSONHandler(failingDiagnosticWriter{panicOnWrite}, nil)), Actor{}))
		result, err := tool.Handler(context.Background(), map[string]any{"domain": DomainExecution, "action": ActionInspect})
		if err != nil || result.IsError || result.StructuredContent["outcome"] != "applied" {
			t.Fatalf("diagnostic failure changed result: %+v, %v", result, err)
		}
	}
	var output bytes.Buffer
	tool := NewTool(func(context.Context, Request) (any, error) { panic("domain panic") }, NewDiagnostics(slog.New(slog.NewJSONHandler(&output, nil)), Actor{}))
	func() {
		defer func() {
			if recovered := recover(); recovered != "domain panic" {
				t.Fatalf("domain panic changed: %v", recovered)
			}
		}()
		_, _ = tool.Handler(context.Background(), map[string]any{"domain": DomainExecution, "action": ActionInspect})
	}()
	records := decodeDiagnosticLog(t, &output)
	if records[1]["status"] != "incomplete" || records[1]["outcome"] != "unknown" {
		t.Fatalf("panicked call was marked successful: %+v", records[1])
	}
}

func decodeDiagnosticLog(t *testing.T, output *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(output.Bytes()), []byte{'\n'}) {
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	return records
}

// Measures only the in-process command wrapper and JSON diagnostic encoding;
// there is no database, file sink, provider, token usage or end-to-end workload.
func BenchmarkCommandDiagnostics(b *testing.B) {
	for _, enabled := range []bool{false, true} {
		name := "disabled"
		var diagnostics *Diagnostics
		if enabled {
			name = "json_to_discard"
			diagnostics = NewDiagnostics(slog.New(slog.NewJSONHandler(io.Discard, nil)), Actor{OwnerUserID: "benchmark-owner", AgentID: "agent", SessionKey: "benchmark-session", LeaseRoundID: "round"})
		}
		b.Run(name, func(b *testing.B) {
			result := Result{StructuredContent: map[string]any{"outcome": "no_op"}}
			tool := NewTool(func(context.Context, Request) (any, error) { return result, nil }, diagnostics)
			input := map[string]any{"domain": DomainExecution, "action": ActionInvoke, "operation": "assign_work_batch", "request_id": "benchmark-request-001"}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := tool.Handler(context.Background(), input); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

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

func TestOperationInputReportsAllErrorsAndSchemaBeforeExecuting(t *testing.T) {
	schema := map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"result_summary"},
		"properties": map[string]any{
			"result_summary": map[string]any{"type": "string"},
			"evidence":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
	}
	called := false
	operation := Operation{Name: "submit_work", InputSchema: schema, Handler: func(context.Context, map[string]any) (Result, error) { called = true; return Result{}, nil }}
	tool := NewTool(func(ctx context.Context, request Request) (any, error) {
		return HandleSemantic(ctx, Actor{}, "execution", "get_execution", []Operation{operation}, request)
	})
	var previous string
	for range 10 {
		result, err := tool.Handler(context.Background(), map[string]any{
			"domain": "execution", "action": "invoke", "operation": "submit_work", "request_id": "submit-work-0001",
			"input": map[string]any{"content": "private supplied content", "result": "secret", "evidence": []any{true, 42}},
		})
		if err != nil || !result.IsError || called {
			t.Fatalf("invalid request executed: %+v %v", result, err)
		}
		message := result.Content[0]["text"].(string)
		for _, field := range []string{"$.result_summary", "$.content", "$.result", "$.evidence[0]", "$.evidence[1]"} {
			if !strings.Contains(message, field) {
				t.Fatalf("missing %s in %s", field, message)
			}
		}
		if strings.Contains(message, "secret") || strings.Contains(message, "private supplied content") {
			t.Fatal("supplied values leaked")
		}
		if previous != "" && previous != message {
			t.Fatal("field error order changed")
		}
		previous = message
		encoded, _ := json.Marshal(result.StructuredContent)
		if !strings.Contains(string(encoded), `"executed":false`) || !strings.Contains(string(encoded), `"input_schema"`) {
			t.Fatalf("missing correction contract: %s", encoded)
		}
	}
}
