package imagegen

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

const (
	defaultSize         = "1024x1024"
	defaultOutputFormat = "png"
	maxImageBytes       = 25 * 1024 * 1024
	requestTimeout      = 120 * time.Second
	defaultMaxAttempts  = 3
)

// GenerateInput 表示图片生成请求。
type GenerateInput struct {
	Provider          string
	Model             string
	Prompt            string
	WorkspacePath     string
	Size              string
	Quality           string
	Background        string
	OutputFormat      string
	OutputCompression *int
	FileName          string
}

// EditInput 表示图片编辑请求。
type EditInput struct {
	Provider          string
	Model             string
	Prompt            string
	WorkspacePath     string
	ImagePath         string
	MaskPath          string
	Size              string
	Quality           string
	OutputFormat      string
	OutputCompression *int
	FileName          string
}

// Result 表示已落盘的图片生成结果。
type Result struct {
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	Path          string `json:"path"`
	MIMEType      string `json:"mime_type"`
	Size          string `json:"size,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
	Markdown      string `json:"markdown"`
}

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
	providers     ProviderResolver
	prefs         preferencesService
	workspaceRoot string
	now           func() time.Time
	client        *http.Client
}

// NewService 创建图片生成服务。
func NewService(providers ProviderResolver, workspaceRoot string) *Service {
	return &Service{
		providers:     providers,
		workspaceRoot: strings.TrimSpace(workspaceRoot),
		now:           func() time.Time { return time.Now().UTC() },
		client:        &http.Client{Timeout: requestTimeout},
	}
}

// SetPreferences 注入用户偏好服务，用于解析默认生图模型。
func (s *Service) SetPreferences(prefs preferencesService) {
	s.prefs = prefs
}

func (s *Service) resolveImageConfig(ctx context.Context, provider, model string) (*providercfg.ImageConfig, error) {
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
	relativePath, err := s.writeImage(ctx, normalized, payload, mimeType)
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
	relativePath, err := s.writeImage(ctx, generateInput, payload, mimeType)
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

func (s *Service) openWorkspace(
	ctx context.Context,
	workspacePath string,
	create bool,
) (*confinedfs.Root, error) {
	return workspacestore.New(s.workspaceRoot).OpenOwnerWorkspacePath(
		authctx.OwnerUserID(ctx),
		workspacePath,
		create,
	)
}
