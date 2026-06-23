package imagegen

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

const (
	defaultSize         = "1024x1024"
	defaultOutputFormat = "png"
	maxImageBytes       = 25 * 1024 * 1024
	requestTimeout      = 120 * time.Second
	defaultMaxAttempts  = 3
)

// ProviderResolver 是图片生成服务依赖的 provider 配置解析子集。
type ProviderResolver interface {
	ResolveImageConfig(ctx context.Context, provider string) (*providercfg.ImageConfig, error)
}

type providerModelResolver interface {
	ResolveImageModelConfig(ctx context.Context, provider string, model string) (*providercfg.ImageConfig, error)
}

type preferencesService interface {
	Get(context.Context, string) (preferencessvc.Preferences, error)
}

// Service 提供 Provider 驱动的图片生成能力。
type Service struct {
	providers ProviderResolver
	prefs     preferencesService
	now       func() time.Time
	client    *http.Client
}

// NewService 创建图片生成服务。
func NewService(providers ProviderResolver) *Service {
	return &Service{
		providers: providers,
		now:       func() time.Time { return time.Now().UTC() },
		client:    &http.Client{Timeout: requestTimeout},
	}
}

// SetPreferences 注入用户偏好服务，用于解析默认生图模型。
func (s *Service) SetPreferences(prefs preferencesService) {
	s.prefs = prefs
}

// GenerateImage 调用图片生成 Provider 并保存图片。
func (s *Service) GenerateImage(ctx context.Context, input GenerateInput) (*Result, []byte, error) {
	if s == nil || s.providers == nil {
		return nil, nil, errors.New("图片生成服务未初始化")
	}
	normalized, err := normalizeInput(input)
	if err != nil {
		return nil, nil, err
	}
	config, err := s.resolveImageConfig(ctx, normalized.Provider, normalized.Model)
	if err != nil {
		return nil, nil, err
	}
	normalized = applyGenerateProviderDefaults(config, normalized)
	normalized.Size = normalizeProviderImageSize(config, normalized.Size)
	payload, revisedPrompt, mimeType, err := s.callGenerateProvider(ctx, config, normalized)
	if err != nil {
		return nil, nil, err
	}
	if len(payload) == 0 {
		return nil, nil, errors.New("图片生成接口未返回图片数据")
	}
	if len(payload) > maxImageBytes {
		return nil, nil, fmt.Errorf("图片过大: %d bytes", len(payload))
	}
	if mimeType == "" {
		mimeType = detectMIMEType(payload, normalized.OutputFormat)
	}
	relativePath, err := s.writeImage(normalized, payload, mimeType)
	if err != nil {
		return nil, nil, err
	}
	result := &Result{
		Provider:      config.Provider,
		Model:         config.Model,
		Path:          relativePath,
		MIMEType:      mimeType,
		Size:          normalized.Size,
		RevisedPrompt: revisedPrompt,
		Markdown:      fmt.Sprintf("![generated image](%s)", relativePath),
	}
	return result, payload, nil
}

// EditImage 调用图片编辑 Provider 并保存图片。
func (s *Service) EditImage(ctx context.Context, input EditInput) (*Result, []byte, error) {
	if s == nil || s.providers == nil {
		return nil, nil, errors.New("图片生成服务未初始化")
	}
	normalized, err := normalizeEditInput(input)
	if err != nil {
		return nil, nil, err
	}
	config, err := s.resolveImageConfig(ctx, normalized.Provider, normalized.Model)
	if err != nil {
		return nil, nil, err
	}
	normalized.Size = normalizeProviderImageSize(config, normalized.Size)
	payload, revisedPrompt, mimeType, err := s.callEditProvider(ctx, config, normalized)
	if err != nil {
		return nil, nil, err
	}
	if len(payload) == 0 {
		return nil, nil, errors.New("图片编辑接口未返回图片数据")
	}
	if len(payload) > maxImageBytes {
		return nil, nil, fmt.Errorf("图片过大: %d bytes", len(payload))
	}
	if mimeType == "" {
		mimeType = detectMIMEType(payload, normalized.OutputFormat)
	}
	generateInput := GenerateInput{
		Prompt:        normalized.Prompt,
		WorkspacePath: normalized.WorkspacePath,
		OutputFormat:  normalized.OutputFormat,
		FileName:      normalized.FileName,
	}
	relativePath, err := s.writeImage(generateInput, payload, mimeType)
	if err != nil {
		return nil, nil, err
	}
	result := &Result{
		Provider:      config.Provider,
		Model:         config.Model,
		Path:          relativePath,
		MIMEType:      mimeType,
		Size:          normalized.Size,
		RevisedPrompt: revisedPrompt,
		Markdown:      fmt.Sprintf("![edited image](%s)", relativePath),
	}
	return result, payload, nil
}
