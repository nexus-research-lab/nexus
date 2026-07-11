package provider

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

var providerPattern = regexp.MustCompile(`[^a-z0-9]+`)

// NormalizeProvider 规整 provider key。
func NormalizeProvider(provider string, allowEmpty bool) (string, error) {
	cleaned := strings.ToLower(strings.TrimSpace(provider))
	if cleaned == "" {
		if allowEmpty {
			return "", nil
		}
		return "", errors.New("provider 不能为空")
	}
	normalized := strings.Trim(providerPattern.ReplaceAllString(cleaned, "-"), "-")
	if normalized == "" {
		return "", fmt.Errorf("非法的 provider: %s", provider)
	}
	return normalized, nil
}

func normalizeProviderKind(providerKind string) string {
	switch strings.TrimSpace(providerKind) {
	case "", ProviderKindLLM:
		return ProviderKindLLM
	case ProviderKindImageGeneration:
		return ProviderKindImageGeneration
	default:
		return ProviderKindLLM
	}
}

func normalizeCreateInput(input CreateInput) (CreateInput, error) {
	provider, err := NormalizeProvider(input.Provider, false)
	if err != nil {
		return CreateInput{}, err
	}
	preset := resolvePreset(input.PresetKey)
	apiFormat := normalizeAPIFormat(input.APIFormat)
	if apiFormat == "" {
		if strings.TrimSpace(input.PresetKey) == "" {
			apiFormat = APIFormatAnthropicMessages
		} else {
			apiFormat = preset.DefaultFormat
		}
	}
	format := preset.Format(apiFormat)
	providerKind := providerKindForFormat(preset, format, input.ProviderKind)
	baseURL := format.BaseURL
	if preset.PresetKey == presetCustom {
		baseURL = firstNonEmpty(input.BaseURL, format.BaseURL)
	}
	modelsPath := strings.TrimSpace(input.ModelsPath)
	if preset.PresetKey != presetCustom {
		modelsPath = format.ModelsPath
	} else if modelsPath == "" {
		modelsPath = format.ModelsPath
	}
	result := CreateInput{
		ProviderKind: providerKind,
		Provider:     provider,
		Visibility:   strings.TrimSpace(input.Visibility),
		PresetKey:    preset.PresetKey,
		APIFormat:    apiFormat,
		DisplayName:  firstNonEmpty(input.DisplayName, preset.DisplayName, provider),
		AuthToken:    strings.TrimSpace(input.AuthToken),
		BaseURL:      baseURL,
		ModelsPath:   modelsPath,
		Enabled:      input.Enabled,
	}
	if result.AuthToken == "" {
		return CreateInput{}, errors.New("auth_token 不能为空")
	}
	if result.BaseURL == "" {
		return CreateInput{}, errors.New("base_url 不能为空")
	}
	return result, nil
}

func normalizeUpdateInput(current providerstore.Entity, input UpdateInput) (providerstore.Entity, error) {
	preset := resolvePreset(firstNonEmpty(input.PresetKey, current.PresetKey))
	apiFormat := normalizeAPIFormat(firstNonEmpty(input.APIFormat, current.APIFormat))
	if apiFormat == "" {
		apiFormat = preset.DefaultFormat
	}
	format := preset.Format(apiFormat)
	providerKind := providerKindForFormat(preset, format, firstNonEmpty(input.ProviderKind, current.ProviderKind))
	displayName := firstNonEmpty(input.DisplayName, preset.DisplayName, current.Provider)
	baseURL := format.BaseURL
	if preset.PresetKey == presetCustom {
		baseURL = firstNonEmpty(input.BaseURL, format.BaseURL)
	}
	if baseURL == "" {
		return providerstore.Entity{}, errors.New("base_url 不能为空")
	}
	modelsPath := strings.TrimSpace(input.ModelsPath)
	if preset.PresetKey != presetCustom {
		modelsPath = format.ModelsPath
	} else if modelsPath == "" {
		modelsPath = format.ModelsPath
	}
	authToken := current.AuthToken
	if input.AuthToken != nil {
		authToken = strings.TrimSpace(*input.AuthToken)
	}
	if input.Enabled && authToken == "" {
		return providerstore.Entity{}, errors.New("auth_token 不能为空")
	}
	current.DisplayName = displayName
	current.AuthToken = authToken
	current.BaseURL = baseURL
	current.ModelsPath = modelsPath
	current.Enabled = input.Enabled
	current.PresetKey = preset.PresetKey
	current.APIFormat = apiFormat
	current.ProviderKind = providerKind
	return current, nil
}

func providerKindForFormat(preset Preset, format PresetFormat, fallback string) string {
	if strings.TrimSpace(format.ProviderKind) != "" {
		return normalizeProviderKind(format.ProviderKind)
	}
	if preset.PresetKey != presetCustom && strings.TrimSpace(preset.ProviderKind) != "" {
		return normalizeProviderKind(preset.ProviderKind)
	}
	if isImageGenerationAPIFormat(format.APIFormat) {
		return ProviderKindImageGeneration
	}
	return normalizeProviderKind(fallback)
}

func isImageGenerationAPIFormat(apiFormat string) bool {
	switch normalizeAPIFormat(apiFormat) {
	case APIFormatOpenAIImageGeneration, APIFormatDashScopeImageGeneration, APIFormatModelScopeImageGeneration:
		return true
	default:
		return false
	}
}
