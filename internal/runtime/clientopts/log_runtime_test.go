package clientopts

import (
	"reflect"
	"strings"
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

func TestRuntimeStartupLogFieldsUsesBridgeSnapshot(t *testing.T) {
	fields := RuntimeStartupLogFields(agentclient.NewOptions().
		WithCLIPath("claude").
		WithCWD("/workspace").
		WithSystemPrompt("secret prompt").
		WithEnv(map[string]string{
			"ANTHROPIC_AUTH_TOKEN": "secret-token",
			"ANTHROPIC_MODEL":      "glm-4.5-air",
		}))
	values := logFieldMap(fields)
	if values["command_path"] != "claude" {
		t.Fatalf("command_path = %v", values["command_path"])
	}
	if values["cwd"] != "/workspace" {
		t.Fatalf("cwd = %v", values["cwd"])
	}
	if values["runtime_env_fingerprint"] == "" || values["options_fingerprint"] == "" {
		t.Fatalf("fingerprints missing: %+v", values)
	}
	envKeys, ok := values["runtime_env_keys"].([]string)
	if !ok || !stringSliceContainsForLogTest(envKeys, "ANTHROPIC_AUTH_TOKEN") {
		t.Fatalf("runtime_env_keys = %+v", values["runtime_env_keys"])
	}
	args, ok := values["runtime_args"].([]string)
	if !ok || !stringSliceContainsForLogTest(args, "<redacted>") {
		t.Fatalf("runtime_args = %+v", values["runtime_args"])
	}
	raw := fieldsToString(fields)
	for _, forbidden := range []string{"secret-token", "secret prompt"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("startup log fields leaked %q: %s", forbidden, raw)
		}
	}
}

func TestRuntimeStartupLogFieldsSkipsSnapshotForUnresolvedNXS(t *testing.T) {
	t.Setenv(nexusNXSCommandPathEnvName, "")

	fields := RuntimeStartupLogFields(agentclient.NewOptions().
		WithRuntime(agentclient.RuntimeNXS).
		WithCWD("/workspace"))
	values := logFieldMap(fields)
	if values["cwd"] != "/workspace" {
		t.Fatalf("cwd = %v", values["cwd"])
	}
	for _, key := range []string{
		"command_path",
		"runtime_args",
		"runtime_env_fingerprint",
		"options_fingerprint",
		"launch_snapshot_error",
	} {
		if _, ok := values[key]; ok {
			t.Fatalf("field %q should not be present for unresolved nxs: %+v", key, values)
		}
	}
}

func logFieldMap(fields []any) map[string]any {
	result := map[string]any{}
	for index := 0; index+1 < len(fields); index += 2 {
		key, ok := fields[index].(string)
		if !ok {
			continue
		}
		result[key] = fields[index+1]
	}
	return result
}

func fieldsToString(fields []any) string {
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, strings.TrimSpace(toLogTestString(field)))
	}
	return strings.Join(parts, " ")
}

func toLogTestString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []string:
		return strings.Join(typed, " ")
	default:
		return ""
	}
}

func stringSliceContainsForLogTest(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
