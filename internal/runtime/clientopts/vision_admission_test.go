// INPUT: Main and optional vision configurations, including stale/missing bindings.
// OUTPUT: Text startup independent of auxiliary vision, with exact environment isolation.
// POS: DM/Room shared runtime admission regression tests.
package clientopts

import (
	"context"
	"errors"
	"testing"
)

type visionTestResolver struct {
	main, auxiliary       *RuntimeConfig
	mainErr, auxiliaryErr error
}

func (r visionTestResolver) ResolveRuntimeConfig(_ context.Context, provider, _ string) (*RuntimeConfig, error) {
	if provider == "main" {
		return r.main, r.mainErr
	}
	return r.auxiliary, r.auxiliaryErr
}

func TestOptionalVisionDoesNotBlockMainChat(t *testing.T) {
	for _, mainVision := range []bool{false, true} {
		for _, tc := range []struct {
			name, provider, model string
			config                *RuntimeConfig
			err                   error
		}{
			{name: "unknown", provider: "custom", model: "kimi-k2.6", config: &RuntimeConfig{Model: "kimi-k2.6", APIFormat: apiFormatChatCompletions}},
			{name: "removed", provider: "custom", model: "kimi-k2.6", err: errors.New("provider no longer exists")},
			{name: "disabled", provider: "custom", model: "kimi-k2.6", err: errors.New("provider disabled")},
			{name: "partial", provider: "custom"},
			{name: "unset"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Setenv("NEXUS_VISION_MODEL", "inherited-model")
				t.Setenv("NEXUS_VISION_API_KEY", "inherited-key")
				resolver := visionTestResolver{main: &RuntimeConfig{Model: "main-model", Vision: mainVision, APIFormat: apiFormatChatCompletions, AuthToken: "main-key", BaseURL: "https://main.test"}, auxiliary: tc.config, auxiliaryErr: tc.err}
				options, err := BuildAgentClientOptions(context.Background(), resolver, AgentClientOptionsInput{RuntimeKind: "nxs", Provider: "main", Model: "main-model", VisionProvider: tc.provider, VisionModel: tc.model, ExtraEnv: map[string]string{"NEXUS_VISION_MODEL": "stale-model", "NEXUS_VISION_API_KEY": "stale-key"}})
				if err != nil {
					t.Fatal(err)
				}
				if options.Model != "main-model" {
					t.Fatalf("main model changed: %s", options.Model)
				}
				for _, key := range []string{"NEXUS_VISION_MODEL", "NEXUS_VISION_API_KEY", "NEXUS_VISION_BASE_URL"} {
					if options.Env[key] != "" {
						t.Fatalf("invalid auxiliary retained %s", key)
					}
				}
				if (options.Env[nexusModelSupportsVisionEnvName] == "true") != mainVision {
					t.Fatal("main vision capability changed")
				}
			})
		}
	}
}

func TestVisionAdmissionRetainsValidRouteAndMainFailures(t *testing.T) {
	resolver := visionTestResolver{main: &RuntimeConfig{Model: "main-model", APIFormat: apiFormatChatCompletions}, auxiliary: &RuntimeConfig{Provider: "vision", Model: "visual-model", Vision: true, APIFormat: apiFormatResponses, AuthToken: "visual-key", BaseURL: "https://vision.test"}}
	input := AgentClientOptionsInput{RuntimeKind: "nxs", Provider: "main", Model: "main-model", VisionProvider: "vision", VisionModel: "visual-model"}
	options, err := BuildAgentClientOptions(context.Background(), resolver, input)
	if err != nil || options.Env["NEXUS_VISION_MODEL"] != "visual-model" || options.Env["NEXUS_VISION_API_PROVIDER"] != "responses" {
		t.Fatalf("valid route lost: %v", err)
	}
	resolver.mainErr = errors.New("main model unavailable")
	if _, err = BuildAgentClientOptions(context.Background(), resolver, input); !errors.Is(err, resolver.mainErr) {
		t.Fatalf("main failure swallowed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resolver.mainErr = nil
	resolver.auxiliaryErr = ctx.Err()
	if _, err = BuildAgentClientOptions(ctx, resolver, input); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation swallowed: %v", err)
	}
}

// TestOptionalVisionPreservesSystemPrompt 验证缺失辅助视觉不再修改常驻提示词。
func TestOptionalVisionPreservesSystemPrompt(t *testing.T) {
	resolver := visionTestResolver{main: &RuntimeConfig{Model: "main-model", APIFormat: apiFormatChatCompletions}}
	input := AgentClientOptionsInput{RuntimeKind: "nxs", Provider: "main", Model: "main-model",
		AppendSystemPrompt: "combined", AppendSystemPromptStatic: "static", AppendSystemPromptDynamic: "dynamic"}
	options, err := BuildAgentClientOptions(context.Background(), resolver, input)
	if err != nil {
		t.Fatal(err)
	}
	if options.System.Append != input.AppendSystemPrompt || options.System.AppendStatic != input.AppendSystemPromptStatic || options.System.AppendDynamic != input.AppendSystemPromptDynamic {
		t.Fatal("optional vision changed system prompt")
	}
}
