// INPUT: Exact provider preset and model ID; aliases are explicitly enumerated.
// OUTPUT: Versioned Nexus selection advice, independent of capability inference.
// POS: Curated product policy. No pricing, performance or account-access guarantees.
package provider

import "strings"

const modelAdviceVersion = "2026-09-17.1"

type modelAdvice struct {
	Presets         []string
	IDs             []string
	Capabilities    ModelCapabilities
	Recommendations map[string]string
}

// Advice is a Nexus starting-point policy, not a vendor ranking. Only matching
// discovered models receive it; entries never add models or change saved defaults.
// Reviewed 2026-09-17 against the existing Nexus provider/model catalog.
var modelAdviceCatalog = []modelAdvice{
	{Presets: []string{presetOpenAI}, IDs: []string{"gpt-image-2"}, Capabilities: ModelCapabilities{ImageOutput: adviceBool(true), ImageEditing: adviceBool(true)}, Recommendations: map[string]string{PurposeImage: "image_generation"}},
	{Presets: []string{presetDashScope}, IDs: []string{"wan2.7-image-pro"}, Capabilities: ModelCapabilities{ImageOutput: adviceBool(true)}, Recommendations: map[string]string{PurposeImage: "image_generation"}},
	{Presets: []string{presetDoubao}, IDs: []string{"doubao-seedream-5-0-260128"}, Capabilities: ModelCapabilities{ImageOutput: adviceBool(true)}, Recommendations: map[string]string{PurposeImage: "image_generation"}},
	{Presets: []string{presetAnthropic}, IDs: []string{"claude-sonnet-4-6", "claude-sonnet-4-5", "claude-sonnet-4-5-20250929"}, Recommendations: map[string]string{PurposeChat: "general_assistant", PurposeVision: "vision_understanding"}},
	{Presets: []string{presetOpenAI, presetAzure}, IDs: []string{"gpt-5.4", "gpt-5.5", "gpt-5.6"}, Recommendations: map[string]string{PurposeChat: "general_assistant", PurposeVision: "vision_understanding"}},
	{Presets: []string{presetDeepSeek}, IDs: []string{"deepseek-chat"}, Recommendations: map[string]string{PurposeChat: "general_assistant"}},
	{Presets: []string{presetDashScope, presetQwenTokenPlan}, IDs: []string{"qwen3.5-plus", "qwen3.6-plus"}, Recommendations: map[string]string{PurposeChat: "general_assistant", PurposeVision: "vision_understanding"}},
	{Presets: []string{presetGLMCodingPlan}, IDs: []string{"glm-5", "glm-5.1"}, Recommendations: map[string]string{PurposeChat: "general_assistant"}},
	{Presets: []string{presetKimiCode}, IDs: []string{"kimi-for-coding"}, Recommendations: map[string]string{PurposeChat: "general_assistant"}},
	{Presets: []string{presetMiniMaxToken}, IDs: []string{"MiniMax-M2.5"}, Recommendations: map[string]string{PurposeChat: "general_assistant"}},
}

func lookupModelAdvice(preset, id string) *modelAdvice {
	// Never strip a namespace or infer a vendor from an arbitrary custom endpoint.
	for i := range modelAdviceCatalog {
		entry := &modelAdviceCatalog[i]
		for _, scope := range entry.Presets {
			if scope == preset {
				for _, alias := range entry.IDs {
					if strings.EqualFold(strings.TrimSpace(id), alias) {
						return entry
					}
				}
			}
		}
	}
	return nil
}

func adviceBool(value bool) *bool { return &value }
