package imagegen

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

func (s *Service) resolveImageConfig(ctx context.Context, provider string, model string) (*providercfg.ImageConfig, error) {
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)
	if provider != "" || model != "" || s.prefs == nil {
		if model != "" {
			if resolver, ok := s.providers.(providerModelResolver); ok {
				return resolver.ResolveImageModelConfig(ctx, provider, model)
			}
			return nil, errors.New("图片生成 Provider 不支持显式 model 选择")
		}
		return s.providers.ResolveImageConfig(ctx, provider)
	}
	prefs, err := s.prefs.Get(ctx, authctx.OwnerUserID(ctx))
	if err != nil {
		return nil, err
	}
	selection := prefs.DefaultImageModelSelection
	selection.Provider = strings.TrimSpace(selection.Provider)
	selection.Model = strings.TrimSpace(selection.Model)
	if selection.Provider == "" || selection.Model == "" {
		return s.providers.ResolveImageConfig(ctx, "")
	}
	if resolver, ok := s.providers.(providerModelResolver); ok {
		return resolver.ResolveImageModelConfig(ctx, selection.Provider, selection.Model)
	}
	return s.providers.ResolveImageConfig(ctx, selection.Provider)
}
