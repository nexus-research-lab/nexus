package clientopts

import (
	"context"
	"testing"

	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func TestBuildAgentClientOptionsProjectsDiagnosticsForBothRuntimes(t *testing.T) {
	t.Setenv(runtimectx.AgentSDKDiagnosticsJSONLEnvName, "")
	for _, test := range []struct {
		name    string
		kind    string
		enabled bool
	}{
		{"Claude enabled", runtimeKindClaude, true},
		{"Claude disabled", runtimeKindClaude, false},
		{"nxs enabled", runtimeKindNXS, true},
		{"nxs disabled", runtimeKindNXS, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
				RuntimeKind:                test.kind,
				AgentSDKDiagnosticsEnabled: test.enabled,
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := runtimectx.AgentSDKDiagnosticsEnabled(options.Env); got != test.enabled {
				t.Fatalf("diagnostics enabled = %t, want %t", got, test.enabled)
			}
			_, hasBridgeFlag := options.Env[runtimectx.AgentSDKDiagnosticsEnvName]
			if wantBridgeFlag := test.enabled && test.kind == runtimeKindClaude; hasBridgeFlag != wantBridgeFlag {
				t.Fatalf("Claude diagnostics flag present = %t, want %t", hasBridgeFlag, wantBridgeFlag)
			}
			_, hasJSONL := options.Env[runtimectx.AgentSDKDiagnosticsJSONLEnvName]
			if wantJSONL := test.enabled && test.kind == runtimeKindNXS; hasJSONL != wantJSONL {
				t.Fatalf("nxs JSONL diagnostics present = %t, want %t", hasJSONL, wantJSONL)
			}
		})
	}
}
