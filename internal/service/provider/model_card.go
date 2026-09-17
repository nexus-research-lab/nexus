// INPUT: Remote model cards, modality arrays and optional capability booleans.
// OUTPUT: Independent input/output capabilities; discovery persists remote facts only.
// POS: Model discovery decoder; category remains a compatibility hint.
package provider

import (
	"strconv"
	"strings"
)

type remoteModel struct {
	ID              string
	DisplayName     string
	Category        string
	Capabilities    ModelCapabilities
	ContextWindow   *int
	MaxOutputTokens *int
}

func defaultModelCard(modelID string, providerKind string) (ModelCapabilities, string, *int, *int) {
	category := "chat"
	if normalizeProviderKind(providerKind) == ProviderKindImageGeneration {
		category = "image"
		return ModelCapabilities{}, category, nil, nil
	}
	return ModelCapabilities{}, category,
		contextWindowOrDefault(modelID, nil), maxOutputTokensOrDefault(modelID, nil)
}

func (model remoteModel) modelCard(providerKind string) (ModelCapabilities, string, *int, *int) {
	category := strings.TrimSpace(model.Category)
	if category == "" {
		_, category, _, _ = defaultModelCard(model.ID, providerKind)
	}
	contextWindow := model.ContextWindow
	maxOutputTokens := model.MaxOutputTokens
	if normalizeProviderKind(providerKind) == ProviderKindLLM && category == "chat" {
		contextWindow = contextWindowOrDefault(model.ID, contextWindow)
		maxOutputTokens = maxOutputTokensOrDefault(model.ID, maxOutputTokens)
	}
	return model.Capabilities, category, contextWindow, maxOutputTokens
}

func remoteModelFromCard(card map[string]any) remoteModel {
	capabilities := modelCapabilitiesFromCard(card)
	return remoteModel{
		ID:           firstStringField(card, "id", "model", "name"),
		DisplayName:  firstStringField(card, "display_name", "displayName", "name"),
		Category:     modelCategoryFromCard(card, capabilities),
		Capabilities: capabilities,
		ContextWindow: firstIntField(
			card,
			"context_length",
			"contextLength",
			"context_window",
			"contextWindow",
			"max_context_length",
			"maxContextLength",
			"input_token_limit",
			"inputTokenLimit",
			"max_input_tokens",
			"maxInputTokens",
		),
		MaxOutputTokens: firstIntField(
			card,
			"max_output_tokens",
			"maxOutputTokens",
			"output_token_limit",
			"outputTokenLimit",
			"max_tokens",
			"maxTokens",
			"max_completion_tokens",
			"maxCompletionTokens",
		),
	}
}

func modelCapabilitiesFromCard(card map[string]any) ModelCapabilities {
	result := ModelCapabilities{
		TextOutput: capabilityPointerFromCard(card, "text_output", "supports_text_output"),
		Vision: capabilityPointerFromCard(
			card,
			"vision",
			"image_input",
			"image_in",
			"supports_vision",
			"supports_image_input",
			"supports_image_in",
		),
		ImageOutput: capabilityPointerFromCard(
			card,
			"image_output",
			"image_out",
			"supports_image_output",
			"supports_image_out",
		),
		ToolCalling: capabilityPointerFromCard(
			card,
			"tool_calling",
			"tools",
			"function_calling",
			"supports_tool_calling",
			"supports_tools",
			"supports_function_calling",
		),
		Reasoning: capabilityPointerFromCard(
			card,
			"reasoning",
			"thinking",
			"supports_reasoning",
			"supports_thinking",
		),
		Embedding: capabilityPointerFromCard(
			card,
			"embedding",
			"embeddings",
			"supports_embedding",
			"supports_embeddings",
		),
		ImageEditing: capabilityPointerFromCard(
			card,
			"image_editing",
			"image_edit",
			"supports_image_editing",
			"supports_image_edit",
		),
	}
	// Some model APIs declare modalities instead of boolean capability fields.
	sources := []map[string]any{card}
	if architecture, ok := mapFromAny(card["architecture"]); ok {
		sources = append(sources, architecture)
	}
	for _, source := range sources {
		for _, spec := range []struct {
			key, modality string
			target        **bool
		}{
			{"input_modalities", "image", &result.Vision},
			{"output_modalities", "text", &result.TextOutput},
			{"output_modalities", "image", &result.ImageOutput},
		} {
			if *spec.target != nil {
				continue
			}
			if values, ok := stringSliceFromAny(source[spec.key]); ok && len(values) > 0 {
				found := false
				for _, value := range values {
					if strings.EqualFold(value, spec.modality) {
						found = true
					}
				}
				*spec.target = boolPointer(found)
			}
		}
	}
	return result

}

func modelCategoryFromCard(card map[string]any, capabilities ModelCapabilities) string {
	for _, value := range []string{
		firstStringField(card, "category", "model_category"),
		firstStringField(card, "model_type", "mode"),
		firstStringField(card, "type"),
	} {
		if category := normalizeModelCategory(value); category != "" {
			return category
		}
	}
	if capabilities.Embedding != nil && *capabilities.Embedding {
		return "embedding"
	}
	if capabilities.TextOutput != nil && *capabilities.TextOutput {
		return "chat"
	}
	if capabilities.ImageOutput != nil && *capabilities.ImageOutput {
		return "image"
	}
	return ""
}

func normalizeModelCategory(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" || normalized == "model" {
		return ""
	}
	switch {
	case strings.Contains(normalized, "embed"):
		return "embedding"
	case strings.Contains(normalized, "image"):
		return "image"
	case strings.Contains(normalized, "audio"):
		return "audio"
	case strings.Contains(normalized, "video"):
		return "video"
	case strings.Contains(normalized, "rerank"):
		return "rerank"
	default:
		return "chat"
	}
}

func capabilityPointerFromCard(card map[string]any, keys ...string) *bool {
	for _, source := range modelCardSources(card) {
		for _, key := range keys {
			if value, exists := source[key]; exists {
				if parsed, ok := boolFromAny(value); ok {
					return boolPointer(parsed)
				}
			}
		}
	}
	tokens := capabilityTokensFromCard(card)
	for _, token := range tokens {
		for _, key := range keys {
			if token == normalizeCapabilityToken(key) {
				return boolPointer(true)
			}
		}
	}
	return nil
}

func modelCardSources(card map[string]any) []map[string]any {
	result := []map[string]any{card}
	for _, key := range []string{"capabilities", "features", "limits"} {
		if nested, ok := mapFromAny(card[key]); ok {
			result = append(result, nested)
		}
	}
	return result
}

func capabilityTokensFromCard(card map[string]any) []string {
	result := []string{}
	for _, key := range []string{"capabilities", "features", "supported_features"} {
		values, ok := stringSliceFromAny(card[key])
		if !ok {
			continue
		}
		for _, value := range values {
			token := normalizeCapabilityToken(value)
			if token != "" {
				result = append(result, token)
			}
		}
	}
	return result
}

func firstStringField(card map[string]any, keys ...string) string {
	for _, source := range modelCardSources(card) {
		for _, key := range keys {
			if value, ok := stringFromAny(source[key]); ok {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func firstIntField(card map[string]any, keys ...string) *int {
	for _, source := range modelCardSources(card) {
		for _, key := range keys {
			if value, ok := intFromAny(source[key]); ok {
				return &value
			}
		}
	}
	return nil
}

func stringFromAny(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return typed, true
	default:
		return "", false
	}
}

func stringSliceFromAny(value any) ([]string, bool) {
	items, ok := value.([]any)
	if !ok {
		return nil, false
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if value, ok := stringFromAny(item); ok {
			result = append(result, value)
		}
	}
	return result, true
}

func mapFromAny(value any) (map[string]any, bool) {
	typed, ok := value.(map[string]any)
	return typed, ok
}

func boolFromAny(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "yes", "y", "1", "supported", "enabled":
			return true, true
		case "false", "no", "n", "0", "unsupported", "disabled":
			return false, true
		default:
			return false, false
		}
	case float64:
		if typed == 1 {
			return true, true
		}
		if typed == 0 {
			return false, true
		}
	case map[string]any:
		return boolFromNestedCapability(typed)
	}
	return false, false
}

func boolFromNestedCapability(value map[string]any) (bool, bool) {
	for _, key := range []string{"supported", "enabled", "available"} {
		if parsed, ok := boolFromAny(value[key]); ok {
			return parsed, true
		}
	}
	return false, false
}

func intFromAny(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		if typed <= 0 {
			return 0, false
		}
		return int(typed), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil || parsed <= 0 {
			return 0, false
		}
		return parsed, true
	}
	return 0, false
}

func normalizeCapabilityToken(value string) string {
	replacer := strings.NewReplacer("_", "", "-", "", " ", "")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(value)))
}
