package provider

import (
	"context"
	"fmt"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

// Create 新增 Provider 配置。
func (s *Service) Create(ctx context.Context, input CreateInput) (*Record, error) {
	normalized, err := normalizeCreateInput(input)
	if err != nil {
		return nil, err
	}
	visibility, ownerUserID, err := s.createVisibility(ctx, normalized.Visibility)
	if err != nil {
		return nil, err
	}
	existing, err := s.repository.GetScopedByProvider(ctx, visibility, ownerUserID, normalized.Provider)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("provider 已存在: %s", normalized.Provider)
	}
	now := s.now()
	item := providerstore.Entity{
		ID:             s.idFactory("provider"),
		OwnerUserID:    ownerUserID,
		Visibility:     visibility,
		ProviderKind:   normalized.ProviderKind,
		Provider:       normalized.Provider,
		PresetKey:      normalized.PresetKey,
		APIFormat:      normalized.APIFormat,
		DisplayName:    normalized.DisplayName,
		AuthToken:      normalized.AuthToken,
		BaseURL:        normalized.BaseURL,
		ModelsPath:     normalized.ModelsPath,
		Enabled:        normalized.Enabled,
		LastTestStatus: "",
		LastTestError:  "",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err = s.repository.Create(ctx, item); err != nil {
		return nil, err
	}
	return s.recordForScopedItem(ctx, item)
}

// CreatePublic 新增订阅运营使用的公共 Provider 配置。
func (s *Service) CreatePublic(ctx context.Context, input CreateInput) (*Record, error) {
	input.Visibility = providerstore.VisibilityPublic
	return s.Create(ctx, input)
}

// Update 更新 Provider 配置。
func (s *Service) Update(ctx context.Context, provider string, input UpdateInput) (*Record, error) {
	normalizedProvider, err := NormalizeProvider(provider, false)
	if err != nil {
		return nil, err
	}
	current, err := s.repository.GetVisibleByProvider(ctx, ownerUserIDFromContext(ctx), normalizedProvider)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("provider 不存在: %s", normalizedProvider)
	}
	if err = s.requireProviderManagement(ctx, *current); err != nil {
		return nil, err
	}
	usageCount, err := s.usageCountForMutation(ctx, *current)
	if err != nil {
		return nil, err
	}
	if current.ProviderKind == ProviderKindLLM && usageCount > 0 && !input.Enabled {
		return nil, fmt.Errorf("provider=%s 仍被 %d 个 Agent 使用，不能禁用", normalizedProvider, usageCount)
	}
	updated, err := normalizeUpdateInput(*current, input)
	if err != nil {
		return nil, err
	}
	updated.UpdatedAt = s.now()
	if err = s.repository.Update(ctx, updated); err != nil {
		return nil, err
	}
	return s.Get(ctx, normalizedProvider)
}

// UpdatePublic 更新订阅运营使用的公共 Provider 配置。
func (s *Service) UpdatePublic(ctx context.Context, provider string, input UpdateInput) (*Record, error) {
	normalizedProvider, current, err := s.getPublicProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	usageCount, err := s.usageCountForMutation(ctx, *current)
	if err != nil {
		return nil, err
	}
	if current.ProviderKind == ProviderKindLLM && usageCount > 0 && !input.Enabled {
		return nil, fmt.Errorf("provider=%s 仍被 %d 个 Agent 使用，不能禁用", normalizedProvider, usageCount)
	}
	updated, err := normalizeUpdateInput(*current, input)
	if err != nil {
		return nil, err
	}
	updated.UpdatedAt = s.now()
	if err = s.repository.Update(ctx, updated); err != nil {
		return nil, err
	}
	return s.GetPublic(ctx, normalizedProvider)
}

// Delete 删除 Provider 配置；强制删除会先把显式绑定切到平台默认 Provider。
func (s *Service) Delete(ctx context.Context, provider string, input DeleteInput) (*DeleteResult, error) {
	normalizedProvider, err := NormalizeProvider(provider, false)
	if err != nil {
		return nil, err
	}
	current, err := s.repository.GetVisibleByProvider(ctx, ownerUserIDFromContext(ctx), normalizedProvider)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("provider 不存在: %s", normalizedProvider)
	}
	if err = s.requireProviderManagement(ctx, *current); err != nil {
		return nil, err
	}
	usageCount, err := s.usageCountForMutation(ctx, *current)
	if err != nil {
		return nil, err
	}
	result := &DeleteResult{Provider: normalizedProvider}
	if current.ProviderKind == ProviderKindLLM && usageCount > 0 {
		if !input.Force {
			return nil, fmt.Errorf("provider=%s 仍被 %d 个 Agent 使用，不能删除", normalizedProvider, usageCount)
		}
		replacement, replacementErr := s.replacementRuntimeSelectionForDelete(ctx, *current)
		if replacementErr != nil {
			return nil, replacementErr
		}
		reassigned, replaceErr := s.replaceRuntimeProviderForDelete(ctx, *current, replacement.provider.Provider, replacement.model.ModelID)
		if replaceErr != nil {
			return nil, replaceErr
		}
		result.ReplacementProvider = replacement.provider.Provider
		result.ReplacementModel = replacement.model.ModelID
		result.ReassignedRuntimeCount = reassigned
	}
	if err = s.repository.Delete(ctx, current.ID); err != nil {
		return nil, err
	}
	return result, nil
}

// DeletePublic 删除订阅运营使用的公共 Provider 配置。
func (s *Service) DeletePublic(ctx context.Context, provider string, input DeleteInput) (*DeleteResult, error) {
	normalizedProvider, current, err := s.getPublicProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	usageCount, err := s.usageCountForMutation(ctx, *current)
	if err != nil {
		return nil, err
	}
	result := &DeleteResult{Provider: normalizedProvider}
	if current.ProviderKind == ProviderKindLLM && usageCount > 0 {
		if !input.Force {
			return nil, fmt.Errorf("provider=%s 仍被 %d 个 Agent 使用，不能删除", normalizedProvider, usageCount)
		}
		replacement, replacementErr := s.replacementRuntimeSelectionForDelete(ctx, *current)
		if replacementErr != nil {
			return nil, replacementErr
		}
		reassigned, replaceErr := s.replaceRuntimeProviderForDelete(ctx, *current, replacement.provider.Provider, replacement.model.ModelID)
		if replaceErr != nil {
			return nil, replaceErr
		}
		result.ReplacementProvider = replacement.provider.Provider
		result.ReplacementModel = replacement.model.ModelID
		result.ReassignedRuntimeCount = reassigned
	}
	if err = s.repository.Delete(ctx, current.ID); err != nil {
		return nil, err
	}
	return result, nil
}

// Get 读取单个 Provider 配置。
func (s *Service) Get(ctx context.Context, provider string) (*Record, error) {
	normalizedProvider, err := NormalizeProvider(provider, false)
	if err != nil {
		return nil, err
	}
	if _, err = s.listAndNormalize(ctx); err != nil {
		return nil, err
	}
	ownerUserID := ownerUserIDFromContext(ctx)
	item, err := s.repository.GetVisibleByProvider(ctx, ownerUserID, normalizedProvider)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("provider 不存在: %s", normalizedProvider)
	}
	normalizeBuiltinEndpoint(item)
	usageCount := 0
	usageAgents := []providerstore.UsageAgentEntity(nil)
	if item.ProviderKind == ProviderKindLLM {
		var countErr error
		usageCount, countErr = s.repository.UsageCountForOwner(ctx, ownerUserID, item.Provider)
		if countErr != nil {
			return nil, countErr
		}
		var usageErr error
		usageAgents, usageErr = s.repository.ListUsageAgentsByOwnerProvider(ctx, ownerUserID, item.Provider)
		if usageErr != nil {
			return nil, usageErr
		}
	}
	models, err := s.modelsForRecord(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	record := toRecord(ctx, *item, usageCount, usageAgents, models)
	return &record, nil
}

// GetPublic 读取订阅运营使用的公共 Provider 配置。
func (s *Service) GetPublic(ctx context.Context, provider string) (*Record, error) {
	_, item, err := s.getPublicProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	return s.recordForScopedItem(ctx, *item)
}
