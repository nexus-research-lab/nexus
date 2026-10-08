// INPUT: Official preset/model identities, stored facts and explicit overrides.
// OUTPUT: Regression coverage for recommendation scope, modalities and provenance.
// POS: Provider guidance contract tests; no live credentials or provider calls.
package provider

import (
	"testing"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

func TestModelNameFallbackKeepsProviderPolicyScoped(t *testing.T) {
	item := providerstore.Entity{PresetKey: presetCustom, ProviderKind: ProviderKindLLM}
	model := providerstore.ModelEntity{ModelID: "qwen3.8-max"}
	g := projectModelGuidance(item, model)
	if !g.Eligibility[PurposeVision].Available || g.Recommendations[PurposeChat] != "" || g.Evidence == nil || g.Evidence.Notice != "" {
		t.Fatalf("模型名能力与 Provider 专属建议未分离: %+v", g)
	}
	if lookupModelAdviceByID("wan2.7-image-pro") != nil {
		t.Fatal("冲突的同名模型能力不应自动推断")
	}
	model.CapabilitiesAutoJSON = encodeModelAutoCapabilities(ModelCapabilities{Vision: adviceBool(false)})
	if projectModelGuidance(item, model).Eligibility[PurposeVision].Available {
		t.Fatal("Provider 显式能力否定未覆盖模型名默认值")
	}
	model.CapabilitiesOverrideJSON = `{"vision":true}`
	if !projectModelGuidance(item, model).Eligibility[PurposeVision].Available {
		t.Fatal("用户显式能力覆盖未生效")
	}
	model.ModelID = "glm-5.3"
	model.CapabilitiesOverrideJSON = "{}"
	model.CapabilitiesAutoJSON = encodeModelAutoCapabilities(ModelCapabilities{Vision: adviceBool(true)})
	if !projectModelGuidance(item, model).Eligibility[PurposeVision].Available {
		t.Fatal("远端事实应能修正跨 Provider 的同名默认值")
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

func TestMultimodalTextOutputWinsOverEmbeddingFlag(t *testing.T) {
	item := providerstore.Entity{PresetKey: presetKimiCode, ProviderKind: ProviderKindLLM}
	model := providerstore.ModelEntity{
		ModelID:                  "k3-256k",
		CapabilitiesAutoJSON:     `{"vision":true,"reasoning":true}`,
		CapabilitiesOverrideJSON: `{"vision":true,"image_output":true,"tool_calling":true,"reasoning":true,"embedding":true}`,
	}

	guidance := projectModelGuidance(item, model)
	if !guidance.Eligibility[PurposeChat].Available || !guidance.Eligibility[PurposeVision].Available {
		t.Fatalf("text-capable multimodal model must remain eligible for chat and vision: %+v", guidance)
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
