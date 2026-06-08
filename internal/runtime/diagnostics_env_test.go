package runtime

import "testing"

func TestAgentSDKDiagnosticsEnabledUsesExplicitRuntimeEnv(t *testing.T) {
	t.Setenv(AgentSDKDiagnosticsEnvName, "1")
	env := map[string]string{AgentSDKDiagnosticsEnvName: ""}

	if AgentSDKDiagnosticsEnabled(env) {
		t.Fatalf("显式 runtime env 空值应覆盖进程环境")
	}
}

func TestAgentSDKDiagnosticsEnabledIgnoresProcessEnv(t *testing.T) {
	t.Setenv(AgentSDKDiagnosticsEnvName, "stderr")

	if AgentSDKDiagnosticsEnabled(nil) {
		t.Fatalf("未显式传入 runtime env 时不应读取进程环境")
	}
}

func TestAgentSDKProviderDebugBodyValueUsesRuntimeEnv(t *testing.T) {
	t.Setenv(AgentSDKProviderDebugBodyEnvName, "full")
	env := map[string]string{AgentSDKProviderDebugBodyEnvName: "16384"}

	if got := AgentSDKProviderDebugBodyValue(env); got != "16384" {
		t.Fatalf("AgentSDKProviderDebugBodyValue() = %q, want 16384", got)
	}
}
