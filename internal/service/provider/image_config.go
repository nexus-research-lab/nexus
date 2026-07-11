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
	resolver := imageConfigResolver{
		service:        s,
		ctx:            ctx,
		items:          items,
		targetProvider: targetProvider,
		targetModel:    normalizeModelID(model),
	}
	return resolver.resolve()
}

type imageConfigResolver struct {
	service        *Service
	ctx            context.Context
	items          []providerstore.Entity
	targetProvider string
	targetModel    string
	provider       *providerstore.Entity
	model          *providerstore.ModelEntity
}

func (r *imageConfigResolver) resolve() (*ImageConfig, error) {
	if err := r.selectProvider(); err != nil {
		return nil, err
	}
	if err := r.validateProvider(); err != nil {
		return nil, err
	}
	if err := r.selectModel(); err != nil {
		return nil, err
	}
	if err := r.validateModelAndCredentials(); err != nil {
		return nil, err
	}
	return r.config(), nil
}

func (r *imageConfigResolver) selectProvider() error {
	if r.targetProvider != "" {
		return r.selectExplicitProvider()
	}
	if r.targetModel != "" {
		return errors.New("指定图片 model 时必须同时指定 provider")
	}
	target, err := r.service.selectImageTarget(r.ctx, r.items, true)
	if err != nil || target == nil {
		return err
	}
	r.provider = &target.provider
	r.model = &target.model
	return nil
}

func (r *imageConfigResolver) selectExplicitProvider() error {
	for _, item := range r.items {
		if item.Provider != r.targetProvider {
			continue
		}
		if imageItem, ok := imageRuntimeProvider(item); ok {
			r.provider = &imageItem
			return nil
		}
	}
	return fmt.Errorf("provider 不存在: %s", r.targetProvider)
}

func (r *imageConfigResolver) validateProvider() error {
	if r.provider == nil {
		return errors.New("未配置可用的图片生成 Provider，请先到 Settings 添加 image_generation Provider")
	}
	if !r.provider.Enabled {
		return fmt.Errorf("provider=%s 已禁用", r.provider.Provider)
	}
	if r.provider.ProviderKind != ProviderKindImageGeneration {
		return fmt.Errorf("provider=%s 不是图片生成 Provider", r.provider.Provider)
	}
	return nil
}

func (r *imageConfigResolver) selectModel() error {
	if r.model != nil {
		return nil
	}
	var err error
	if r.targetModel != "" {
		r.model, err = r.service.getModelByID(r.ctx, r.provider.ID, r.targetModel)
		return err
	}
	r.model, err = r.service.defaultOrFirstEnabledModelForKind(r.ctx, *r.provider, ProviderKindImageGeneration)
	return err
}

func (r *imageConfigResolver) validateModelAndCredentials() error {
	missing := make([]string, 0, 3)
	if r.provider.AuthToken == "" {
		missing = append(missing, "auth_token")
	}
	if r.provider.BaseURL == "" {
		missing = append(missing, "base_url")
	}
	if r.model == nil {
		missing = append(missing, "model")
	}
	if len(missing) > 0 {
		return fmt.Errorf("provider=%s 图片生成配置不完整: %s", r.provider.Provider, strings.Join(missing, ", "))
	}
	if !modelUsableForProviderKind(*r.provider, *r.model, ProviderKindImageGeneration) {
		return fmt.Errorf("provider=%s model=%s 不是图片生成模型", r.provider.Provider, r.model.ModelID)
	}
	if !r.model.Enabled {
		return fmt.Errorf("provider=%s model=%s 已禁用", r.provider.Provider, r.model.ModelID)
	}
	return nil
}

func (r *imageConfigResolver) config() *ImageConfig {
	return &ImageConfig{
		Provider:        r.provider.Provider,
		DisplayName:     r.provider.DisplayName,
		APIFormat:       r.provider.APIFormat,
		AuthToken:       r.provider.AuthToken,
		BaseURL:         r.provider.BaseURL,
		Model:           normalizeModelID(r.model.ModelID),
		ProviderOptions: decodeProviderOptions(r.model.ProviderOptionsJSON),
	}
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

// selectImageTarget 单次扫描图片 Provider：默认模型优先，允许时保留首个可用模型作为回退。
func (s *Service) selectImageTarget(
	ctx context.Context,
	items []providerstore.Entity,
	allowFallback bool,
) (*providerModelTarget, error) {
	var fallback *providerModelTarget
	for _, item := range items {
		imageItem, ok := imageRuntimeProvider(item)
		if !item.Enabled || !ok {
			continue
		}
		model, err := s.defaultOrFirstEnabledModelForKind(ctx, item, ProviderKindImageGeneration)
		if err != nil {
			return nil, err
		}
		if model == nil {
			continue
		}
		candidate := &providerModelTarget{provider: imageItem, model: *model}
		if model.IsDefault {
			return candidate, nil
		}
		if fallback == nil {
			fallback = candidate
		}
	}
	if allowFallback {
		return fallback, nil
	}
	return nil, nil
}
