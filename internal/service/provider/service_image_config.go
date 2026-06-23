package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

// ResolveImageConfig 解析图片生成最终要使用的 Provider 配置。
func (s *Service) ResolveImageConfig(ctx context.Context, provider string) (*ImageConfig, error) {
	return s.ResolveImageModelConfig(ctx, provider, "")
}

// ResolveImageModelConfig 按显式 Provider/Model 解析图片生成配置。
func (s *Service) ResolveImageModelConfig(ctx context.Context, provider string, model string) (*ImageConfig, error) {
	items, err := s.listAndNormalize(ctx)
	if err != nil {
		return nil, err
	}
	targetProvider, err := NormalizeProvider(provider, true)
	if err != nil {
		return nil, err
	}
	targetModel := normalizeModelID(model)

	var target *providerstore.Entity
	if targetProvider == "" {
		if targetModel != "" {
			return nil, errors.New("指定图片 model 时必须同时指定 provider")
		}
		defaultTarget, defaultErr := s.defaultImageSelection(ctx)
		if defaultErr != nil {
			return nil, defaultErr
		}
		if defaultTarget != nil {
			target = &defaultTarget.provider
			targetModel = defaultTarget.model.ModelID
		}
	}
	if target == nil {
		var selectErr error
		target, selectErr = s.selectImageProvider(ctx, items, targetProvider)
		if selectErr != nil {
			return nil, selectErr
		}
	}
	if target == nil {
		return nil, errors.New("未配置可用的图片生成 Provider，请先到 Settings 添加 image_generation Provider")
	}
	if !target.Enabled {
		return nil, fmt.Errorf("provider=%s 已禁用", target.Provider)
	}
	if target.ProviderKind != ProviderKindImageGeneration {
		return nil, fmt.Errorf("provider=%s 不是图片生成 Provider", target.Provider)
	}

	missing := make([]string, 0, 3)
	if target.AuthToken == "" {
		missing = append(missing, "auth_token")
	}
	if target.BaseURL == "" {
		missing = append(missing, "base_url")
	}
	var modelRecord *providerstore.ModelEntity
	if targetModel != "" {
		modelRecord, err = s.getModelByID(ctx, target.ID, targetModel)
	} else {
		modelRecord, err = s.defaultOrFirstEnabledModelForKind(ctx, *target, ProviderKindImageGeneration)
	}
	if err != nil {
		return nil, err
	}
	if modelRecord == nil {
		missing = append(missing, "model")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("provider=%s 图片生成配置不完整: %s", target.Provider, strings.Join(missing, ", "))
	}
	if !modelUsableForProviderKind(*target, *modelRecord, ProviderKindImageGeneration) {
		return nil, fmt.Errorf("provider=%s model=%s 不是图片生成模型", target.Provider, modelRecord.ModelID)
	}
	if !modelRecord.Enabled {
		return nil, fmt.Errorf("provider=%s model=%s 已禁用", target.Provider, modelRecord.ModelID)
	}
	return &ImageConfig{
		Provider:        target.Provider,
		DisplayName:     target.DisplayName,
		APIFormat:       target.APIFormat,
		AuthToken:       target.AuthToken,
		BaseURL:         target.BaseURL,
		Model:           normalizeModelID(modelRecord.ModelID),
		ProviderOptions: decodeProviderOptions(modelRecord.ProviderOptionsJSON),
	}, nil
}

func providerSupportsImageGeneration(item providerstore.Entity) bool {
	_, ok := imageRuntimeProvider(item)
	return ok
}

func imageRuntimeProvider(item providerstore.Entity) (providerstore.Entity, bool) {
	if item.ProviderKind == ProviderKindImageGeneration {
		return item, true
	}
	preset := resolvePreset(item.PresetKey)
	if preset.PresetKey == presetCustom {
		return providerstore.Entity{}, false
	}
	for _, format := range preset.Formats {
		if normalizeProviderKind(format.ProviderKind) != ProviderKindImageGeneration {
			continue
		}
		imageItem := item
		imageItem.ProviderKind = ProviderKindImageGeneration
		imageItem.APIFormat = normalizeAPIFormat(format.APIFormat)
		imageItem.BaseURL = strings.TrimSpace(format.BaseURL)
		imageItem.ModelsPath = strings.TrimSpace(format.ModelsPath)
		return imageItem, true
	}
	return providerstore.Entity{}, false
}

func (s *Service) selectImageProvider(
	ctx context.Context,
	items []providerstore.Entity,
	targetProvider string,
) (*providerstore.Entity, error) {
	if targetProvider != "" {
		for index := range items {
			if items[index].Provider != targetProvider {
				continue
			}
			if imageItem, ok := imageRuntimeProvider(items[index]); ok {
				return &imageItem, nil
			}
		}
		return nil, fmt.Errorf("provider 不存在: %s", targetProvider)
	}
	for index := range items {
		if !items[index].Enabled {
			continue
		}
		imageItem, ok := imageRuntimeProvider(items[index])
		if !ok {
			continue
		}
		model, err := s.defaultOrFirstEnabledModelForKind(ctx, items[index], ProviderKindImageGeneration)
		if err != nil {
			return nil, err
		}
		if model != nil && model.IsDefault {
			return &imageItem, nil
		}
	}
	for index := range items {
		if !items[index].Enabled {
			continue
		}
		if imageItem, ok := imageRuntimeProvider(items[index]); ok {
			model, err := s.defaultOrFirstEnabledModelForKind(ctx, items[index], ProviderKindImageGeneration)
			if err != nil {
				return nil, err
			}
			if model != nil {
				return &imageItem, nil
			}
		}
	}
	return nil, nil
}
