package clientopts

import "testing"

func TestResolveRuntimeKindDefaultsToNXS(t *testing.T) {
	if got := resolveRuntimeKind("", fakeRuntimeProfileEnv(nil)); got != runtimeKindNXS {
		t.Fatalf("runtime kind = %q, want %q", got, runtimeKindNXS)
	}
}

func TestResolveRuntimeKindAllowsEnvOverrideToClaude(t *testing.T) {
	got := resolveRuntimeKind(runtimeKindNXS, fakeRuntimeProfileEnv(map[string]string{
		nexusAgentRuntimeKindEnvName: "claude",
	}))
	if got != runtimeKindClaude {
		t.Fatalf("runtime kind = %q, want %q", got, runtimeKindClaude)
	}
}

func fakeRuntimeProfileEnv(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}
