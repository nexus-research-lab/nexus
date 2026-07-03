package provider

import (
	"context"
)

// List 返回完整 Provider 配置列表。
func (s *Service) List(ctx context.Context) ([]Record, error) {
	items, err := s.listAndNormalize(ctx)
	if err != nil {
		return nil, err
	}
	ownerUserID := ownerUserIDFromContext(ctx)
	usageAgents, err := s.repository.ListUsageAgentsByOwner(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	result := make([]Record, 0, len(items))
	for _, item := range items {
		usageCount := 0
		if item.ProviderKind == ProviderKindLLM {
			usageCount = len(usageAgents[item.Provider])
		}
		models, err := s.modelsForRecord(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, toRecord(ctx, item, usageCount, usageAgents[item.Provider], models))
	}
	return result, nil
}

// ListPublic 返回订阅运营页管理的公共 Provider 配置列表。
func (s *Service) ListPublic(ctx context.Context) ([]Record, error) {
	if err := requirePublicProviderManagement(ctx); err != nil {
		return nil, err
	}
	items, err := s.listPublicAndNormalize(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Record, 0, len(items))
	for _, item := range items {
		record, recordErr := s.recordForScopedItem(ctx, item)
		if recordErr != nil {
			return nil, recordErr
		}
		result = append(result, *record)
	}
	return result, nil
}

// ListOptions 返回启用状态的 Provider 下拉选项。
func (s *Service) ListOptions(ctx context.Context) (*OptionsResponse, error) {
	return s.ListOptionsForRuntime(ctx, "claude")
}

// ListOptionsForRuntime 返回指定 Agent runtime 可使用的 Provider 下拉选项。
func (s *Service) ListOptionsForRuntime(ctx context.Context, runtimeKind string) (*OptionsResponse, error) {
	items, err := s.listAndNormalize(ctx)
	if err != nil {
		return nil, err
	}
	runtimeKind = normalizeRuntimeKind(runtimeKind)
	result := &OptionsResponse{
		Items:           make([]Option, 0, len(items)),
		BackgroundItems: make([]Option, 0, len(items)),
		ImageItems:      make([]Option, 0, len(items)),
	}
	for _, item := range items {
		if !item.Enabled {
			continue
		}
		models, err := s.enabledModelOptionsForKind(ctx, item, ProviderKindLLM)
		if err != nil {
			return nil, err
		}
		option := Option{
			Provider:    item.Provider,
			DisplayName: item.DisplayName,
			Visibility:  item.Visibility,
			Models:      models,
		}
		switch {
		case item.ProviderKind == ProviderKindLLM:
			result.BackgroundItems = append(result.BackgroundItems, option)
			if isAgentRuntimeProviderForRuntime(item, runtimeKind) {
				result.Items = append(result.Items, option)
			}
			imageModels, modelErr := s.enabledModelOptionsForKind(ctx, item, ProviderKindImageGeneration)
			if modelErr != nil {
				return nil, modelErr
			}
			if len(imageModels) > 0 {
				result.ImageItems = append(result.ImageItems, Option{
					Provider:    item.Provider,
					DisplayName: item.DisplayName,
					Visibility:  item.Visibility,
					Models:      imageModels,
				})
			}
		case item.ProviderKind == ProviderKindImageGeneration:
			imageModels, modelErr := s.enabledModelOptionsForKind(ctx, item, ProviderKindImageGeneration)
			if modelErr != nil {
				return nil, modelErr
			}
			result.ImageItems = append(result.ImageItems, Option{
				Provider:    item.Provider,
				DisplayName: item.DisplayName,
				Visibility:  item.Visibility,
				Models:      imageModels,
			})
		}
	}
	if target, err := s.defaultRuntimeSelectionForRuntime(ctx, runtimeKind); err != nil {
		return nil, err
	} else if target != nil {
		selection := modelSelectionFromTarget(*target)
		result.DefaultProvider = &selection.Provider
		result.DefaultModel = &selection.Model
		result.DefaultSelection = &selection
	}
	if target, err := s.defaultImageSelection(ctx); err != nil {
		return nil, err
	} else if target != nil {
		selection := modelSelectionFromTarget(*target)
		result.DefaultImageProvider = &selection.Provider
		result.DefaultImageModel = &selection.Model
		result.DefaultImageSelection = &selection
	}
	return result, nil
}

// DefaultProvider 返回当前默认运行模型所属的 Provider，保留给前端启动数据使用。
func (s *Service) DefaultProvider(ctx context.Context) (*string, error) {
	target, err := s.defaultRuntimeSelection(ctx)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, nil
	}
	value := target.provider.Provider
	return &value, nil
}

// AvailabilityState 描述当前 Provider 配置的就绪程度，便于启动期或健康检查上报。
type AvailabilityState struct {
	Total       int
	EnabledList []string
	HasDefault  bool
}

// Availability 汇总 Provider 现状：是否有可用条目、是否已选默认。
func (s *Service) Availability(ctx context.Context) (AvailabilityState, error) {
	items, err := s.listAndNormalize(ctx)
	if err != nil {
		return AvailabilityState{}, err
	}
	state := AvailabilityState{}
	for _, item := range items {
		if item.ProviderKind != ProviderKindLLM {
			continue
		}
		state.Total++
		if !item.Enabled || !isAnyAgentRuntimeProvider(item) {
			continue
		}
		state.EnabledList = append(state.EnabledList, item.Provider)
		models, modelErr := s.repository.ListModelsByProviderID(ctx, item.ID)
		if modelErr != nil {
			return AvailabilityState{}, modelErr
		}
		for _, model := range models {
			if model.Enabled && model.IsDefault && modelUsableForProviderKind(item, model, ProviderKindLLM) {
				state.HasDefault = true
			}
		}
	}
	return state, nil
}
