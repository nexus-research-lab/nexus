package provider

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

func (s *Service) modelsForRecord(ctx context.Context, providerID string) ([]ModelRecord, error) {
	items, err := s.repository.ListModelsByProviderID(ctx, providerID)
	if err != nil {
		return nil, err
	}
	result := make([]ModelRecord, 0, len(items))
	for _, item := range items {
		result = append(result, toModelRecord(item))
	}
	return result, nil
}

func (s *Service) getModelByID(ctx context.Context, providerID string, modelID string) (*providerstore.ModelEntity, error) {
	normalized := normalizeModelID(modelID)
	if normalized == "" {
		return nil, nil
	}
	model, err := s.repository.GetModel(ctx, providerID, normalized)
	if err != nil || model != nil {
		return model, err
	}
	escaped := url.PathEscape(normalized)
	if escaped == normalized {
		return nil, nil
	}
	return s.repository.GetModel(ctx, providerID, escaped)
}

func (s *Service) requireProvider(ctx context.Context, provider string) (*providerstore.Entity, error) {
	normalizedProvider, err := NormalizeProvider(provider, false)
	if err != nil {
		return nil, err
	}
	item, err := s.repository.GetVisibleByProvider(ctx, ownerUserIDFromContext(ctx), normalizedProvider)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("provider 不存在: %s", normalizedProvider)
	}
	normalizeBuiltinEndpoint(item)
	if strings.TrimSpace(item.AuthToken) == "" {
		return nil, fmt.Errorf("provider=%s 缺少 auth_token", item.Provider)
	}
	if strings.TrimSpace(item.BaseURL) == "" {
		return nil, fmt.Errorf("provider=%s 缺少 base_url", item.Provider)
	}
	return item, nil
}

func (s *Service) getPublicProvider(ctx context.Context, provider string) (string, *providerstore.Entity, error) {
	if err := requirePublicProviderManagement(ctx); err != nil {
		return "", nil, err
	}
	normalizedProvider, err := NormalizeProvider(provider, false)
	if err != nil {
		return "", nil, err
	}
	item, err := s.repository.GetScopedByProvider(ctx, providerstore.VisibilityPublic, "", normalizedProvider)
	if err != nil {
		return "", nil, err
	}
	if item == nil {
		return "", nil, fmt.Errorf("provider 不存在: %s", normalizedProvider)
	}
	normalizeBuiltinEndpoint(item)
	return normalizedProvider, item, nil
}

func (s *Service) requirePublicProvider(ctx context.Context, provider string) (*providerstore.Entity, error) {
	_, item, err := s.getPublicProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(item.AuthToken) == "" {
		return nil, fmt.Errorf("provider=%s 缺少 auth_token", item.Provider)
	}
	if strings.TrimSpace(item.BaseURL) == "" {
		return nil, fmt.Errorf("provider=%s 缺少 base_url", item.Provider)
	}
	return item, nil
}

func normalizeModelID(modelID string) string {
	trimmed := strings.TrimSpace(modelID)
	if trimmed == "" {
		return ""
	}
	decoded, err := url.PathUnescape(trimmed)
	if err != nil || strings.TrimSpace(decoded) == "" {
		return trimmed
	}
	return strings.TrimSpace(decoded)
}

func modelDisplayName(modelID string, displayName string) string {
	normalizedID := normalizeModelID(modelID)
	trimmed := strings.TrimSpace(displayName)
	if trimmed == "" {
		return normalizedID
	}
	if normalizeModelID(trimmed) == normalizedID {
		return normalizedID
	}
	return trimmed
}

func normalizeModelEntityIdentity(model *providerstore.ModelEntity, modelID string) bool {
	modelID = normalizeModelID(modelID)
	originalModelID := model.ModelID
	originalDisplayName := model.DisplayName
	model.ModelID = modelID
	if modelDisplayName(originalModelID, originalDisplayName) == normalizeModelID(originalModelID) {
		model.DisplayName = modelID
	}
	return model.ModelID != originalModelID || model.DisplayName != originalDisplayName
}

func toModelRecord(item providerstore.ModelEntity) ModelRecord {
	createdAt := item.CreatedAt
	updatedAt := item.UpdatedAt
	lastSeenAt := item.LastSeenAt
	modelID := normalizeModelID(item.ModelID)
	return ModelRecord{
		ID:                   item.ID,
		ProviderID:           item.ProviderID,
		ModelID:              modelID,
		DisplayName:          modelDisplayName(item.ModelID, item.DisplayName),
		Category:             item.Category,
		Enabled:              item.Enabled,
		IsDefault:            item.IsDefault,
		CapabilitiesAuto:     decodeModelCapabilities(item.CapabilitiesAutoJSON),
		CapabilitiesOverride: decodeModelCapabilities(item.CapabilitiesOverrideJSON),
		ContextWindow:        item.ContextWindow,
		MaxOutputTokens:      item.MaxOutputTokens,
		ProviderOptions:      decodeProviderOptions(item.ProviderOptionsJSON),
		LastSeenAt:           &lastSeenAt,
		CreatedAt:            &createdAt,
		UpdatedAt:            &updatedAt,
	}
}
