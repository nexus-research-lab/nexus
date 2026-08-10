package runtime

import "testing"

func TestAgentSDKDiagnosticsEnabledIgnoresProcessEnv(t *testing.T) {
	t.Setenv(AgentSDKDiagnosticsEnvName, "stderr")

	if AgentSDKDiagnosticsEnabled(nil) {
		t.Fatalf("未显式传入 runtime env 时不应读取进程环境")
	}
}

func TestAgentSDKDiagnosticsEnabledUsesJSONLEnv(t *testing.T) {
	env := map[string]string{AgentSDKDiagnosticsJSONLEnvName: "1"}

	if !AgentSDKDiagnosticsEnabled(env) {
		t.Fatalf("JSONL runtime env 应开启诊断")
	}
	if got := AgentSDKDiagnosticsValue(env); got != "jsonl" {
		t.Fatalf("AgentSDKDiagnosticsValue() = %q, want jsonl", got)
	}
}

func TestNormalizeRuntimeStderrLine(t *testing.T) {
	for _, test := range []struct {
		name string
		line string
		want string
	}{
		{name: "GBK", line: string([]byte{0xc3, 0xfc, 0xc1, 0xee, 0xd0, 0xd0, 0xcc, 0xab, 0xb3, 0xa4, 0xa1, 0xa3}), want: "命令行太长。"},
		{name: "UTF-8", line: "  process failed  ", want: "process failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := NormalizeRuntimeStderrLine(test.line); got != test.want {
				t.Fatalf("NormalizeRuntimeStderrLine() = %q, want %q", got, test.want)
			}
		})
	}
}
