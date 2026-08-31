// INPUT: Provider 模型发现、模型 patch、默认模型目标与期望 configuration_version。
// OUTPUT: 单次 Provider 聚合事务后的模型目录和模型记录。
// POS: 模型、默认选择与 Provider configuration_version 的服务级一致性边界。
package provider

import (
	"context"
	"errors"
	"fmt"
	"net/url"

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
	return s.fetchModelsForItem(ctx, *item, item.ConfigurationVersion)
}

// FetchModelsAtVersion 在远端读取后，以 Provider 聚合版本 CAS 合并模型目录。
func (s *Service) FetchModelsAtVersion(
	ctx context.Context,
	provider string,
	expectedVersion int64,
) (*FetchModelsResult, error) {
	item, err := s.requireProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	if err = s.requireProviderManagement(ctx, *item); err != nil {
		return nil, err
	}
	return s.fetchModelsForItem(ctx, *item, expectedVersion)
}

// FetchPublicModels 从公共 Provider 拉取模型列表。
func (s *Service) FetchPublicModels(ctx context.Context, provider string) (*FetchModelsResult, error) {
	item, err := s.requirePublicProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	return s.fetchModelsForItem(ctx, *item, item.ConfigurationVersion)
}

func (s *Service) fetchModelsForItem(
	ctx context.Context,
	item providerstore.Entity,
	expectedVersion int64,
) (*FetchModelsResult, error) {
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
		capabilities, category, contextWindow, maxOutput := model.modelCard(item.ProviderKind)
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
	shouldAutoDefault, err := s.shouldAutoDefaultDiscoveredModel(ctx, item)
	if err != nil {
		return nil, err
	}
	_, err = s.repository.WithProviderMutation(
		ctx,
		item.ID,
		expectedVersion,
		func(mutation *providerstore.Mutation) error {
			if upsertErr := mutation.UpsertModels(ctx, entities); upsertErr != nil {
				return upsertErr
			}
			if !shouldAutoDefault {
				return nil
			}
			hasDefault, defaultErr := mutation.HasDefaultModelInScope(ctx)
			if defaultErr != nil || hasDefault {
				return defaultErr
			}
			savedModels, listErr := mutation.ListModels(ctx)
			if listErr != nil {
				return listErr
			}
			modelID := firstRemoteModelID(models)
			if preferred := preferredEnabledModel(savedModels, nil); preferred != nil {
				modelID = normalizeModelID(preferred.ModelID)
			}
			if modelID == "" {
				return nil
			}
			if defaultErr = mutation.UpdateDefaultModel(ctx, modelID, s.now()); defaultErr != nil {
				return defaultErr
			}
			s.loggerFor(ctx).Info(
				"自动设置 Provider 默认模型",
				"provider", item.Provider,
				"model", modelID,
			)
			return nil
		},
	)
	if err != nil {
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
	return s.updateModelForItem(ctx, *item, modelID, input, item.ConfigurationVersion)
}

// UpdateModelAtVersion 以 Provider 聚合版本 CAS 更新模型卡。
func (s *Service) UpdateModelAtVersion(
	ctx context.Context,
	provider string,
	modelID string,
	input UpdateModelInput,
	expectedVersion int64,
) (*ModelRecord, error) {
	item, err := s.requireProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	if err = s.requireProviderManagement(ctx, *item); err != nil {
		return nil, err
	}
	return s.updateModelForItem(ctx, *item, modelID, input, expectedVersion)
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
	return s.updateModelForItem(ctx, *item, modelID, input, item.ConfigurationVersion)
}

// DeleteModel 删除当前用户可管理 Provider 下的模型卡。
func (s *Service) DeleteModel(ctx context.Context, provider string, modelID string) (*DeleteModelResult, error) {
	item, err := s.requireProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	if err = s.requireProviderManagement(ctx, *item); err != nil {
		return nil, err
	}
	return s.deleteModelForItem(ctx, *item, modelID, item.ConfigurationVersion)
}

// DeletePublicModel 删除公共 Provider 下的模型卡。
func (s *Service) DeletePublicModel(
	ctx context.Context,
	provider string,
	modelID string,
) (*DeleteModelResult, error) {
	item, err := s.requirePublicProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	return s.deleteModelForItem(ctx, *item, modelID, item.ConfigurationVersion)
}

func (s *Service) deleteModelForItem(
	ctx context.Context,
	item providerstore.Entity,
	modelID string,
	expectedVersion int64,
) (*DeleteModelResult, error) {
	modelID = normalizeModelID(modelID)
	if modelID == "" {
		return nil, errors.New("model_id 不能为空")
	}
	result := &DeleteModelResult{Provider: item.Provider, Model: modelID}
	_, err := s.repository.WithProviderMutation(
		ctx,
		item.ID,
		expectedVersion,
		func(mutation *providerstore.Mutation) error {
			model, loadErr := getMutationModel(ctx, mutation, modelID)
			if loadErr != nil {
				return loadErr
			}
			if model == nil {
				return ErrModelNotFound
			}
			if model.IsDefault {
				return fmt.Errorf("默认模型不能删除: %s", model.ModelID)
			}
			result.Model = model.ModelID
			return mutation.DeleteModel(ctx, *model)
		},
	)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) updateModelForItem(
	ctx context.Context,
	item providerstore.Entity,
	modelID string,
	input UpdateModelInput,
	expectedVersion int64,
) (*ModelRecord, error) {
	update := modelUpdate{
		service: s,
		ctx:     ctx,
		item:    item,
		modelID: normalizeModelID(modelID),
		input:   input,
	}
	return update.run(expectedVersion)
}

type modelUpdate struct {
	service  *Service
	ctx      context.Context
	item     providerstore.Entity
	modelID  string
	input    UpdateModelInput
	model    *providerstore.ModelEntity
	mutation *providerstore.Mutation
}

func (u *modelUpdate) run(expectedVersion int64) (*ModelRecord, error) {
	if u.modelID == "" {
		return nil, errors.New("model_id 不能为空")
	}
	_, err := u.service.repository.WithProviderMutation(
		u.ctx,
		u.item.ID,
		expectedVersion,
		func(mutation *providerstore.Mutation) error {
			u.mutation = mutation
			if loadErr := u.load(); loadErr != nil {
				return loadErr
			}
			if validateErr := u.validateDefaultCandidate(); validateErr != nil {
				return validateErr
			}
			if persistErr := u.persist(); persistErr != nil {
				return persistErr
			}
			return u.promoteDefault()
		},
	)
	if err != nil {
		return nil, err
	}
	return u.loadRecord()
}

func (u *modelUpdate) load() error {
	model, err := getMutationModel(u.ctx, u.mutation, u.modelID)
	u.model = model
	return err
}

func getMutationModel(
	ctx context.Context,
	mutation *providerstore.Mutation,
	modelID string,
) (*providerstore.ModelEntity, error) {
	model, err := mutation.GetModel(ctx, modelID)
	if err == nil && model == nil {
		escaped := url.PathEscape(modelID)
		if escaped != modelID {
			model, err = mutation.GetModel(ctx, escaped)
		}
	}
	return model, err
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
		return u.mutation.UpsertModels(u.ctx, []providerstore.ModelEntity{*u.model})
	}
	if err := u.applyToExistingModel(); err != nil {
		return err
	}
	return u.mutation.UpdateModel(u.ctx, *u.model)
}

func (u *modelUpdate) newModel() *providerstore.ModelEntity {
	capabilities, category, contextWindow, maxOutput := defaultModelCard(u.modelID, u.item.ProviderKind)
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
	return u.mutation.UpdateDefaultModel(u.ctx, u.model.ModelID, u.service.now())
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
	return s.setDefaultModelForItem(ctx, *item, modelID, item.ConfigurationVersion)
}

// SetPublicDefaultModel 把指定公共模型设置为订阅用户的默认运行模型。
func (s *Service) SetPublicDefaultModel(ctx context.Context, provider string, modelID string) (*ModelRecord, error) {
	item, err := s.requirePublicProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	return s.setDefaultModelForItem(ctx, *item, modelID, item.ConfigurationVersion)
}

// SetDefaultModelAtVersion 以 Provider 聚合版本 CAS 切换默认模型。
func (s *Service) SetDefaultModelAtVersion(
	ctx context.Context,
	provider string,
	modelID string,
	expectedVersion int64,
) (*ModelRecord, error) {
	item, err := s.requireProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	if err = s.requireProviderManagement(ctx, *item); err != nil {
		return nil, err
	}
	return s.setDefaultModelForItem(ctx, *item, modelID, expectedVersion)
}

func (s *Service) setDefaultModelForItem(
	ctx context.Context,
	item providerstore.Entity,
	modelID string,
	expectedVersion int64,
) (*ModelRecord, error) {
	modelID = normalizeModelID(modelID)
	if modelID == "" {
		return nil, errors.New("model_id 不能为空")
	}
	_, err := s.repository.WithProviderMutation(
		ctx,
		item.ID,
		expectedVersion,
		func(mutation *providerstore.Mutation) error {
			model, loadErr := mutation.GetModel(ctx, modelID)
			if loadErr != nil {
				return loadErr
			}
			if model == nil {
				return fmt.Errorf("模型不存在: %s", modelID)
			}
			identityChanged := normalizeModelEntityIdentity(model, modelID)
			if !canSetDefaultModel(item, *model) {
				return fmt.Errorf("provider=%s 暂不可设置默认模型", item.Provider)
			}
			if identityChanged {
				model.UpdatedAt = s.now()
				if updateErr := mutation.UpdateModel(ctx, *model); updateErr != nil {
					return updateErr
				}
			}
			return mutation.UpdateDefaultModel(ctx, model.ModelID, s.now())
		},
	)
	if err != nil {
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
