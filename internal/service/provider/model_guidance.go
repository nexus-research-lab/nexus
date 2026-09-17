// INPUT: Provider route, remote model facts, legacy catalog fallback and user overrides.
// OUTPUT: Read-only effective capabilities, provenance, purpose eligibility and recommendations.
// POS: Shared model projection for settings, onboarding, selectors and execution admission.
package provider

import (
	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
	"strings"
)

const (
	PurposeChat   = "chat"
	PurposeVision = "vision"
	PurposeImage  = "image_generation"
	PurposeEdit   = "image_editing"
)

type ModelEligibility struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}
type ModelGuidance struct {
	CatalogVersion  string                      `json:"catalog_version"`
	Capabilities    ModelCapabilities           `json:"capabilities"`
	Sources         map[string]string           `json:"sources"`
	Eligibility     map[string]ModelEligibility `json:"eligibility"`
	Recommendations map[string]string           `json:"recommendations"`
}

func projectModelGuidance(item providerstore.Entity, model providerstore.ModelEntity) ModelGuidance {
	entry := lookupModelAdvice(item.PresetKey, model.ModelID)
	var c ModelCapabilities
	sources := map[string]string{}
	merge := func(value ModelCapabilities, source string) {
		for _, field := range []struct {
			name   string
			value  *bool
			target **bool
		}{
			{"text_output", value.TextOutput, &c.TextOutput},
			{"vision", value.Vision, &c.Vision},
			{"image_output", value.ImageOutput, &c.ImageOutput},
			{"image_editing", value.ImageEditing, &c.ImageEditing},
			{"tool_calling", value.ToolCalling, &c.ToolCalling},
			{"reasoning", value.Reasoning, &c.Reasoning},
			{"embedding", value.Embedding, &c.Embedding},
		} {
			if field.value != nil {
				*field.target = field.value
				sources[field.name] = source
			}
		}
	}
	merge(modelCapabilitiesWithDefaults(model.ModelID, ModelCapabilities{}), "legacy_catalog")
	if entry != nil {
		merge(entry.Capabilities, "catalog")
	}
	merge(decodeModelCapabilities(model.CapabilitiesAutoJSON), "provider_record")
	merge(decodeModelCapabilities(model.CapabilitiesOverrideJSON), "user")
	yes := func(v *bool) bool { return v != nil && *v }
	// Preserve unknown ordinary chat IDs. Known non-chat models need explicit text output.
	chat := item.ProviderKind == ProviderKindLLM && !yes(c.Embedding)
	if c.TextOutput != nil {
		chat = chat && *c.TextOutput
	} else {
		chat = chat && !yes(c.ImageOutput)
		switch strings.ToLower(model.Category) {
		case "image", "embedding", "audio", "video", "rerank":
			chat = false
		}
	}
	imageRoute, routeOK := imageRuntimeProvider(item)
	routeOK = routeOK && supportedImageRoute(imageRoute.APIFormat)
	image := routeOK && yes(c.ImageOutput)
	// An explicitly configured dedicated image endpoint is a user declaration of generation support.
	if routeOK && c.ImageOutput == nil && item.ProviderKind == ProviderKindImageGeneration && !imageProviderRequiresModelFilter(item) {
		image = true
	}
	eligible := func(ok bool, reason string) ModelEligibility {
		if ok {
			reason = ""
		}
		return ModelEligibility{ok, reason}
	}
	g := ModelGuidance{CatalogVersion: modelAdviceVersion, Capabilities: c, Sources: sources,
		Eligibility: map[string]ModelEligibility{
			PurposeChat:   eligible(chat, "not_chat_model"),
			PurposeVision: eligible(chat && yes(c.Vision), "vision_not_confirmed"),
			PurposeImage:  eligible(image, "image_output_or_route_unavailable"),
			PurposeEdit:   eligible(image && yes(c.ImageEditing) && imageRoute.APIFormat != APIFormatModelScopeImageGeneration, "image_editing_not_confirmed"),
		}, Recommendations: map[string]string{}}
	if entry != nil {
		for purpose, reason := range entry.Recommendations {
			if g.Eligibility[purpose].Available {
				g.Recommendations[purpose] = reason
			}
		}
	}
	return g
}

func supportedImageRoute(format string) bool {
	switch format {
	case APIFormatOpenAIImageGeneration, APIFormatDashScopeImageGeneration, APIFormatModelScopeImageGeneration:
		return true
	}
	return false
}
