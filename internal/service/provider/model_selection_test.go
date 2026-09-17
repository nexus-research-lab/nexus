package provider

import (
	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
	"testing"
)

func TestModelGuidancePurposeBoundaries(t *testing.T) {
	item := providerstore.Entity{ProviderKind: ProviderKindLLM, PresetKey: presetDashScope}
	for _, tc := range []struct {
		name, auto, override string
		chat, image, edit    bool
	}{
		{"unknown", `{}`, `{}`, true, false, false},
		{"pure image", `{"image_output":true}`, `{}`, false, true, false},
		{"multimodal", `{"text_output":true,"image_output":true,"image_editing":true}`, `{}`, true, true, true},
		{"explicit false", `{"text_output":true,"image_output":true,"image_editing":true}`, `{"image_output":false}`, true, false, false},
		{"embedding", `{"embedding":true}`, `{}`, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := providerstore.ModelEntity{ModelID: "custom-id", Enabled: true, CapabilitiesAutoJSON: tc.auto, CapabilitiesOverrideJSON: tc.override}
			g := projectModelGuidance(item, model)
			if g.Eligibility[PurposeChat].Available != tc.chat || g.Eligibility[PurposeImage].Available != tc.image || g.Eligibility[PurposeEdit].Available != tc.edit {
				t.Fatalf("unexpected purposes: %+v", g)
			}
			if len(g.Recommendations) != 0 {
				t.Fatal("capabilities must not imply recommendation")
			}
		})
	}
}
func TestAdviceIsScopedAndDoesNotChangeDefaults(t *testing.T) {
	model := providerstore.ModelEntity{ModelID: "deepseek-chat", Enabled: true, IsDefault: false}
	item := providerstore.Entity{ProviderKind: ProviderKindLLM, PresetKey: presetDeepSeek}
	options := modelOptionsForKind(item, []providerstore.ModelEntity{model}, ProviderKindLLM)
	if len(options) != 1 || options[0].Guidance.Recommendations[PurposeChat] == "" || options[0].IsDefault {
		t.Fatalf("bad projection: %+v", options)
	}
	item.PresetKey = presetCustom
	if len(projectModelGuidance(item, model).Recommendations) != 0 {
		t.Fatal("custom provider inherited vendor advice")
	}
	item.PresetKey = presetDeepSeek
	model.ModelID = "other/deepseek-chat"
	if len(projectModelGuidance(item, model).Recommendations) != 0 {
		t.Fatal("namespace was stripped")
	}
}
func TestImageRouteAndOverrideAdmission(t *testing.T) {
	model := providerstore.ModelEntity{ModelID: "test", Enabled: true, CapabilitiesAutoJSON: `{"image_output":true,"image_editing":true}`}
	item := providerstore.Entity{ProviderKind: ProviderKindLLM, PresetKey: presetCustom, APIFormat: APIFormatChatCompletions}
	if projectModelGuidance(item, model).Eligibility[PurposeImage].Available {
		t.Fatal("chat route cannot generate images")
	}
	item.ProviderKind = ProviderKindImageGeneration
	item.APIFormat = APIFormatModelScopeImageGeneration
	g := projectModelGuidance(item, model)
	if !g.Eligibility[PurposeImage].Available || g.Eligibility[PurposeEdit].Available {
		t.Fatal("ModelScope editing must be unavailable")
	}
	model.CapabilitiesOverrideJSON = `{"image_output":false}`
	if projectModelGuidance(item, model).Eligibility[PurposeImage].Available {
		t.Fatal("dedicated route ignored explicit false")
	}
}

func TestModalitiesPreserveTextAndImageAndExplicitDenial(t *testing.T) {
	card := remoteModelFromCard(map[string]any{"id": "mixed", "architecture": map[string]any{"input_modalities": []any{"text", "image"}, "output_modalities": []any{"text", "image"}}})
	if card.Capabilities.TextOutput == nil || !*card.Capabilities.TextOutput || card.Capabilities.ImageOutput == nil || !*card.Capabilities.ImageOutput || card.Category != "chat" {
		t.Fatalf("lost independent modalities: %+v", card)
	}
	c := modelCapabilitiesFromCard(map[string]any{"image_output": false, "output_modalities": []any{"image"}})
	if c.ImageOutput == nil || *c.ImageOutput {
		t.Fatal("modalities overrode explicit denial")
	}
}
