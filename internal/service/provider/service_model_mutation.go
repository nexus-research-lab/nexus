package provider

import (
	"context"
	"errors"
	"fmt"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

// FetchModels 从远端 /models 端点拉取模型列表并合并到本地模型卡。
func (s *Service) FetchModels(ctx context.Context, provider string) (*FetchModelsResult, error) {
	item, err := s.requireProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	if err = s.requireProviderManagement(ctx, *item); err != nil {
		return nil, err
	}
	models, err := s.fetchRemoteModels(ctx, *item)
	if err != nil {
		return nil, err
	}
	now := s.now()
	entities := make([]providerstore.ModelEntity, 0, len(models))
	for _, model := range models {
		modelID := normalizeModelID(model.ID)
		if modelID == "" {
			continue
		}
		capabilities, category, contextWindow, maxOutput := model.modelCard()
		entities = append(entities, providerstore.ModelEntity{
			ID:                       s.idFactory("provider_model"),
			ProviderID:               item.ID,
			ModelID:                  modelID,
			DisplayName:              modelDisplayName(modelID, model.DisplayName),
			Category:                 category,
			Enabled:                  false,
			IsDefault:                false,
			CapabilitiesAutoJSON:     encodeModelCapabilities(capabilities),
			CapabilitiesOverrideJSON: "{}",
			ContextWindow:            contextWindow,
			MaxOutputTokens:          maxOutput,
			ProviderOptionsJSON:      "{}",
			LastSeenAt:               now,
			CreatedAt:                now,
			UpdatedAt:                now,
		})
	}
	if len(entities) == 0 {
		return nil, errors.New("远端没有返回可用模型")
	}
	if err = s.repository.UpsertModels(ctx, entities); err != nil {
		return nil, err
	}
	if err = s.autoDefaultDiscoveredModel(ctx, *item, models); err != nil {
		return nil, err
	}
	saved, err := s.modelsForRecord(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	return &FetchModelsResult{
		Provider: item.Provider,
		Models:   saved,
		Count:    len(saved),
	}, nil
}

// UpdateModel 更新模型开关、能力覆盖和 Provider 原生 options。
func (s *Service) UpdateModel(ctx context.Context, provider string, modelID string, input UpdateModelInput) (*ModelRecord, error) {
	item, err := s.requireProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	if err = s.requireProviderManagement(ctx, *item); err != nil {
		return nil, err
	}
	modelID = normalizeModelID(modelID)
	if modelID == "" {
		return nil, errors.New("model_id 不能为空")
	}
	model, err := s.getModelByID(ctx, item.ID, modelID)
	if err != nil {
		return nil, err
	}
	if input.IsDefault {
		candidate := providerstore.ModelEntity{
			ModelID:                  modelID,
			DisplayName:              modelID,
			CapabilitiesAutoJSON:     encodeModelCapabilities(ModelCapabilities{}),
			CapabilitiesOverrideJSON: encodeModelCapabilities(input.CapabilitiesOverride),
		}
		if model != nil {
			candidate = *model
			normalizeModelEntityIdentity(&candidate, modelID)
			candidate.CapabilitiesOverrideJSON = encodeModelCapabilities(input.CapabilitiesOverride)
		}
		if !canSetDefaultModel(*item, candidate) {
			return nil, fmt.Errorf("provider=%s 暂不可设置默认模型", item.Provider)
		}
	}
	if model == nil {
		capabilities, category, contextWindow, maxOutput := defaultModelCard()
		now := s.now()
		if input.ContextWindow != nil {
			contextWindow = input.ContextWindow
		}
		if input.MaxOutputTokens != nil {
			maxOutput = input.MaxOutputTokens
		}
		model = &providerstore.ModelEntity{
			ID:                       s.idFactory("provider_model"),
			ProviderID:               item.ID,
			ModelID:                  modelID,
			DisplayName:              modelID,
			Category:                 category,
			Enabled:                  input.Enabled || input.IsDefault,
			IsDefault:                input.IsDefault,
			CapabilitiesAutoJSON:     encodeModelCapabilities(capabilities),
			CapabilitiesOverrideJSON: encodeModelCapabilities(input.CapabilitiesOverride),
			ContextWindow:            contextWindow,
			MaxOutputTokens:          maxOutput,
			ProviderOptionsJSON:      encodeProviderOptions(input.ProviderOptions),
			LastSeenAt:               now,
			CreatedAt:                now,
			UpdatedAt:                now,
		}
		if err = s.repository.UpsertModels(ctx, []providerstore.ModelEntity{*model}); err != nil {
			return nil, err
		}
	} else {
		normalizeModelEntityIdentity(model, modelID)
		if model.IsDefault && !input.Enabled && !input.IsDefault {
			return nil, fmt.Errorf("默认模型不能禁用: %s", modelID)
		}
		model.Enabled = input.Enabled || input.IsDefault || model.IsDefault
		model.IsDefault = input.IsDefault || model.IsDefault
		model.CapabilitiesOverrideJSON = encodeModelCapabilities(input.CapabilitiesOverride)
		model.ContextWindow = input.ContextWindow
		model.MaxOutputTokens = input.MaxOutputTokens
		model.ProviderOptionsJSON = encodeProviderOptions(input.ProviderOptions)
		model.UpdatedAt = s.now()
		if err = s.repository.UpdateModel(ctx, *model); err != nil {
			return nil, err
		}
	}
	if input.IsDefault {
		if err = s.repository.UpdateDefaultModel(ctx, item.ID, model.ModelID, s.now()); err != nil {
			return nil, err
		}
	}
	updated, err := s.getModelByID(ctx, item.ID, modelID)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, fmt.Errorf("模型不存在: %s", modelID)
	}
	record := toModelRecord(*updated)
	return &record, nil
}

// SetDefaultModel 把指定模型设置为当前 Provider 类型的默认模型，不改写模型卡其它字段。
func (s *Service) SetDefaultModel(ctx context.Context, provider string, modelID string) (*ModelRecord, error) {
	item, err := s.requireProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	if err = s.requireProviderManagement(ctx, *item); err != nil {
		return nil, err
	}
	modelID = normalizeModelID(modelID)
	if modelID == "" {
		return nil, errors.New("model_id 不能为空")
	}
	model, err := s.getModelByID(ctx, item.ID, modelID)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, fmt.Errorf("模型不存在: %s", modelID)
	}
	identityChanged := normalizeModelEntityIdentity(model, modelID)
	if !canSetDefaultModel(*item, *model) {
		return nil, fmt.Errorf("provider=%s 暂不可设置默认模型", item.Provider)
	}
	if identityChanged {
		model.UpdatedAt = s.now()
		if err = s.repository.UpdateModel(ctx, *model); err != nil {
			return nil, err
		}
	}
	now := s.now()
	if err = s.repository.UpdateDefaultModel(ctx, item.ID, model.ModelID, now); err != nil {
		return nil, err
	}
	updated, err := s.getModelByID(ctx, item.ID, modelID)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, fmt.Errorf("模型不存在: %s", modelID)
	}
	record := toModelRecord(*updated)
	return &record, nil
}
