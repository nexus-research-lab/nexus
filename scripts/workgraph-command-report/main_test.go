package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/command"
)

func TestReportConsumesCommandDiagnosticsAndKeepsMissingFinishUnknown(t *testing.T) {
	var logs bytes.Buffer
	callCount := 0
	tool := command.NewTool(func(ctx context.Context, request command.Request) (any, error) {
		operations := []command.Operation{{Name: "assign_work_batch", Handler: func(context.Context, map[string]any) (command.Result, error) {
			callCount++
			outcome := "applied"
			if callCount == 2 {
				outcome = "no_op"
			}
			return command.Result{StructuredContent: map[string]any{"outcome": outcome}}, nil
		}}}
		return command.HandleSemantic(ctx, command.Actor{}, command.DomainExecution, "", operations, request)
	}, command.NewDiagnostics(slog.New(slog.NewJSONHandler(&logs, nil)), command.Actor{OwnerUserID: "owner", SessionKey: "session", AgentID: "agent"}))
	for _, id := range []string{"request-001", "request-001", "request-002"} {
		if _, err := tool.Handler(context.Background(), map[string]any{"domain": "execution", "action": "invoke", "operation": "assign_work_batch", "request_id": id}); err != nil {
			t.Fatal(err)
		}
	}
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	// The last call's finish is deliberately unavailable. Copy a rotated log
	// record as well: repeated log records must not inflate repeated calls.
	input := strings.Join(append(lines[:5], lines[1]), "\n")
	file := filepath.Join(t.TempDir(), "synthetic.jsonl")
	if err := os.WriteFile(file, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run([]string{"--label", "synthetic-collector-test", "--environment", "in-process fake domain, no provider", file}, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	var result report
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Calls != 3 || result.Incomplete != 1 || result.RepeatedRequests != 1 || result.DuplicateLogRecords != 1 || result.PerToolTokens != nil {
		t.Fatalf("report = %+v", result)
	}
	counts := map[string]int{}
	for _, group := range result.Groups {
		counts[group.Outcome] += group.Calls
		if group.Outcome == "unknown" && (group.HostMS.Samples != 0 || group.HostMS.P95 != nil || group.Status != "incomplete") {
			t.Fatalf("missing finish invented a result or duration: %+v", group)
		}
		if group.Outcome == "applied" && (group.Operation != "assign_work_batch" || group.HostMS.Samples != 1) {
			t.Fatalf("registered operation or duration lost: %+v", group)
		}
	}
	if counts["applied"] != 1 || counts["no_op"] != 1 || counts["unknown"] != 1 {
		t.Fatalf("outcomes = %+v", counts)
	}
}

func TestReportCountsCorruptionAndDoesNotTrustConflictingRecords(t *testing.T) {
	base := diagnosticRecord{Event: "nexus.command.call", SchemaVersion: 1, CallID: "call-1", Phase: "finish", Domain: "execution", Action: "invoke", Operation: "submit_work", Status: "ok", Outcome: "applied"}
	var input bytes.Buffer
	encoder := json.NewEncoder(&input)
	for _, record := range []diagnosticRecord{base, base} {
		if err := encoder.Encode(record); err != nil {
			t.Fatal(err)
		}
	}
	base.Outcome = "rejected"
	_ = encoder.Encode(base)
	base.CallID, base.SchemaVersion = "future-call", 2
	_ = encoder.Encode(base)
	input.WriteString("not json\n{\"event\":\"unrelated\"}\n")
	var c collector
	if err := c.read(&input); err != nil {
		t.Fatal(err)
	}
	r := c.report("synthetic-corruption", "fixture")
	if r.Calls != 1 || r.MissingStart != 1 || r.Conflicting != 1 || r.MalformedLines != 1 || r.UnsupportedRecords != 1 || r.DuplicateLogRecords != 1 {
		t.Fatalf("corrupt report = %+v", r)
	}
	if len(r.Groups) != 1 || r.Groups[0].Outcome != "unknown" || r.Groups[0].Status != "unknown" || r.Groups[0].HostMS.Mean != nil {
		t.Fatalf("conflict reported as success: %+v", r.Groups)
	}
}

func TestReportRequiresExperimentMetadataAndPreservesEmptyDurations(t *testing.T) {
	if err := run(nil, io.Discard, io.Discard); err == nil {
		t.Fatal("accepted an unlabeled measurement")
	}
	if empty := summarizeDurations(nil); empty.Samples != 0 || empty.Mean != nil || empty.P50 != nil || empty.P95 != nil {
		t.Fatalf("empty durations = %+v", empty)
	}
	values := summarizeDurations([]float64{4, 1, 3, 2})
	if values.Samples != 4 || *values.Mean != 2.5 || *values.P50 != 2 || *values.P95 != 4 {
		t.Fatalf("duration summary = %+v", values)
	}
}
