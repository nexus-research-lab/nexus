// INPUT: Provider create/full update/merge patch 与当前未脱敏持久化记录。
// OUTPUT: 经过端点策略、格式与凭据规则规整的完整 Provider 写入值。
// POS: 对话 patch 在 CAS 前合并最新 Provider 真相的纯规整层。
package provider

import (
	"errors"
	"fmt"
	"net/url"
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

// normalizeProviderReference 保留已持久化 Provider 标识，只清理传输层首尾空白。
func normalizeProviderReference(provider string, allowEmpty bool) (string, error) {
	trimmed := strings.TrimSpace(provider)
	if trimmed == "" && !allowEmpty {
		return "", errors.New("provider 不能为空")
	}
	return trimmed, nil
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
	baseURL, err := normalizePresetBaseURL(preset, input.BaseURL, format.BaseURL)
	if err != nil {
		return CreateInput{}, err
	}
	modelsPath := strings.TrimSpace(input.ModelsPath)
	if normalizeEndpointMode(preset.EndpointMode) != EndpointModeCustom {
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
	baseURL, err := normalizePresetBaseURL(
		preset,
		firstNonEmpty(input.BaseURL, current.BaseURL),
		format.BaseURL,
	)
	if err != nil {
		return providerstore.Entity{}, err
	}
	if baseURL == "" {
		return providerstore.Entity{}, errors.New("base_url 不能为空")
	}
	modelsPath := strings.TrimSpace(input.ModelsPath)
	if normalizeEndpointMode(preset.EndpointMode) != EndpointModeCustom {
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

func updateInputFromPatch(current providerstore.Entity, patch PatchInput) UpdateInput {
	input := UpdateInput{
		ProviderKind: current.ProviderKind,
		PresetKey:    current.PresetKey,
		APIFormat:    current.APIFormat,
		DisplayName:  current.DisplayName,
		BaseURL:      current.BaseURL,
		ModelsPath:   current.ModelsPath,
		Enabled:      current.Enabled,
		AuthToken:    patch.AuthToken,
	}
	if patch.ProviderKind != nil {
		input.ProviderKind = *patch.ProviderKind
	}
	if patch.PresetKey != nil {
		input.PresetKey = *patch.PresetKey
	}
	if patch.APIFormat != nil {
		input.APIFormat = *patch.APIFormat
	}
	if patch.DisplayName != nil {
		input.DisplayName = *patch.DisplayName
	}
	if patch.BaseURL != nil {
		input.BaseURL = *patch.BaseURL
	}
	if patch.ModelsPath != nil {
		input.ModelsPath = *patch.ModelsPath
	}
	if patch.Enabled != nil {
		input.Enabled = *patch.Enabled
	}
	return input
}

func normalizePresetBaseURL(preset Preset, inputBaseURL string, fallbackBaseURL string) (string, error) {
	endpointMode := normalizeEndpointMode(preset.EndpointMode)
	baseURL := strings.TrimSpace(fallbackBaseURL)
	if endpointMode != EndpointModeFixed {
		baseURL = firstNonEmpty(inputBaseURL, fallbackBaseURL)
	}
	if preset.PresetKey == presetAzure && baseURL != "" {
		return normalizeAzureOpenAIBaseURL(baseURL)
	}
	return baseURL, nil
}

// normalizeAzureOpenAIBaseURL 只接受资源级 v1 地址，避免把 deployment operation URL 当成 Base URL。
func normalizeAzureOpenAIBaseURL(baseURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("Azure OpenAI base_url 必须是完整的 HTTPS 地址")
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return "", errors.New("Azure OpenAI base_url 必须使用 HTTPS")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("Azure OpenAI base_url 不能包含 query 或 fragment")
	}
	path := strings.TrimRight(parsed.Path, "/")
	switch {
	case path == "":
		parsed.Path = "/openai/v1"
	case strings.EqualFold(path, "/openai"):
		parsed.Path = "/openai/v1"
	case strings.EqualFold(path, "/openai/v1"):
		parsed.Path = "/openai/v1"
	default:
		return "", errors.New("Azure OpenAI base_url 必须是资源根地址或以 /openai/v1 结尾")
	}
	parsed.RawPath = ""
	return strings.TrimRight(parsed.String(), "/"), nil
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func boolPointer(value bool) *bool {
	return &value
}
