package imagegen

import (
	"context"
	"testing"

	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

func TestResolveImageConfigUsesPreferenceDefaultModel(t *testing.T) {
	resolver := &fakeProviderResolver{config: &providercfg.ImageConfig{
		Provider:  "image-provider",
		AuthToken: "token",
		BaseURL:   "https://image.example.com/v1/images",
		Model:     "image-model",
	}}
	service := NewService(resolver)
	service.SetPreferences(fakePreferencesService{prefs: preferencessvc.Preferences{
		DefaultImageModelSelection: preferencessvc.ModelSelection{
			Provider: "image-provider",
			Model:    "image-model",
		},
	}})
	config, err := service.resolveImageConfig(context.Background(), "", "")
	if err != nil {
		t.Fatalf("解析图片默认模型失败: %v", err)
	}
	if config.Model != "image-model" || resolver.provider != "image-provider" || resolver.model != "image-model" {
		t.Fatalf("未使用默认生图模型: config=%+v provider=%s model=%s", config, resolver.provider, resolver.model)
	}
}

func TestResolveImageConfigUsesExplicitProviderModel(t *testing.T) {
	resolver := &fakeProviderResolver{config: &providercfg.ImageConfig{
		Provider:  "image-provider",
		AuthToken: "token",
		BaseURL:   "https://image.example.com/v1/images",
		Model:     "image-model",
	}}
	service := NewService(resolver)

	config, err := service.resolveImageConfig(context.Background(), "image-provider", "image-model")
	if err != nil {
		t.Fatalf("解析显式图片模型失败: %v", err)
	}
	if config.Model != "image-model" || resolver.provider != "image-provider" || resolver.model != "image-model" {
		t.Fatalf("未使用显式图片模型: config=%+v provider=%s model=%s", config, resolver.provider, resolver.model)
	}
}
