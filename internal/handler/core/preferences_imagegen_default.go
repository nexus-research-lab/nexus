package core

import (
	"net/http"
	"strings"

	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

func (h *Handlers) withProviderPreferenceDefaults(
	request *http.Request,
	prefs preferencessvc.Preferences,
) (preferencessvc.Preferences, error) {
	if h.providers == nil {
		return prefs, nil
	}
	providerOptions, err := h.providers.ListOptionsForRuntime(request.Context(), prefs.AgentRuntimeKind)
	if err != nil {
		return preferencessvc.Preferences{}, err
	}
	adjusted, _ := applyImagegenDefaultTool(prefs, providerOptions)
	return adjusted, nil
}

func (h *Handlers) persistProviderPreferenceDefaults(
	request *http.Request,
	prefs preferencessvc.Preferences,
) (preferencessvc.Preferences, error) {
	if h.prefs == nil || h.providers == nil {
		return prefs, nil
	}
	providerOptions, err := h.providers.ListOptionsForRuntime(request.Context(), prefs.AgentRuntimeKind)
	if err != nil {
		return preferencessvc.Preferences{}, err
	}
	adjusted, changed := applyImagegenDefaultTool(prefs, providerOptions)
	if !changed {
		return adjusted, nil
	}
	return h.prefs.Update(request.Context(), currentOwnerUserID(request), preferencessvc.UpdateRequest{
		DefaultAgentOptions: &adjusted.DefaultAgentOptions,
	})
}

func applyImagegenDefaultTool(
	prefs preferencessvc.Preferences,
	providerOptions *providercfg.OptionsResponse,
) (preferencessvc.Preferences, bool) {
	enabled := hasConfiguredImageModel(prefs, providerOptions)
	tools, changed := normalizeImagegenDefaultTool(prefs.DefaultAgentOptions.AllowedTools, enabled)
	if !changed {
		return prefs, false
	}
	prefs.DefaultAgentOptions.AllowedTools = tools
	return prefs, true
}

func hasConfiguredImageModel(prefs preferencessvc.Preferences, providerOptions *providercfg.OptionsResponse) bool {
	if providerOptions == nil {
		return false
	}
	if strings.TrimSpace(stringPointerValue(providerOptions.DefaultImageProvider)) != "" &&
		strings.TrimSpace(stringPointerValue(providerOptions.DefaultImageModel)) != "" {
		return true
	}
	return providerOptionsContainModel(
		providerOptions.ImageItems,
		prefs.DefaultImageModelSelection.Provider,
		prefs.DefaultImageModelSelection.Model,
	)
}

func providerOptionsContainModel(items []providercfg.Option, provider string, model string) bool {
	targetProvider := strings.TrimSpace(provider)
	targetModel := strings.TrimSpace(model)
	if targetProvider == "" || targetModel == "" {
		return false
	}
	for _, item := range items {
		if strings.TrimSpace(item.Provider) != targetProvider {
			continue
		}
		for _, option := range item.Models {
			if strings.TrimSpace(option.ModelID) == targetModel {
				return true
			}
		}
	}
	return false
}

func normalizeImagegenDefaultTool(tools []string, enabled bool) ([]string, bool) {
	result := make([]string, 0, len(tools)+1)
	hasImagegen := false
	hasExplicitTool := false
	changed := false
	for _, toolName := range tools {
		value := strings.TrimSpace(toolName)
		if value == "" {
			changed = true
			continue
		}
		hasExplicitTool = true
		if isImagegenToolName(value) {
			if !enabled {
				changed = true
				continue
			}
			if hasImagegen || value != "nexus_imagegen" {
				changed = true
			}
			if !hasImagegen {
				result = append(result, "nexus_imagegen")
				hasImagegen = true
			}
			continue
		}
		result = append(result, value)
	}
	if !hasExplicitTool {
		return result, changed
	}
	if enabled && !hasImagegen {
		result = append(result, "nexus_imagegen")
		changed = true
	}
	return result, changed
}

func isImagegenToolName(value string) bool {
	switch strings.TrimSpace(value) {
	case "nexus_imagegen",
		"generate_image",
		"edit_image",
		"mcp__nexus_imagegen__generate_image",
		"mcp__nexus_imagegen__edit_image":
		return true
	default:
		return false
	}
}

func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
