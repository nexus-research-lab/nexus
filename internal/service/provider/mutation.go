// INPUT: Provider create/update patch/delete 请求、可见域与期望 configuration_version。
// OUTPUT: 规范化后的 Provider 持久化结果，或稳定的 CAS/不存在/使用中错误。
// POS: Provider 主记录变更及强制删除重分配的服务事务编排层。
package provider

import (
	"context"
	"fmt"
	"strings"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

// Create 新增 Provider 配置。
func (s *Service) Create(ctx context.Context, input CreateInput) (*Record, error) {
	if strings.TrimSpace(input.Visibility) == providerstore.VisibilityPublic {
		return nil, fmt.Errorf("普通设置只能创建私有 Provider，请使用运营页面创建订阅 Provider")
	}
	input.Visibility = providerstore.VisibilityPrivate
	return s.createScoped(ctx, input)
}

// CreatePublic 新增订阅运营使用的公共 Provider 配置。
func (s *Service) CreatePublic(ctx context.Context, input CreateInput) (*Record, error) {
	input.Visibility = providerstore.VisibilityPublic
	return s.createScoped(ctx, input)
}

func (s *Service) createScoped(ctx context.Context, input CreateInput) (*Record, error) {
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
		ID:                   s.idFactory("provider"),
		OwnerUserID:          ownerUserID,
		Visibility:           visibility,
		ProviderKind:         normalized.ProviderKind,
		Provider:             normalized.Provider,
		PresetKey:            normalized.PresetKey,
		APIFormat:            normalized.APIFormat,
		DisplayName:          normalized.DisplayName,
		AuthToken:            normalized.AuthToken,
		BaseURL:              normalized.BaseURL,
		ModelsPath:           normalized.ModelsPath,
		Enabled:              normalized.Enabled,
		LastTestStatus:       "",
		LastTestError:        "",
		ConfigurationVersion: 1,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err = s.repository.Create(ctx, item); err != nil {
		return nil, err
	}
	return s.recordForScopedItem(ctx, item)
}

// Update 更新 Provider 配置，并以读取到的 configuration_version 执行 CAS。
func (s *Service) Update(ctx context.Context, provider string, input UpdateInput) (*Record, error) {
	normalizedProvider, err := normalizeProviderReference(provider, false)
	if err != nil {
		return nil, err
	}
	current, err := s.repository.GetVisibleByProvider(ctx, ownerUserIDFromContext(ctx), normalizedProvider)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, normalizedProvider)
	}
	if err = s.requireProviderManagement(ctx, *current); err != nil {
		return nil, err
	}
	return s.updateProviderAtVersion(
		ctx,
		normalizedProvider,
		*current,
		input,
		current.ConfigurationVersion,
		false,
	)
}

// PatchAtVersion 在 expectedVersion 对应的最新持久状态上合并 Provider patch。
func (s *Service) PatchAtVersion(
	ctx context.Context,
	provider string,
	input PatchInput,
	expectedVersion int64,
) (*Record, error) {
	normalizedProvider, err := normalizeProviderReference(provider, false)
	if err != nil {
		return nil, err
	}
	current, err := s.repository.GetVisibleByProvider(ctx, ownerUserIDFromContext(ctx), normalizedProvider)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, normalizedProvider)
	}
	if err = s.requireProviderManagement(ctx, *current); err != nil {
		return nil, err
	}
	return s.updateProviderAtVersion(
		ctx,
		normalizedProvider,
		*current,
		updateInputFromPatch(*current, input),
		expectedVersion,
		false,
	)
}

func (s *Service) updateProviderAtVersion(
	ctx context.Context,
	normalizedProvider string,
	current providerstore.Entity,
	input UpdateInput,
	expectedVersion int64,
	public bool,
) (*Record, error) {
	updated, err := normalizeUpdateInput(current, input)
	if err != nil {
		return nil, err
	}
	updated.UpdatedAt = s.now()
	if providerBecameUnavailable(current, updated) {
		if err = s.validateProviderInvalidationFallback(ctx, current); err != nil {
			return nil, err
		}
	}
	if _, err = s.repository.WithProviderMutation(
		ctx,
		current.ID,
		expectedVersion,
		func(mutation *providerstore.Mutation) error {
			return mutation.UpdateProvider(ctx, updated)
		},
	); err != nil {
		return nil, err
	}
	if public {
		return s.GetPublic(ctx, normalizedProvider)
	}
	return s.Get(ctx, normalizedProvider)
}

// UpdatePublic 更新订阅运营使用的公共 Provider 配置。
func (s *Service) UpdatePublic(ctx context.Context, provider string, input UpdateInput) (*Record, error) {
	normalizedProvider, current, err := s.getPublicProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	return s.updateProviderAtVersion(
		ctx,
		normalizedProvider,
		*current,
		input,
		current.ConfigurationVersion,
		true,
	)
}

// Delete 删除 Provider 配置；强制删除会保留显式绑定，并让运行时暂时回退到用户默认模型。
func (s *Service) Delete(ctx context.Context, provider string, input DeleteInput) (*DeleteResult, error) {
	normalizedProvider, err := normalizeProviderReference(provider, false)
	if err != nil {
		return nil, err
	}
	current, err := s.repository.GetVisibleByProvider(ctx, ownerUserIDFromContext(ctx), normalizedProvider)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, normalizedProvider)
	}
	if err = s.requireProviderManagement(ctx, *current); err != nil {
		return nil, err
	}
	return s.deleteProviderAtVersion(ctx, normalizedProvider, *current, input, current.ConfigurationVersion)
}

// DeleteAtVersion 以 Provider configuration_version CAS 执行删除与 Agent 重分配。
func (s *Service) DeleteAtVersion(
	ctx context.Context,
	provider string,
	input DeleteInput,
	expectedVersion int64,
) (*DeleteResult, error) {
	normalizedProvider, err := normalizeProviderReference(provider, false)
	if err != nil {
		return nil, err
	}
	current, err := s.repository.GetVisibleByProvider(ctx, ownerUserIDFromContext(ctx), normalizedProvider)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, normalizedProvider)
	}
	if err = s.requireProviderManagement(ctx, *current); err != nil {
		return nil, err
	}
	return s.deleteProviderAtVersion(ctx, normalizedProvider, *current, input, expectedVersion)
}

// DeletePublic 删除订阅运营使用的公共 Provider 配置。
func (s *Service) DeletePublic(ctx context.Context, provider string, input DeleteInput) (*DeleteResult, error) {
	normalizedProvider, current, err := s.getPublicProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	return s.deleteProviderAtVersion(ctx, normalizedProvider, *current, input, current.ConfigurationVersion)
}

func (s *Service) deleteProviderAtVersion(
	ctx context.Context,
	normalizedProvider string,
	current providerstore.Entity,
	input DeleteInput,
	expectedVersion int64,
) (*DeleteResult, error) {
	if expectedVersion != current.ConfigurationVersion {
		return nil, ErrConfigurationVersionConflict
	}
	bindingCount, err := s.runtimeBindingCountForMutation(ctx, current)
	if err != nil {
		return nil, err
	}
	if current.ProviderKind == ProviderKindLLM && bindingCount > 0 && !input.Force {
		return nil, fmt.Errorf("provider=%s 仍被 %d 个 Agent 使用，不能删除", normalizedProvider, bindingCount)
	}
	if current.ProviderKind == ProviderKindLLM {
		if err = s.validateProviderInvalidationFallback(ctx, current); err != nil {
			return nil, err
		}
	}
	result := &DeleteResult{Provider: normalizedProvider}
	_, err = s.repository.WithProviderMutation(
		ctx,
		current.ID,
		expectedVersion,
		func(mutation *providerstore.Mutation) error {
			currentUsage, usageErr := mutation.RuntimeBindingCount(ctx, current)
			if usageErr != nil {
				return usageErr
			}
			if current.ProviderKind == ProviderKindLLM && currentUsage > 0 {
				if !input.Force {
					return fmt.Errorf(
						"provider=%s 仍被 %d 个 Agent 使用，不能删除",
						normalizedProvider,
						currentUsage,
					)
				}
				result.AffectedRuntimeCount = currentUsage
				result.FallbackToDefault = true
			}
			return mutation.DeleteProvider(ctx)
		},
	)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func providerBecameUnavailable(current providerstore.Entity, updated providerstore.Entity) bool {
	return current.ProviderKind == ProviderKindLLM && current.Enabled && !updated.Enabled
}

// Get 读取单个 Provider 配置。
func (s *Service) Get(ctx context.Context, provider string) (*Record, error) {
	normalizedProvider, err := normalizeProviderReference(provider, false)
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

// GetPrivate 只读取当前 owner 自己管理的私有 Provider。
//
// 即使当前认证主体是 owner/admin，也不会回退到同名公共订阅 Provider。
func (s *Service) GetPrivate(ctx context.Context, provider string) (*Record, error) {
	normalizedProvider, err := normalizeProviderReference(provider, false)
	if err != nil {
		return nil, err
	}
	ownerUserID := ownerUserIDFromContext(ctx)
	item, err := s.repository.GetScopedByProvider(
		ctx,
		providerstore.VisibilityPrivate,
		ownerUserID,
		normalizedProvider,
	)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("私有 provider 不存在: %s", normalizedProvider)
	}
	return s.recordForScopedItem(ctx, *item)
}

// GetPublic 读取订阅运营使用的公共 Provider 配置。
func (s *Service) GetPublic(ctx context.Context, provider string) (*Record, error) {
	_, item, err := s.getPublicProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	return s.recordForScopedItem(ctx, *item)
}
