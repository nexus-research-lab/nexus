package clientopts

import (
	"reflect"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

func TestSanitizeRuntimeDiagnosticAttributesSummarizesProcessArgs(t *testing.T) {
	got := SanitizeRuntimeDiagnosticAttributes("process_start", map[string]any{
		"args": []string{
			"--append-system-prompt",
			"prompt text must not enter logs",
			"--model=glm-4.5",
			"--mcp-config",
			"/tmp/mcp.json",
			"positional",
		},
		"stdout_prefix": "raw stdout prefix",
		"stdout_suffix": "raw stdout suffix",
		"command_path":  "/usr/local/bin/claude",
	})
	if _, ok := got["stdout_prefix"]; ok {
		t.Fatalf("stdout_prefix should be dropped: %+v", got)
	}
	if _, ok := got["stdout_suffix"]; ok {
		t.Fatalf("stdout_suffix should be dropped: %+v", got)
	}
	summary, ok := got["args_summary"].(map[string]any)
	if !ok {
		t.Fatalf("args_summary type = %T, want map[string]any", got["args_summary"])
	}
	if summary["count"] != 6 {
		t.Fatalf("args_summary.count = %v, want 6", summary["count"])
	}
	wantFlags := []string{"--append-system-prompt", "--model", "--mcp-config"}
	if !reflect.DeepEqual(summary["flags"], wantFlags) {
		t.Fatalf("args_summary.flags = %+v, want %+v", summary["flags"], wantFlags)
	}
	if got["command_path"] != "/usr/local/bin/claude" {
		t.Fatalf("command_path = %v", got["command_path"])
	}
}

func TestRuntimeStartupDiagnosticFilters(t *testing.T) {
	if !ShouldLogRuntimeStartupDiagnostic(agentclient.DiagnosticEvent{
		Component: "bridge.process",
		Event:     "process_start",
	}) {
		t.Fatal("process_start should be logged by default")
	}
	if !ShouldWarnRuntimeStartupDiagnostic(agentclient.DiagnosticEvent{
		Component:  "bridge.process",
		Event:      "process_exit",
		Attributes: map[string]any{"error": "exit status 1"},
	}) {
		t.Fatal("process_exit with error should warn by default")
	}
	if ShouldWarnRuntimeStartupDiagnostic(agentclient.DiagnosticEvent{
		Component: "bridge.process",
		Event:     "process_exit",
	}) {
		t.Fatal("process_exit without error should stay quiet by default")
	}
}
