package provider

import (
	"context"
	"fmt"
	"strings"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

func (s *Service) defaultRuntimeSelection(ctx context.Context) (*providerModelTarget, error) {
	return s.defaultRuntimeSelectionForRuntime(ctx, "claude")
}

func (s *Service) defaultRuntimeSelectionForRuntime(ctx context.Context, runtimeKind string) (*providerModelTarget, error) {
	items, err := s.listAndNormalize(ctx)
	if err != nil {
		return nil, err
	}
	runtimeKind = normalizeRuntimeKind(runtimeKind)
	for _, item := range items {
		if !item.Enabled || !isAgentRuntimeProviderForRuntime(item, runtimeKind) {
			continue
		}
		models, modelErr := s.repository.ListModelsByProviderID(ctx, item.ID)
		if modelErr != nil {
			return nil, modelErr
		}
		for _, model := range models {
			if model.Enabled && model.IsDefault && modelUsableForProviderKind(item, model, ProviderKindLLM) {
				return &providerModelTarget{provider: item, model: model}, nil
			}
		}
	}
	return nil, nil
}

func (s *Service) defaultImageSelection(ctx context.Context) (*providerModelTarget, error) {
	items, err := s.listAndNormalize(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		imageProvider, ok := imageRuntimeProvider(item)
		if !item.Enabled || !ok {
			continue
		}
		models, modelErr := s.repository.ListModelsByProviderID(ctx, item.ID)
		if modelErr != nil {
			return nil, modelErr
		}
		for _, model := range models {
			if model.Enabled && model.IsDefault && modelUsableForProviderKind(item, model, ProviderKindImageGeneration) {
				return &providerModelTarget{provider: imageProvider, model: model}, nil
			}
		}
	}
	return nil, nil
}

func modelSelectionFromTarget(target providerModelTarget) ModelSelection {
	modelID := normalizeModelID(target.model.ModelID)
	return ModelSelection{
		Provider:            target.provider.Provider,
		ProviderDisplayName: target.provider.DisplayName,
		Model:               modelID,
		ModelDisplayName:    modelDisplayName(target.model.ModelID, target.model.DisplayName),
	}
}

func (s *Service) enabledModelOptions(ctx context.Context, item providerstore.Entity) ([]ModelOption, error) {
	return s.enabledModelOptionsForKind(ctx, item, item.ProviderKind)
}

func (s *Service) enabledModelOptionsForKind(
	ctx context.Context,
	item providerstore.Entity,
	providerKind string,
) ([]ModelOption, error) {
	models, err := s.repository.ListModelsByProviderID(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	result := make([]ModelOption, 0, len(models))
	for _, model := range models {
		if !model.Enabled || strings.TrimSpace(model.ModelID) == "" {
			continue
		}
		if !modelUsableForProviderKind(item, model, providerKind) {
			continue
		}
		modelID := normalizeModelID(model.ModelID)
		result = append(result, ModelOption{
			ModelID:     modelID,
			DisplayName: modelDisplayName(model.ModelID, model.DisplayName),
			IsDefault:   model.IsDefault,
		})
	}
	return result, nil
}

func modelUsableForProviderKind(
	item providerstore.Entity,
	model providerstore.ModelEntity,
	providerKind string,
) bool {
	switch normalizeProviderKind(providerKind) {
	case ProviderKindImageGeneration:
		if item.ProviderKind != ProviderKindImageGeneration && !providerSupportsImageGeneration(item) {
			return false
		}
		if item.ProviderKind == ProviderKindImageGeneration && !imageProviderRequiresModelFilter(item) {
			return true
		}
		return modelHasImageOutputCapability(model)
	case ProviderKindLLM:
		return !modelHasImageOutputCapability(model)
	default:
		return true
	}
}

func modelHasImageOutputCapability(model providerstore.ModelEntity) bool {
	overrideCapabilities := decodeModelCapabilities(model.CapabilitiesOverrideJSON)
	if overrideCapabilities.ImageOutput != nil {
		return *overrideCapabilities.ImageOutput
	}
	autoCapabilities := decodeModelCapabilities(model.CapabilitiesAutoJSON)
	return autoCapabilities.ImageOutput != nil && *autoCapabilities.ImageOutput
}

func modelHasReasoningCapability(model providerstore.ModelEntity) bool {
	overrideCapabilities := decodeModelCapabilities(model.CapabilitiesOverrideJSON)
	if overrideCapabilities.Reasoning != nil {
		return *overrideCapabilities.Reasoning
	}
	autoCapabilities := decodeModelCapabilities(model.CapabilitiesAutoJSON)
	return autoCapabilities.Reasoning != nil && *autoCapabilities.Reasoning
}

func imageProviderRequiresModelFilter(item providerstore.Entity) bool {
	preset := resolvePreset(item.PresetKey)
	if preset.PresetKey == presetCustom {
		return false
	}
	hasLLM := false
	hasImage := false
	for _, format := range preset.Formats {
		switch normalizeProviderKind(format.ProviderKind) {
		case ProviderKindLLM:
			hasLLM = true
		case ProviderKindImageGeneration:
			hasImage = true
		}
	}
	return hasLLM && hasImage
}

func canSetDefaultModel(item providerstore.Entity, model providerstore.ModelEntity) bool {
	if !item.Enabled {
		return false
	}
	switch item.ProviderKind {
	case ProviderKindLLM:
		if isAnyAgentRuntimeProvider(item) && modelUsableForProviderKind(item, model, ProviderKindLLM) {
			return true
		}
		if !providerSupportsImageGeneration(item) {
			return false
		}
		return modelUsableForProviderKind(item, model, ProviderKindImageGeneration)
	case ProviderKindImageGeneration:
		return true
	default:
		return false
	}
}

func (s *Service) defaultOrFirstEnabledModel(
	ctx context.Context,
	providerID string,
) (*providerstore.ModelEntity, error) {
	models, err := s.repository.ListModelsByProviderID(ctx, providerID)
	if err != nil {
		return nil, err
	}
	for _, model := range models {
		if model.Enabled && model.IsDefault {
			return &model, nil
		}
	}
	for _, model := range models {
		if model.Enabled {
			return &model, nil
		}
	}
	return nil, nil
}

func (s *Service) defaultOrFirstEnabledModelForKind(
	ctx context.Context,
	item providerstore.Entity,
	providerKind string,
) (*providerstore.ModelEntity, error) {
	models, err := s.repository.ListModelsByProviderID(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	for _, model := range models {
		if model.Enabled && model.IsDefault && modelUsableForProviderKind(item, model, providerKind) {
			return &model, nil
		}
	}
	for _, model := range models {
		if model.Enabled && modelUsableForProviderKind(item, model, providerKind) {
			return &model, nil
		}
	}
	return nil, nil
}

func (s *Service) resolveMissingExplicitModel(ctx context.Context, providerID string) (string, error) {
	model, err := s.defaultOrFirstEnabledModel(ctx, providerID)
	if err != nil {
		return "", err
	}
	if model == nil {
		return "", nil
	}
	return normalizeModelID(model.ModelID), nil
}

func (s *Service) replacementRuntimeSelectionForDelete(
	ctx context.Context,
	deleting providerstore.Entity,
) (*providerModelTarget, error) {
	var items []providerstore.Entity
	var err error
	if deleting.Visibility == providerstore.VisibilityPublic {
		items, err = s.listPublicAndNormalize(ctx)
	} else {
		items, err = s.listAndNormalize(ctx)
	}
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID == deleting.ID || !item.Enabled || !isAgentRuntimeProvider(item) {
			continue
		}
		models, modelErr := s.repository.ListModelsByProviderID(ctx, item.ID)
		if modelErr != nil {
			return nil, modelErr
		}
		for _, model := range models {
			if model.Enabled && model.IsDefault {
				return &providerModelTarget{provider: item, model: model}, nil
			}
		}
	}
	for _, item := range items {
		if item.ID == deleting.ID || !item.Enabled || !isAgentRuntimeProvider(item) {
			continue
		}
		model, modelErr := s.defaultOrFirstEnabledModel(ctx, item.ID)
		if modelErr != nil {
			return nil, modelErr
		}
		if model != nil {
			return &providerModelTarget{provider: item, model: *model}, nil
		}
	}
	return nil, fmt.Errorf("provider=%s 仍被 Agent 使用，但没有可替换的默认模型", deleting.Provider)
}
