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
	if environment[nexusOpenAIProtocolEnvName] != apiFormatChatCompletions {
		t.Fatalf("OpenAI protocol = %q, want %q", environment[nexusOpenAIProtocolEnvName], apiFormatChatCompletions)
	}
}

func TestRuntimeEnvPublishesResponsesProtocolAndCapabilities(t *testing.T) {
	environment := runtimeEnvFromConfig(&RuntimeConfig{
		APIFormat: apiFormatResponses,
		Model:     "responses-main",
		Vision:    true,
	}, runtimeKindNXS)
	if environment[nexusAPIProviderEnvName] != "openai" ||
		environment[nexusOpenAIProtocolEnvName] != apiFormatResponses {
		t.Fatalf("Responses route = %#v", environment)
	}
	if environment[nexusModelSupportsVisionEnvName] != "true" ||
		environment[nexusMultimodalUserContentEnvName] != "1" ||
		environment[nexusMultimodalToolResultEnvName] != "1" {
		t.Fatalf("Responses vision capabilities = %#v", environment)
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

func TestVisionRuntimeEnvSelectsResponsesProtocol(t *testing.T) {
	environment := visionRuntimeEnvFromConfig(&RuntimeConfig{
		APIFormat: apiFormatResponses,
		AuthToken: "vision-token",
		BaseURL:   "https://vision.example.com/v1",
		Model:     "vision-responses-model",
		Provider:  "vision-responses-provider",
		Vision:    true,
	})
	if environment["NEXUS_VISION_API_PROVIDER"] != "responses" {
		t.Fatalf("vision provider = %q, want responses; env=%#v", environment["NEXUS_VISION_API_PROVIDER"], environment)
	}
}

func fakeRuntimeProfileEnv(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}
