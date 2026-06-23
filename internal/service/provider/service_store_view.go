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
	if item == nil || strings.TrimSpace(item.PresetKey) == "" || item.PresetKey == presetCustom {
		return
	}
	preset := resolvePreset(item.PresetKey)
	if preset.PresetKey == presetCustom {
		return
	}
	apiFormat := normalizeAPIFormat(item.APIFormat)
	if apiFormat == "" {
		apiFormat = preset.DefaultFormat
	}
	format := preset.Format(apiFormat)
	item.APIFormat = apiFormat
	item.BaseURL = format.BaseURL
	item.ModelsPath = format.ModelsPath
}
