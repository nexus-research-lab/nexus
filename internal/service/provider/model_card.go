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

func defaultModelCard() (ModelCapabilities, string, *int, *int) {
	return ModelCapabilities{}, "chat", nil, nil
}

func (model remoteModel) modelCard() (ModelCapabilities, string, *int, *int) {
	category := strings.TrimSpace(model.Category)
	if category == "" {
		category = "chat"
	}
	return model.Capabilities, category, model.ContextWindow, model.MaxOutputTokens
}

func remoteModelFromCard(card map[string]any) remoteModel {
	capabilities := modelCapabilitiesFromCard(card)
	return remoteModel{
		ID:              firstStringField(card, "id", "model", "name"),
		DisplayName:     firstStringField(card, "display_name", "displayName", "name"),
		Category:        modelCategoryFromCard(card, capabilities),
		Capabilities:    capabilities,
		ContextWindow:   firstIntField(card, "context_length", "context_window", "max_context_length", "input_token_limit", "max_input_tokens"),
		MaxOutputTokens: firstIntField(card, "max_output_tokens", "output_token_limit", "max_tokens", "max_completion_tokens"),
	}
}

func modelCapabilitiesFromCard(card map[string]any) ModelCapabilities {
	return ModelCapabilities{
		Vision: capabilityPointerFromCard(
			card,
			"vision",
			"image_input",
			"image_in",
			"supports_vision",
			"supports_image_input",
			"supports_image_in",
			"supports_video_in",
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
	}
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
	if capabilities.ImageOutput != nil && *capabilities.ImageOutput {
		return "image"
	}
	return "chat"
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
