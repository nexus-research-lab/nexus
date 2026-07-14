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

func TestRuntimeEnvPublishesExplicitVisionCapabilities(t *testing.T) {
	environment := runtimeEnvFromConfig(&RuntimeConfig{
		APIFormat: apiFormatChatCompletions,
		Model:     "vision-main",
		Vision:    true,
	}, runtimeKindNXS)
	if environment[nexusModelSupportsVisionEnvName] != "true" ||
		environment[nexusMultimodalUserContentEnvName] != "1" ||
		environment[nexusMultimodalToolResultEnvName] != "1" {
		t.Fatalf("vision capabilities = %#v", environment)
	}
	if _, exists := environment[nexusRemoteImageURLEnvName]; exists {
		t.Fatalf("compatible provider must explicitly declare remote URL support: %#v", environment)
	}
}

func TestVisionRuntimeEnvUsesIndependentNamespace(t *testing.T) {
	environment := visionRuntimeEnvFromConfig(&RuntimeConfig{
		APIFormat: apiFormatAnthropicMessages,
		AuthToken: "vision-token",
		BaseURL:   "https://vision.example.com",
		Model:     "vision-model",
		Provider:  "vision-provider",
		Vision:    true,
	})
	if environment["NEXUS_VISION_MODEL"] != "vision-model" || environment["NEXUS_VISION_API_KEY"] != "vision-token" {
		t.Fatalf("vision env = %#v", environment)
	}
	if _, exists := environment[anthropicModelEnvName]; exists {
		t.Fatalf("vision env polluted main provider namespace: %#v", environment)
	}
	if _, exists := environment["NEXUS_VISION_REMOTE_IMAGE_URL"]; exists {
		t.Fatalf("compatible vision provider must explicitly declare remote URL support: %#v", environment)
	}
}

func fakeRuntimeProfileEnv(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}
