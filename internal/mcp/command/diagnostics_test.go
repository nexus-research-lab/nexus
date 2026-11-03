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
			wantReceipts := 0
			if action == ActionInvoke && tc.status == "ok" {
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
