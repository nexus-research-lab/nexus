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

func TestScrubInheritedRuntimeEnvRemovesControlAndRelayConfiguration(t *testing.T) {
	keys := []string{
		"CONTROL_SERVICE_TOKEN",
		"CONTROL_SETUP_TOKEN",
		"CONTROL_SIGNING_PRIVATE_KEY",
		"NEXUS_CONTROL_SERVICE_TOKEN",
		"NEXUS_CONTROL_SERVICE_TOKEN_FILE",
		"NEXUS_REMOTE_URL",
		"NEXUS_RELAY_URL",
		"NEXUS_RELAY_REQUEST_TIMEOUT_SECONDS",
		"RELAY_DATABASE_DRIVER",
		"RELAY_DATABASE_URL",
		"RELAY_DEPLOYMENT_ID",
		"RELAY_PRINCIPAL_PUBLIC_KEY",
		"RELAY_PRINCIPAL_PUBLIC_KEY_FILE",
	}
	for _, key := range keys {
		t.Setenv(key, "must-not-reach-runtime")
	}
	values := scrubInheritedRuntimeEnv()
	for _, key := range keys {
		if value, ok := values[key]; !ok || value != "" {
			t.Fatalf("%s = %q, present = %t", key, value, ok)
		}
	}
}

func TestScrubInheritedRuntimeEnvRemovesProviderAndHelperSecrets(t *testing.T) {
	keys := []string{
		"NEXUS_API_KEY_FILE_DESCRIPTOR",
		"NEXUS_OAUTH_TOKEN_FILE_DESCRIPTOR",
		"NEXUS_CLIENT_KEY",
		"NEXUS_CLIENT_KEY_PASSPHRASE",
		"ANTHROPIC_CUSTOM_HEADERS",
		"ANTHROPIC_FOUNDRY_API_KEY",
		"CLAUDE_CODE_OAUTH_TOKEN",
		"AWS_BEARER_TOKEN_BEDROCK",
		"AZURE_CLIENT_SECRET",
		"GOOGLE_APPLICATION_CREDENTIALS",
		"OPENAI_CUSTOM_HEADERS",
		"NEXUS_WEBSEARCH_API_KEY",
		"NEXUS_WEBFETCH_SUMMARIZER_API_KEY",
		"OTEL_EXPORTER_OTLP_HEADERS",
		"SSH_AUTH_SOCK",
		"CONNECTOR_GITHUB_CLIENT_SECRET",
	}
	for _, key := range keys {
		t.Setenv(key, "host-secret-or-handle")
	}

	values := scrubInheritedRuntimeEnv()
	for _, key := range keys {
		if value, ok := values[key]; !ok || value != "" {
			t.Fatalf("%s = %q, present = %t", key, value, ok)
		}
	}
}

func TestScrubInheritedRuntimeEnvDoesNotCreateAbsentOverrides(t *testing.T) {
	for _, key := range []string{
		"NEXUS_API_KEY_FILE_DESCRIPTOR",
		"ANTHROPIC_CUSTOM_HEADERS",
		"GOOGLE_APPLICATION_CREDENTIALS",
		"OTEL_EXPORTER_OTLP_HEADERS",
	} {
		t.Setenv(key, "")
	}

	values := scrubInheritedRuntimeEnv()
	for _, key := range []string{
		"NEXUS_API_KEY_FILE_DESCRIPTOR",
		"ANTHROPIC_CUSTOM_HEADERS",
		"GOOGLE_APPLICATION_CREDENTIALS",
		"OTEL_EXPORTER_OTLP_HEADERS",
	} {
		if _, ok := values[key]; ok {
			t.Fatalf("empty host variable %s should not create an explicit runtime override", key)
		}
	}
}
