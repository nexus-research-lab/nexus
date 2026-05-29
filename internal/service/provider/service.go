package provider

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

var providerPattern = regexp.MustCompile(`[^a-z0-9]+`)

var providerIDCounter atomic.Uint64

const (
	// ProviderKindLLM 表示对话运行时 Provider。
	ProviderKindLLM = "llm"
	// ProviderKindImageGeneration 表示图片生成 Provider。
	ProviderKindImageGeneration = "image_generation"
)

// Service 提供 Provider 配置管理与运行时解析。
type Service struct {
	repository *providerstore.Repository
	now        func() time.Time
	idFactory  func(string) string
	client     *http.Client
	logger     *slog.Logger
}

type providerModelTarget struct {
	provider providerstore.Entity
	model    providerstore.ModelEntity
}

// NewServiceWithDB 使用共享 DB 创建 Provider 配置服务。
func NewServiceWithDB(cfg config.Config, db *sql.DB) *Service {
	return &Service{
		repository: providerstore.NewRepository(cfg, db),
		now:        func() time.Time { return time.Now().UTC() },
		idFactory:  newProviderID,
		client:     &http.Client{Timeout: 30 * time.Second},
		logger:     logx.NewDiscardLogger(),
	}
}

// SetLogger 注入 Provider 服务日志器。
func (s *Service) SetLogger(logger *slog.Logger) {
	if logger == nil {
		s.logger = logx.NewDiscardLogger()
		return
	}
	s.logger = logger
}

// SetHTTPClient 覆盖 Provider 服务使用的 HTTP client，主要用于测试。
func (s *Service) SetHTTPClient(client *http.Client) {
	if client != nil {
		s.client = client
	}
}

func (s *Service) loggerFor(ctx context.Context) *slog.Logger {
	return logx.Resolve(ctx, s.logger)
}

// List 返回完整 Provider 配置列表。
func (s *Service) List(ctx context.Context) ([]Record, error) {
	items, err := s.listAndNormalize(ctx)
	if err != nil {
		return nil, err
	}
	ownerUserID := ownerUserIDFromContext(ctx)
	usageAgents, err := s.repository.ListUsageAgentsByOwner(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	result := make([]Record, 0, len(items))
	for _, item := range items {
		usageCount := 0
		if item.ProviderKind == ProviderKindLLM {
			usageCount = len(usageAgents[item.Provider])
		}
		models, err := s.modelsForRecord(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, toRecord(ctx, item, usageCount, usageAgents[item.Provider], models))
	}
	return result, nil
}

// ListOptions 返回启用状态的 Provider 下拉选项。
func (s *Service) ListOptions(ctx context.Context) (*OptionsResponse, error) {
	items, err := s.listAndNormalize(ctx)
	if err != nil {
		return nil, err
	}
	result := &OptionsResponse{
		Items:           make([]Option, 0, len(items)),
		BackgroundItems: make([]Option, 0, len(items)),
		ImageItems:      make([]Option, 0, len(items)),
	}
	for _, item := range items {
		if !item.Enabled {
			continue
		}
		models, err := s.enabledModelOptions(ctx, item)
		if err != nil {
			return nil, err
		}
		option := Option{
			Provider:    item.Provider,
			DisplayName: item.DisplayName,
			Models:      models,
		}
		switch {
		case item.ProviderKind == ProviderKindLLM:
			result.BackgroundItems = append(result.BackgroundItems, option)
			if isAgentRuntimeProvider(item) {
				result.Items = append(result.Items, option)
			}
		case item.ProviderKind == ProviderKindImageGeneration:
			result.ImageItems = append(result.ImageItems, option)
		}
	}
	if target, err := s.defaultRuntimeSelection(ctx); err != nil {
		return nil, err
	} else if target != nil {
		selection := modelSelectionFromTarget(*target)
		result.DefaultProvider = &selection.Provider
		result.DefaultModel = &selection.Model
		result.DefaultSelection = &selection
	}
	if target, err := s.defaultImageSelection(ctx); err != nil {
		return nil, err
	} else if target != nil {
		selection := modelSelectionFromTarget(*target)
		result.DefaultImageProvider = &selection.Provider
		result.DefaultImageModel = &selection.Model
		result.DefaultImageSelection = &selection
	}
	return result, nil
}

// DefaultProvider 返回当前默认运行模型所属的 Provider，保留给前端启动数据使用。
func (s *Service) DefaultProvider(ctx context.Context) (*string, error) {
	target, err := s.defaultRuntimeSelection(ctx)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, nil
	}
	value := target.provider.Provider
	return &value, nil
}

// AvailabilityState 描述当前 Provider 配置的就绪程度，便于启动期或健康检查上报。
type AvailabilityState struct {
	Total       int
	EnabledList []string
	HasDefault  bool
}

// Availability 汇总 Provider 现状：是否有可用条目、是否已选默认。
func (s *Service) Availability(ctx context.Context) (AvailabilityState, error) {
	items, err := s.listAndNormalize(ctx)
	if err != nil {
		return AvailabilityState{}, err
	}
	state := AvailabilityState{}
	for _, item := range items {
		if item.ProviderKind != ProviderKindLLM {
			continue
		}
		state.Total++
		if !item.Enabled || !isAgentRuntimeProvider(item) {
			continue
		}
		state.EnabledList = append(state.EnabledList, item.Provider)
		models, modelErr := s.repository.ListModelsByProviderID(ctx, item.ID)
		if modelErr != nil {
			return AvailabilityState{}, modelErr
		}
		for _, model := range models {
			if model.Enabled && model.IsDefault {
				state.HasDefault = true
			}
		}
	}
	return state, nil
}

// Create 新增 Provider 配置。
func (s *Service) Create(ctx context.Context, input CreateInput) (*Record, error) {
	normalized, err := normalizeCreateInput(input)
	if err != nil {
		return nil, err
	}
	visibility, ownerUserID, err := s.createVisibility(ctx, normalized.Visibility)
	if err != nil {
		return nil, err
	}
	existing, err := s.repository.GetScopedByProvider(ctx, visibility, ownerUserID, normalized.Provider)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("provider 已存在: %s", normalized.Provider)
	}
	now := s.now()
	item := providerstore.Entity{
		ID:             s.idFactory("provider"),
		OwnerUserID:    ownerUserID,
		Visibility:     visibility,
		ProviderKind:   normalized.ProviderKind,
		Provider:       normalized.Provider,
		PresetKey:      normalized.PresetKey,
		APIFormat:      normalized.APIFormat,
		DisplayName:    normalized.DisplayName,
		AuthToken:      normalized.AuthToken,
		BaseURL:        normalized.BaseURL,
		ModelsPath:     normalized.ModelsPath,
		Enabled:        normalized.Enabled,
		LastTestStatus: "",
		LastTestError:  "",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err = s.repository.Create(ctx, item); err != nil {
		return nil, err
	}
	return s.Get(ctx, item.Provider)
}

// Update 更新 Provider 配置。
func (s *Service) Update(ctx context.Context, provider string, input UpdateInput) (*Record, error) {
	normalizedProvider, err := NormalizeProvider(provider, false)
	if err != nil {
		return nil, err
	}
	current, err := s.repository.GetVisibleByProvider(ctx, ownerUserIDFromContext(ctx), normalizedProvider)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("provider 不存在: %s", normalizedProvider)
	}
	if err = s.requireProviderManagement(ctx, *current); err != nil {
		return nil, err
	}
	usageCount, err := s.usageCountForMutation(ctx, *current)
	if err != nil {
		return nil, err
	}
	if current.ProviderKind == ProviderKindLLM && usageCount > 0 && !input.Enabled {
		return nil, fmt.Errorf("provider=%s 仍被 %d 个 Agent 使用，不能禁用", normalizedProvider, usageCount)
	}
	updated, err := normalizeUpdateInput(*current, input)
	if err != nil {
		return nil, err
	}
	updated.UpdatedAt = s.now()
	if err = s.repository.Update(ctx, updated); err != nil {
		return nil, err
	}
	return s.Get(ctx, normalizedProvider)
}

// Delete 删除 Provider 配置；强制删除会先把显式绑定切到平台默认 Provider。
func (s *Service) Delete(ctx context.Context, provider string, input DeleteInput) (*DeleteResult, error) {
	normalizedProvider, err := NormalizeProvider(provider, false)
	if err != nil {
		return nil, err
	}
	current, err := s.repository.GetVisibleByProvider(ctx, ownerUserIDFromContext(ctx), normalizedProvider)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("provider 不存在: %s", normalizedProvider)
	}
	if err = s.requireProviderManagement(ctx, *current); err != nil {
		return nil, err
	}
	usageCount, err := s.usageCountForMutation(ctx, *current)
	if err != nil {
		return nil, err
	}
	result := &DeleteResult{Provider: normalizedProvider}
	if current.ProviderKind == ProviderKindLLM && usageCount > 0 {
		if !input.Force {
			return nil, fmt.Errorf("provider=%s 仍被 %d 个 Agent 使用，不能删除", normalizedProvider, usageCount)
		}
		replacement, replacementErr := s.replacementRuntimeSelectionForDelete(ctx, *current)
		if replacementErr != nil {
			return nil, replacementErr
		}
		reassigned, replaceErr := s.replaceRuntimeProviderForDelete(ctx, *current, replacement.provider.Provider, replacement.model.ModelID)
		if replaceErr != nil {
			return nil, replaceErr
		}
		result.ReplacementProvider = replacement.provider.Provider
		result.ReplacementModel = replacement.model.ModelID
		result.ReassignedRuntimeCount = reassigned
	}
	if err = s.repository.Delete(ctx, current.ID); err != nil {
		return nil, err
	}
	return result, nil
}

// Get 读取单个 Provider 配置。
func (s *Service) Get(ctx context.Context, provider string) (*Record, error) {
	normalizedProvider, err := NormalizeProvider(provider, false)
	if err != nil {
		return nil, err
	}
	if _, err = s.listAndNormalize(ctx); err != nil {
		return nil, err
	}
	ownerUserID := ownerUserIDFromContext(ctx)
	item, err := s.repository.GetVisibleByProvider(ctx, ownerUserID, normalizedProvider)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("provider 不存在: %s", normalizedProvider)
	}
	normalizeBuiltinEndpoint(item)
	usageCount := 0
	usageAgents := []providerstore.UsageAgentEntity(nil)
	if item.ProviderKind == ProviderKindLLM {
		var countErr error
		usageCount, countErr = s.repository.UsageCountForOwner(ctx, ownerUserID, item.Provider)
		if countErr != nil {
			return nil, countErr
		}
		var usageErr error
		usageAgents, usageErr = s.repository.ListUsageAgentsByOwnerProvider(ctx, ownerUserID, item.Provider)
		if usageErr != nil {
			return nil, usageErr
		}
	}
	models, err := s.modelsForRecord(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	record := toRecord(ctx, *item, usageCount, usageAgents, models)
	return &record, nil
}

// ResolveRuntimeConfig 解析 Agent 最终运行时要使用的 Provider 配置。
func (s *Service) ResolveRuntimeConfig(ctx context.Context, provider string, model string) (*clientopts.RuntimeConfig, error) {
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
	if targetProvider != "" {
		for index := range items {
			if items[index].Provider == targetProvider && items[index].ProviderKind == ProviderKindLLM {
				target = &items[index]
				break
			}
		}
		if target == nil {
			return nil, fmt.Errorf("provider 不存在: %s", targetProvider)
		}
		if targetModel == "" {
			targetModel, err = s.resolveMissingExplicitModel(ctx, target.ID)
			if err != nil {
				return nil, err
			}
		}
	} else {
		if targetModel != "" {
			return nil, errors.New("指定 model 时必须同时指定 provider")
		}
		defaultTarget, defaultErr := s.defaultRuntimeSelection(ctx)
		if defaultErr != nil {
			return nil, defaultErr
		}
		if defaultTarget != nil {
			target = &defaultTarget.provider
			targetModel = defaultTarget.model.ModelID
		}
	}
	return s.runtimeConfigFromTarget(ctx, target, targetModel)
}

// ResolveLLMConfig 解析后端轻量 LLM 任务要使用的 Provider 配置，不受 Agent runtime 协议限制。
func (s *Service) ResolveLLMConfig(ctx context.Context, provider string, model string) (*clientopts.RuntimeConfig, error) {
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
	if targetProvider != "" {
		for index := range items {
			if items[index].Provider == targetProvider && items[index].ProviderKind == ProviderKindLLM {
				target = &items[index]
				break
			}
		}
		if target == nil {
			return nil, fmt.Errorf("provider 不存在: %s", targetProvider)
		}
		if targetModel == "" {
			targetModel, err = s.resolveMissingExplicitModel(ctx, target.ID)
			if err != nil {
				return nil, err
			}
		}
	} else {
		if targetModel != "" {
			return nil, errors.New("指定 model 时必须同时指定 provider")
		}
		defaultTarget, defaultErr := s.defaultRuntimeSelection(ctx)
		if defaultErr != nil {
			return nil, defaultErr
		}
		if defaultTarget != nil {
			target = &defaultTarget.provider
			targetModel = defaultTarget.model.ModelID
		}
	}
	return s.llmConfigFromTarget(ctx, target, targetModel)
}

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
		modelRecord, err = s.defaultOrFirstEnabledModel(ctx, target.ID)
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

// NormalizeProvider 规整 provider key。
func NormalizeProvider(provider string, allowEmpty bool) (string, error) {
	cleaned := strings.ToLower(strings.TrimSpace(provider))
	if cleaned == "" {
		if allowEmpty {
			return "", nil
		}
		return "", errors.New("provider 不能为空")
	}
	normalized := strings.Trim(providerPattern.ReplaceAllString(cleaned, "-"), "-")
	if normalized == "" {
		return "", fmt.Errorf("非法的 provider: %s", provider)
	}
	return normalized, nil
}

func normalizeProviderKind(providerKind string) string {
	switch strings.TrimSpace(providerKind) {
	case "", ProviderKindLLM:
		return ProviderKindLLM
	case ProviderKindImageGeneration:
		return ProviderKindImageGeneration
	default:
		return ProviderKindLLM
	}
}

func ownerUserIDFromContext(ctx context.Context) string {
	return authctx.OwnerUserID(ctx)
}

func canManagePublicProviders(ctx context.Context) bool {
	principal := authctx.PrincipalFromContext(ctx)
	if principal == nil {
		return true
	}
	switch strings.TrimSpace(principal.Role) {
	case authctx.RoleOwner, authctx.RoleAdmin:
		return true
	default:
		return false
	}
}

func (s *Service) listAndNormalize(ctx context.Context) ([]providerstore.Entity, error) {
	items, err := s.repository.ListVisible(ctx, ownerUserIDFromContext(ctx))
	if err != nil {
		return nil, err
	}
	items = collapseVisibleProviders(items)
	for index := range items {
		normalizeBuiltinEndpoint(&items[index])
	}
	return items, nil
}

func (s *Service) listPublicAndNormalize(ctx context.Context) ([]providerstore.Entity, error) {
	items, err := s.repository.ListPublic(ctx)
	if err != nil {
		return nil, err
	}
	for index := range items {
		normalizeBuiltinEndpoint(&items[index])
	}
	return items, nil
}

func collapseVisibleProviders(items []providerstore.Entity) []providerstore.Entity {
	result := make([]providerstore.Entity, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		provider := strings.TrimSpace(item.Provider)
		if provider == "" || seen[provider] {
			continue
		}
		seen[provider] = true
		result = append(result, item)
	}
	return result
}

func normalizeBuiltinEndpoint(item *providerstore.Entity) {
	if item == nil || strings.TrimSpace(item.PresetKey) == "" || item.PresetKey == presetCustom {
		return
	}
	preset := resolvePreset(item.PresetKey)
	if preset.PresetKey == presetCustom {
		return
	}
	apiFormat := normalizeAPIFormat(item.APIFormat)
	if apiFormat == "" {
		apiFormat = preset.DefaultFormat
	}
	format := preset.Format(apiFormat)
	item.APIFormat = apiFormat
	item.BaseURL = format.BaseURL
	item.ModelsPath = format.ModelsPath
}

func (s *Service) defaultRuntimeSelection(ctx context.Context) (*providerModelTarget, error) {
	items, err := s.listAndNormalize(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if !item.Enabled || !isAgentRuntimeProvider(item) {
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
	return nil, nil
}

func (s *Service) defaultImageSelection(ctx context.Context) (*providerModelTarget, error) {
	items, err := s.listAndNormalize(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if !item.Enabled || item.ProviderKind != ProviderKindImageGeneration {
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
	return nil, nil
}

func (s *Service) runtimeConfigFromTarget(
	ctx context.Context,
	target *providerstore.Entity,
	targetModel string,
) (*clientopts.RuntimeConfig, error) {
	if target == nil {
		return nil, errors.New("未配置默认模型，请先到 Settings 选择默认模型")
	}
	if !target.Enabled {
		return nil, fmt.Errorf("provider=%s 已禁用", target.Provider)
	}
	if target.ProviderKind != ProviderKindLLM {
		return nil, fmt.Errorf("provider=%s 不是 LLM Provider", target.Provider)
	}
	if !isAgentRuntimeProvider(*target) {
		return nil, fmt.Errorf("provider=%s 的 api_format=%s 暂不可用于 Agent runtime", target.Provider, target.APIFormat)
	}
	return s.llmConfigFromTarget(ctx, target, targetModel)
}

func (s *Service) llmConfigFromTarget(
	ctx context.Context,
	target *providerstore.Entity,
	targetModel string,
) (*clientopts.RuntimeConfig, error) {
	if target == nil {
		return nil, errors.New("未配置默认模型，请先到 Settings 选择默认模型")
	}
	if !target.Enabled {
		return nil, fmt.Errorf("provider=%s 已禁用", target.Provider)
	}
	if target.ProviderKind != ProviderKindLLM {
		return nil, fmt.Errorf("provider=%s 不是 LLM Provider", target.Provider)
	}
	if strings.TrimSpace(targetModel) == "" {
		return nil, fmt.Errorf("provider=%s 缺少 model，请先选择该 Provider 下的模型", target.Provider)
	}
	modelRecord, err := s.getModelByID(ctx, target.ID, targetModel)
	if err != nil {
		return nil, err
	}
	if modelRecord == nil {
		return nil, fmt.Errorf("provider=%s 模型不存在: %s", target.Provider, targetModel)
	}
	if !modelRecord.Enabled {
		return nil, fmt.Errorf("provider=%s model=%s 已禁用", target.Provider, targetModel)
	}

	missing := make([]string, 0, 3)
	if target.AuthToken == "" {
		missing = append(missing, "auth_token")
	}
	if target.BaseURL == "" {
		missing = append(missing, "base_url")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("provider=%s 配置不完整: %s", target.Provider, strings.Join(missing, ", "))
	}
	return &clientopts.RuntimeConfig{
		Provider:    target.Provider,
		DisplayName: target.DisplayName,
		AuthToken:   target.AuthToken,
		BaseURL:     target.BaseURL,
		Model:       normalizeModelID(modelRecord.ModelID),
		APIFormat:   target.APIFormat,
	}, nil
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
	models, err := s.repository.ListModelsByProviderID(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	result := make([]ModelOption, 0, len(models))
	for _, model := range models {
		if !model.Enabled || strings.TrimSpace(model.ModelID) == "" {
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

func normalizeCreateInput(input CreateInput) (CreateInput, error) {
	provider, err := NormalizeProvider(input.Provider, false)
	if err != nil {
		return CreateInput{}, err
	}
	preset := resolvePreset(input.PresetKey)
	apiFormat := normalizeAPIFormat(input.APIFormat)
	if apiFormat == "" {
		if strings.TrimSpace(input.PresetKey) == "" {
			apiFormat = APIFormatAnthropicMessages
		} else {
			apiFormat = preset.DefaultFormat
		}
	}
	format := preset.Format(apiFormat)
	providerKind := normalizeProviderKind(input.ProviderKind)
	if strings.TrimSpace(input.ProviderKind) == "" {
		switch {
		case strings.TrimSpace(format.ProviderKind) != "":
			providerKind = normalizeProviderKind(format.ProviderKind)
		case strings.TrimSpace(preset.ProviderKind) != "":
			providerKind = normalizeProviderKind(preset.ProviderKind)
		}
	}
	if err := validatePresetFormatKind(preset, format, providerKind); err != nil {
		return CreateInput{}, err
	}
	baseURL := strings.TrimSpace(input.BaseURL)
	if preset.PresetKey != presetCustom {
		baseURL = format.BaseURL
	} else if baseURL == "" {
		baseURL = format.BaseURL
	}
	modelsPath := strings.TrimSpace(input.ModelsPath)
	if preset.PresetKey != presetCustom {
		modelsPath = format.ModelsPath
	} else if modelsPath == "" {
		modelsPath = format.ModelsPath
	}
	result := CreateInput{
		ProviderKind: providerKind,
		Provider:     provider,
		Visibility:   strings.TrimSpace(input.Visibility),
		PresetKey:    preset.PresetKey,
		APIFormat:    apiFormat,
		DisplayName:  strings.TrimSpace(input.DisplayName),
		AuthToken:    strings.TrimSpace(input.AuthToken),
		BaseURL:      baseURL,
		ModelsPath:   modelsPath,
		Enabled:      input.Enabled,
	}
	if result.DisplayName == "" {
		result.DisplayName = preset.DisplayName
	}
	if result.DisplayName == "" {
		result.DisplayName = result.Provider
	}
	if result.AuthToken == "" {
		return CreateInput{}, errors.New("auth_token 不能为空")
	}
	if result.BaseURL == "" {
		return CreateInput{}, errors.New("base_url 不能为空")
	}
	return result, nil
}

func normalizeUpdateInput(current providerstore.Entity, input UpdateInput) (providerstore.Entity, error) {
	preset := resolvePreset(firstNonEmpty(input.PresetKey, current.PresetKey))
	apiFormat := normalizeAPIFormat(firstNonEmpty(input.APIFormat, current.APIFormat))
	if apiFormat == "" {
		apiFormat = preset.DefaultFormat
	}
	format := preset.Format(apiFormat)
	providerKind := normalizeProviderKind(firstNonEmpty(input.ProviderKind, current.ProviderKind))
	if strings.TrimSpace(input.ProviderKind) == "" && current.PresetKey != preset.PresetKey {
		switch {
		case strings.TrimSpace(format.ProviderKind) != "":
			providerKind = normalizeProviderKind(format.ProviderKind)
		case strings.TrimSpace(preset.ProviderKind) != "":
			providerKind = normalizeProviderKind(preset.ProviderKind)
		}
	}
	if err := validatePresetFormatKind(preset, format, providerKind); err != nil {
		return providerstore.Entity{}, err
	}
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = preset.DisplayName
	}
	if displayName == "" {
		displayName = current.Provider
	}
	baseURL := strings.TrimSpace(input.BaseURL)
	if preset.PresetKey != presetCustom {
		baseURL = format.BaseURL
	} else if baseURL == "" {
		baseURL = format.BaseURL
	}
	if baseURL == "" {
		return providerstore.Entity{}, errors.New("base_url 不能为空")
	}
	modelsPath := strings.TrimSpace(input.ModelsPath)
	if preset.PresetKey != presetCustom {
		modelsPath = format.ModelsPath
	} else if modelsPath == "" {
		modelsPath = format.ModelsPath
	}
	authToken := current.AuthToken
	if input.AuthToken != nil {
		authToken = strings.TrimSpace(*input.AuthToken)
	}
	if authToken == "" {
		return providerstore.Entity{}, errors.New("auth_token 不能为空")
	}
	current.DisplayName = displayName
	current.AuthToken = authToken
	current.BaseURL = baseURL
	current.ModelsPath = modelsPath
	current.Enabled = input.Enabled
	current.PresetKey = preset.PresetKey
	current.APIFormat = apiFormat
	current.ProviderKind = providerKind
	return current, nil
}

func validatePresetFormatKind(preset Preset, format PresetFormat, providerKind string) error {
	if preset.PresetKey == presetCustom || strings.TrimSpace(format.ProviderKind) == "" {
		return nil
	}
	expected := normalizeProviderKind(format.ProviderKind)
	if normalizeProviderKind(providerKind) != expected {
		return fmt.Errorf("api_format=%s 不支持 provider_kind=%s", format.APIFormat, providerKind)
	}
	return nil
}

func (s *Service) selectImageProvider(
	ctx context.Context,
	items []providerstore.Entity,
	targetProvider string,
) (*providerstore.Entity, error) {
	if targetProvider != "" {
		for index := range items {
			if items[index].Provider == targetProvider && items[index].ProviderKind == ProviderKindImageGeneration {
				return &items[index], nil
			}
		}
		return nil, fmt.Errorf("provider 不存在: %s", targetProvider)
	}
	for index := range items {
		if !items[index].Enabled || items[index].ProviderKind != ProviderKindImageGeneration {
			continue
		}
		model, err := s.defaultOrFirstEnabledModel(ctx, items[index].ID)
		if err != nil {
			return nil, err
		}
		if model != nil && model.IsDefault {
			return &items[index], nil
		}
	}
	for index := range items {
		if items[index].Enabled && items[index].ProviderKind == ProviderKindImageGeneration {
			return &items[index], nil
		}
	}
	return nil, nil
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

func (s *Service) createVisibility(ctx context.Context, requested string) (string, string, error) {
	visibility, err := normalizeProviderVisibility(requested, canManagePublicProviders(ctx))
	if err != nil {
		return "", "", err
	}
	if visibility == providerstore.VisibilityPublic {
		return visibility, "", nil
	}
	return visibility, ownerUserIDFromContext(ctx), nil
}

func normalizeProviderVisibility(requested string, canManagePublic bool) (string, error) {
	switch strings.TrimSpace(requested) {
	case "":
		if canManagePublic {
			return providerstore.VisibilityPublic, nil
		}
		return providerstore.VisibilityPrivate, nil
	case providerstore.VisibilityPublic:
		if !canManagePublic {
			return "", errors.New("只有管理员可以创建公共 Provider")
		}
		return providerstore.VisibilityPublic, nil
	case providerstore.VisibilityPrivate:
		return providerstore.VisibilityPrivate, nil
	default:
		return "", errors.New("provider visibility 只支持 public 或 private")
	}
}

func (s *Service) requireProviderManagement(ctx context.Context, item providerstore.Entity) error {
	if item.Visibility != providerstore.VisibilityPublic {
		return nil
	}
	if canManagePublicProviders(ctx) {
		return nil
	}
	return errors.New("只有管理员可以维护公共 Provider")
}

func (s *Service) usageCountForMutation(ctx context.Context, item providerstore.Entity) (int, error) {
	if item.Visibility == providerstore.VisibilityPublic {
		return s.repository.UsageCountForPublic(ctx, item.Provider)
	}
	return s.repository.UsageCountForOwner(ctx, item.OwnerUserID, item.Provider)
}

func (s *Service) replaceRuntimeProviderForDelete(
	ctx context.Context,
	deleting providerstore.Entity,
	newProvider string,
	newModel string,
) (int, error) {
	if deleting.Visibility == providerstore.VisibilityPublic {
		return s.repository.ReplaceRuntimeProviderForPublic(ctx, deleting.Provider, newProvider, newModel)
	}
	return s.repository.ReplaceRuntimeProviderForOwner(ctx, deleting.OwnerUserID, deleting.Provider, newProvider, newModel)
}

func toRecord(
	ctx context.Context,
	item providerstore.Entity,
	usageCount int,
	usageAgents []providerstore.UsageAgentEntity,
	models []ModelRecord,
) Record {
	createdAt := item.CreatedAt
	updatedAt := item.UpdatedAt
	canManage := item.Visibility != providerstore.VisibilityPublic || canManagePublicProviders(ctx)
	authTokenMasked := maskToken(item.AuthToken)
	if !canManage {
		authTokenMasked = ""
	}
	return Record{
		ID:                    item.ID,
		OwnerUserID:           item.OwnerUserID,
		Visibility:            item.Visibility,
		ProviderKind:          item.ProviderKind,
		Provider:              item.Provider,
		PresetKey:             item.PresetKey,
		APIFormat:             item.APIFormat,
		DisplayName:           item.DisplayName,
		AuthTokenMasked:       authTokenMasked,
		BaseURL:               item.BaseURL,
		ModelsPath:            item.ModelsPath,
		Enabled:               item.Enabled,
		UsageCount:            usageCount,
		UsedByAgents:          toUsageAgents(usageAgents),
		LastTestStatus:        item.LastTestStatus,
		LastTestError:         item.LastTestError,
		LastTestAt:            item.LastTestAt,
		CanManage:             canManage,
		AgentRuntimeSupported: isAgentRuntimeProvider(item),
		Models:                models,
		CreatedAt:             &createdAt,
		UpdatedAt:             &updatedAt,
	}
}

func toUsageAgents(items []providerstore.UsageAgentEntity) []UsageAgent {
	result := make([]UsageAgent, 0, len(items))
	for _, item := range items {
		displayName := strings.TrimSpace(item.DisplayName)
		if displayName == "" {
			displayName = strings.TrimSpace(item.Name)
		}
		result = append(result, UsageAgent{
			AgentID:     strings.TrimSpace(item.AgentID),
			Name:        strings.TrimSpace(item.Name),
			DisplayName: displayName,
			Avatar:      strings.TrimSpace(item.Avatar),
			IsMain:      item.IsMain,
		})
	}
	return result
}

func maskToken(token string) string {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 10 {
		return strings.Repeat("*", len(trimmed))
	}
	return trimmed[:5] + strings.Repeat("*", 24) + trimmed[len(trimmed)-5:]
}

func newProviderID(prefix string) string {
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UTC().UnixNano(), providerIDCounter.Add(1))
}
