// INPUT: Official preset/model identities, stored facts and explicit overrides.
// OUTPUT: Regression coverage for recommendation scope, modalities and provenance.
// POS: Provider guidance contract tests; no live credentials or provider calls.
package provider

import (
	"net/url"
	"strings"
	"testing"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

func TestOfficialModelAdvice(t *testing.T) {
	for _, tc := range []struct {
		preset, id                  string
		recommend, vision, textOnly bool
	}{
		{presetGLMCodingPlan, "glm-5.3", true, false, true},
		{presetGLMCodingPlan, "glm-5.3-flash", true, true, false},
		{presetGLMCodingPlan, "glm-5.1", false, false, true},
		{presetGLMCodingPlan, "glm-5.4", false, false, false},
		{presetMiniMaxToken, "MiniMax-M3", true, true, false},
		{presetMiniMaxToken, "MiniMax-M2.7", false, false, true},
		{presetKimiCode, "k3-256k", true, true, false},
		{presetKimiCode, "kimi-for-coding", true, true, false},
		{presetQwenTokenPlan, "qwen3.8-max", true, true, false},
		{presetQwenTokenPlan, "deepseek-v4-flash", false, false, true},
		{presetDeepSeek, "deepseek-flash", true, true, false},
		{presetDeepSeek, "deepseek-chat", false, false, false},
		{presetOpenAI, "gpt-6-astra", true, true, false},
		{presetAnthropic, "claude-opus-5", true, true, false},
		{presetVolcengine, "glm-5.3-flash", true, true, false},
		{presetDoubao, "doubao-seed-2-1-pro-260915", true, true, false},
		{presetAzure, "gpt-6-astra", false, false, false},
		{presetCustom, "gpt-6-astra", false, false, false},
		{presetOpenAI, "other/gpt-6-astra", false, false, false},
	} {
		t.Run(tc.preset+"/"+tc.id, func(t *testing.T) {
			g := projectModelGuidance(providerstore.Entity{PresetKey: tc.preset, ProviderKind: ProviderKindLLM}, providerstore.ModelEntity{ModelID: tc.id})
			if (g.Recommendations[PurposeChat] != "") != tc.recommend || g.Eligibility[PurposeVision].Available != tc.vision || g.TextOnly != tc.textOnly {
				t.Fatalf("unexpected guidance: %+v", g)
			}
			if g.Eligibility[PurposeImage].Available {
				t.Fatal("chat/image input must not grant image generation")
			}
		})
	}
}

func TestAdviceAutomaticDenialsAndUserOverride(t *testing.T) {
	item := providerstore.Entity{PresetKey: presetGLMCodingPlan, ProviderKind: ProviderKindLLM}
	model := providerstore.ModelEntity{ModelID: "glm-5.3", CapabilitiesAutoJSON: encodeModelAutoCapabilities(ModelCapabilities{Vision: adviceBool(true)})}
	if projectModelGuidance(item, model).Eligibility[PurposeVision].Available {
		t.Fatal("automatic positive overrode documented denial")
	}
	model.CapabilitiesOverrideJSON = `{"vision":true}`
	if !projectModelGuidance(item, model).Eligibility[PurposeVision].Available {
		t.Fatal("explicit user override lost")
	}
	model.ModelID = "glm-5.3-flash"
	model.CapabilitiesOverrideJSON = "{}"
	model.CapabilitiesAutoJSON = encodeModelAutoCapabilities(ModelCapabilities{Vision: adviceBool(false)})
	g := projectModelGuidance(item, model)
	if g.Eligibility[PurposeVision].Available || g.Sources["vision"] != "provider_record" || g.TextOnly {
		t.Fatalf("remote denial lost or false text-only label: %+v", g)
	}
}

func TestHistoricalGuessesRequireRediscovery(t *testing.T) {
	item := providerstore.Entity{PresetKey: presetCustom, ProviderKind: ProviderKindLLM}
	model := providerstore.ModelEntity{ModelID: "gemini-future", CapabilitiesAutoJSON: `{"vision":true,"reasoning":true}`}
	g := projectModelGuidance(item, model)
	if g.Capabilities.Vision != nil || g.Capabilities.Reasoning != nil {
		t.Fatal("unverified historical positives still trusted")
	}
	model.CapabilitiesAutoJSON = encodeModelAutoCapabilities(ModelCapabilities{Vision: adviceBool(true)})
	if !projectModelGuidance(item, model).Eligibility[PurposeVision].Available {
		t.Fatal("fresh remote facts not accepted")
	}
	model.CapabilitiesOverrideJSON = `{"vision":false}`
	if projectModelGuidance(item, model).Eligibility[PurposeVision].Available {
		t.Fatal("user denial ignored")
	}
}

func TestDocumentedImageCapabilityStillRequiresTransport(t *testing.T) {
	for _, tc := range []struct {
		preset, id     string
		generate, edit bool
	}{
		{presetOpenAI, "gpt-image-2.5-sunburst", true, true},
		{presetDashScope, "wan2.7-image-pro", true, true},
		{presetDoubao, "doubao-seedream-5-0-pro-260628", true, false},
		{presetQwenTokenPlan, "qwen-image-3.0-pro", false, false},
		{presetModelScope, "Qwen/Qwen-Image", true, false},
	} {
		g := projectModelGuidance(providerstore.Entity{PresetKey: tc.preset, ProviderKind: ProviderKindLLM}, providerstore.ModelEntity{ModelID: tc.id})
		if g.Eligibility[PurposeImage].Available != tc.generate || g.Eligibility[PurposeEdit].Available != tc.edit || g.Eligibility[PurposeChat].Available {
			t.Fatalf("%s: %+v", tc.id, g)
		}
	}
}

func TestAdviceEvidenceAndIdentityCompleteness(t *testing.T) {
	seen := map[string]bool{}
	for _, entry := range modelAdviceCatalog {
		if entry.Evidence.ReviewedAt != "2026-09-17" || len(entry.Evidence.URLs) == 0 {
			t.Fatalf("missing evidence: %+v", entry)
		}
		for _, raw := range entry.Evidence.URLs {
			u, err := url.Parse(raw)
			if err != nil || u.Scheme != "https" || u.Host == "" {
				t.Fatalf("bad evidence URL: %s", raw)
			}
		}
		for _, preset := range entry.Presets {
			for _, id := range entry.IDs {
				key := preset + "/" + strings.ToLower(id)
				if seen[key] {
					t.Fatalf("ambiguous advice: %s", key)
				}
				seen[key] = true
			}
		}
	}
}
