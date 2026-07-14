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
	return s.fetchModelsForItem(ctx, *item)
}

// FetchPublicModels 从公共 Provider 拉取模型列表。
func (s *Service) FetchPublicModels(ctx context.Context, provider string) (*FetchModelsResult, error) {
	item, err := s.requirePublicProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	return s.fetchModelsForItem(ctx, *item)
}

func (s *Service) fetchModelsForItem(ctx context.Context, item providerstore.Entity) (*FetchModelsResult, error) {
	models, err := s.fetchRemoteModels(ctx, item)
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
	if err = s.autoDefaultDiscoveredModel(ctx, item, models); err != nil {
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
	return s.updateModelForItem(ctx, *item, modelID, input)
}

// UpdatePublicModel 更新公共 Provider 的模型卡。
func (s *Service) UpdatePublicModel(
	ctx context.Context,
	provider string,
	modelID string,
	input UpdateModelInput,
) (*ModelRecord, error) {
	item, err := s.requirePublicProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	return s.updateModelForItem(ctx, *item, modelID, input)
}

func (s *Service) updateModelForItem(
	ctx context.Context,
	item providerstore.Entity,
	modelID string,
	input UpdateModelInput,
) (*ModelRecord, error) {
	update := modelUpdate{
		service: s,
		ctx:     ctx,
		item:    item,
		modelID: normalizeModelID(modelID),
		input:   input,
	}
	return update.run()
}

type modelUpdate struct {
	service *Service
	ctx     context.Context
	item    providerstore.Entity
	modelID string
	input   UpdateModelInput
	model   *providerstore.ModelEntity
}

func (u *modelUpdate) run() (*ModelRecord, error) {
	if u.modelID == "" {
		return nil, errors.New("model_id 不能为空")
	}
	if err := u.load(); err != nil {
		return nil, err
	}
	if err := u.validateDefaultCandidate(); err != nil {
		return nil, err
	}
	if err := u.persist(); err != nil {
		return nil, err
	}
	if err := u.promoteDefault(); err != nil {
		return nil, err
	}
	return u.loadRecord()
}

func (u *modelUpdate) load() error {
	model, err := u.service.getModelByID(u.ctx, u.item.ID, u.modelID)
	u.model = model
	return err
}

func (u *modelUpdate) validateDefaultCandidate() error {
	if !u.input.IsDefault {
		return nil
	}
	candidate := u.defaultCandidate()
	if canSetDefaultModel(u.item, candidate) {
		return nil
	}
	return fmt.Errorf("provider=%s 暂不可设置默认模型", u.item.Provider)
}

func (u *modelUpdate) defaultCandidate() providerstore.ModelEntity {
	if u.model != nil {
		candidate := *u.model
		normalizeModelEntityIdentity(&candidate, u.modelID)
		candidate.CapabilitiesOverrideJSON = encodeModelCapabilities(u.input.CapabilitiesOverride)
		return candidate
	}
	return providerstore.ModelEntity{
		ModelID:                  u.modelID,
		DisplayName:              u.modelID,
		CapabilitiesAutoJSON:     encodeModelCapabilities(ModelCapabilities{}),
		CapabilitiesOverrideJSON: encodeModelCapabilities(u.input.CapabilitiesOverride),
	}
}

func (u *modelUpdate) persist() error {
	if u.model == nil {
		u.model = u.newModel()
		return u.service.repository.UpsertModels(u.ctx, []providerstore.ModelEntity{*u.model})
	}
	if err := u.applyToExistingModel(); err != nil {
		return err
	}
	return u.service.repository.UpdateModel(u.ctx, *u.model)
}

func (u *modelUpdate) newModel() *providerstore.ModelEntity {
	capabilities, category, contextWindow, maxOutput := defaultModelCard(u.modelID)
	contextWindow = modelLimitOrDefault(u.input.ContextWindow, contextWindow)
	maxOutput = modelLimitOrDefault(u.input.MaxOutputTokens, maxOutput)
	now := u.service.now()
	return &providerstore.ModelEntity{
		ID:                       u.service.idFactory("provider_model"),
		ProviderID:               u.item.ID,
		ModelID:                  u.modelID,
		DisplayName:              u.modelID,
		Category:                 category,
		Enabled:                  u.input.Enabled || u.input.IsDefault,
		IsDefault:                u.input.IsDefault,
		CapabilitiesAutoJSON:     encodeModelCapabilities(capabilities),
		CapabilitiesOverrideJSON: encodeModelCapabilities(u.input.CapabilitiesOverride),
		ContextWindow:            contextWindow,
		MaxOutputTokens:          maxOutput,
		ProviderOptionsJSON:      encodeProviderOptions(u.input.ProviderOptions),
		LastSeenAt:               now,
		CreatedAt:                now,
		UpdatedAt:                now,
	}
}

func modelLimitOrDefault(value *int, fallback *int) *int {
	if value != nil {
		return value
	}
	return fallback
}

func (u *modelUpdate) applyToExistingModel() error {
	normalizeModelEntityIdentity(u.model, u.modelID)
	if u.model.IsDefault && !u.input.Enabled && !u.input.IsDefault {
		return fmt.Errorf("默认模型不能禁用: %s", u.modelID)
	}
	u.model.Enabled = u.input.Enabled || u.input.IsDefault || u.model.IsDefault
	u.model.IsDefault = u.input.IsDefault || u.model.IsDefault
	u.model.CapabilitiesOverrideJSON = encodeModelCapabilities(u.input.CapabilitiesOverride)
	u.model.ContextWindow = u.input.ContextWindow
	u.model.MaxOutputTokens = u.input.MaxOutputTokens
	u.model.ProviderOptionsJSON = encodeProviderOptions(u.input.ProviderOptions)
	u.model.UpdatedAt = u.service.now()
	return nil
}

func (u *modelUpdate) promoteDefault() error {
	if !u.input.IsDefault {
		return nil
	}
	return u.service.repository.UpdateDefaultModel(u.ctx, u.item.ID, u.model.ModelID, u.service.now())
}

func (u *modelUpdate) loadRecord() (*ModelRecord, error) {
	updated, err := u.service.getModelByID(u.ctx, u.item.ID, u.modelID)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, fmt.Errorf("模型不存在: %s", u.modelID)
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
