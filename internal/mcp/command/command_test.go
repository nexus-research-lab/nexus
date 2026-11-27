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
	"testing"

	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
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
