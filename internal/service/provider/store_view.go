package provider

import (
	"context"
	"strings"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

func (s *Service) listAndNormalize(ctx context.Context) ([]providerstore.Entity, error) {
	items, err := s.repository.ListVisible(ctx, ownerUserIDFromContext(ctx))
	if err != nil {
		return nil, err
	}
	items = collapseVisibleProviders(items)
	for index := range items {
		normalizeBuiltinEndpoint(&items[index])
	}
	return items, nil
}

func (s *Service) listPublicAndNormalize(ctx context.Context) ([]providerstore.Entity, error) {
	items, err := s.repository.ListPublic(ctx)
	if err != nil {
		return nil, err
	}
	for index := range items {
		normalizeBuiltinEndpoint(&items[index])
	}
	return items, nil
}

func collapseVisibleProviders(items []providerstore.Entity) []providerstore.Entity {
	result := make([]providerstore.Entity, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		provider := strings.TrimSpace(item.Provider)
		if provider == "" || seen[provider] {
			continue
		}
		seen[provider] = true
		result = append(result, item)
	}
	return result
}

func normalizeBuiltinEndpoint(item *providerstore.Entity) {
	if item == nil || strings.TrimSpace(item.PresetKey) == "" {
		return
	}
	projectLegacyAzurePreset(item)
	preset := resolvePreset(item.PresetKey)
	endpointMode := normalizeEndpointMode(preset.EndpointMode)
	if endpointMode == EndpointModeCustom {
		return
	}
	apiFormat := normalizeAPIFormat(item.APIFormat)
	if apiFormat == "" {
		apiFormat = preset.DefaultFormat
	}
	format := preset.Format(apiFormat)
	item.APIFormat = apiFormat
	item.ModelsPath = format.ModelsPath
	if endpointMode == EndpointModeFixed {
		item.BaseURL = format.BaseURL
		return
	}
	if baseURL, err := normalizePresetBaseURL(preset, item.BaseURL, format.BaseURL); err == nil {
		item.BaseURL = baseURL
	}
}

// projectLegacyAzurePreset 让早期同名 Custom Azure 配置直接进入内置 preset，保存后即可正式写回。
func projectLegacyAzurePreset(item *providerstore.Entity) {
	if item.PresetKey != presetCustom || strings.TrimSpace(item.Provider) != presetAzure {
		return
	}
	baseURL, err := normalizeAzureOpenAIBaseURL(item.BaseURL)
	if err != nil || !IsAzureOpenAIEndpoint(baseURL) {
		return
	}
	item.PresetKey = presetAzure
	item.BaseURL = baseURL
}
