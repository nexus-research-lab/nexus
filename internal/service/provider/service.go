package provider

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
)

var providerPattern = regexp.MustCompile(`[^a-z0-9]+`)

// Service 提供 Provider 配置管理与运行时解析。
type Service struct {
	repository *repository
	now        func() time.Time
	idFactory  func(string) string
}

// NewServiceWithDB 使用共享 DB 创建 Provider 配置服务。
func NewServiceWithDB(cfg config.Config, db *sql.DB) *Service {
	return &Service{
		repository: newRepository(cfg, db),
		now:        func() time.Time { return time.Now().UTC() },
		idFactory:  newProviderID,
	}
}

// List 返回完整 Provider 配置列表。
func (s *Service) List(ctx context.Context) ([]Record, error) {
	items, err := s.listAndNormalize(ctx)
	if err != nil {
		return nil, err
	}
	usageCounts, err := s.repository.listUsageCounts(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Record, 0, len(items))
	for _, item := range items {
		result = append(result, toRecord(item, usageCounts[item.Provider]))
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
		Items: make([]Option, 0, len(items)),
	}
	for _, item := range items {
		if !item.Enabled {
			continue
		}
		if item.IsDefault {
			value := item.Provider
			result.DefaultProvider = &value
		}
		result.Items = append(result.Items, Option{
			Provider:    item.Provider,
			DisplayName: item.DisplayName,
			IsDefault:   item.IsDefault,
		})
	}
	return result, nil
}

// DefaultProvider 返回当前启用的默认 Provider。
func (s *Service) DefaultProvider(ctx context.Context) (*string, error) {
	items, err := s.listAndNormalize(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.Enabled && item.IsDefault {
			value := item.Provider
			return &value, nil
		}
	}
	return nil, nil
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
	state := AvailabilityState{Total: len(items)}
	for _, item := range items {
		if !item.Enabled {
			continue
		}
		state.EnabledList = append(state.EnabledList, item.Provider)
		if item.IsDefault {
			state.HasDefault = true
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
	existing, err := s.repository.getByProvider(ctx, normalized.Provider)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("provider 已存在: %s", normalized.Provider)
	}
	now := s.now()
	item := entity{
		ID:          s.idFactory("provider"),
		Provider:    normalized.Provider,
		DisplayName: normalized.DisplayName,
		AuthToken:   normalized.AuthToken,
		BaseURL:     normalized.BaseURL,
		Model:       normalized.Model,
		Enabled:     normalized.Enabled,
		IsDefault:   normalized.IsDefault,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err = s.repository.create(ctx, item); err != nil {
		return nil, err
	}
	if err = s.ensureDefault(ctx, preferredDefault(item)); err != nil {
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
	current, err := s.repository.getByProvider(ctx, normalizedProvider)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("provider 不存在: %s", normalizedProvider)
	}
	usageCounts, err := s.repository.listUsageCounts(ctx)
	if err != nil {
		return nil, err
	}
	if usageCounts[normalizedProvider] > 0 && !input.Enabled {
		return nil, fmt.Errorf("provider=%s 仍被 %d 个 Agent 使用，不能禁用", normalizedProvider, usageCounts[normalizedProvider])
	}
	updated, err := normalizeUpdateInput(*current, input)
	if err != nil {
		return nil, err
	}
	updated.UpdatedAt = s.now()
	if err = s.repository.update(ctx, updated); err != nil {
		return nil, err
	}
	if err = s.ensureDefault(ctx, preferredDefault(updated)); err != nil {
		return nil, err
	}
	return s.Get(ctx, normalizedProvider)
}

// Delete 删除 Provider 配置。
func (s *Service) Delete(ctx context.Context, provider string) error {
	normalizedProvider, err := NormalizeProvider(provider, false)
	if err != nil {
		return err
	}
	current, err := s.repository.getByProvider(ctx, normalizedProvider)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("provider 不存在: %s", normalizedProvider)
	}
	usageCounts, err := s.repository.listUsageCounts(ctx)
	if err != nil {
		return err
	}
	if usageCounts[normalizedProvider] > 0 {
		return fmt.Errorf("provider=%s 仍被 %d 个 Agent 使用，不能删除", normalizedProvider, usageCounts[normalizedProvider])
	}
	if err = s.repository.delete(ctx, normalizedProvider); err != nil {
		return err
	}
	return s.ensureDefault(ctx, "")
}

// Get 读取单个 Provider 配置。
func (s *Service) Get(ctx context.Context, provider string) (*Record, error) {
	normalizedProvider, err := NormalizeProvider(provider, false)
	if err != nil {
		return nil, err
	}
	items, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.Provider == normalizedProvider {
			value := item
			return &value, nil
		}
	}
	return nil, fmt.Errorf("provider 不存在: %s", normalizedProvider)
}

// ResolveRuntimeConfig 解析 Agent 最终运行时要使用的 Provider 配置。
func (s *Service) ResolveRuntimeConfig(ctx context.Context, provider string) (*RuntimeConfig, error) {
	items, err := s.listAndNormalize(ctx)
	if err != nil {
		return nil, err
	}
	targetProvider, err := NormalizeProvider(provider, true)
	if err != nil {
		return nil, err
	}

	var target *entity
	if targetProvider != "" {
		for index := range items {
			if items[index].Provider == targetProvider {
				target = &items[index]
				break
			}
		}
		if target == nil {
			return nil, fmt.Errorf("provider 不存在: %s", targetProvider)
		}
	} else {
		for index := range items {
			if items[index].Enabled && items[index].IsDefault {
				target = &items[index]
				break
			}
		}
	}
	if target == nil {
		return nil, errors.New("未配置可用的 Provider，请先到 Settings 添加 Provider")
	}
	if !target.Enabled {
		return nil, fmt.Errorf("provider=%s 已禁用", target.Provider)
	}

	missing := make([]string, 0, 3)
	if target.AuthToken == "" {
		missing = append(missing, "auth_token")
	}
	if target.BaseURL == "" {
		missing = append(missing, "base_url")
	}
	if target.Model == "" {
		missing = append(missing, "model")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("provider=%s 配置不完整: %s", target.Provider, strings.Join(missing, ", "))
	}
	return &RuntimeConfig{
		Provider:    target.Provider,
		DisplayName: target.DisplayName,
		AuthToken:   target.AuthToken,
		BaseURL:     target.BaseURL,
		Model:       target.Model,
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

func (s *Service) listAndNormalize(ctx context.Context) ([]entity, error) {
	if err := s.ensureDefault(ctx, ""); err != nil {
		return nil, err
	}
	return s.repository.list(ctx)
}

func (s *Service) ensureDefault(ctx context.Context, preferred string) error {
	items, err := s.repository.list(ctx)
	if err != nil {
		return err
	}
	target := strings.TrimSpace(preferred)
	if target == "" {
		for _, item := range items {
			if item.Enabled && item.IsDefault {
				target = item.Provider
				break
			}
		}
	}
	if target == "" {
		for _, item := range items {
			if item.Enabled {
				target = item.Provider
				break
			}
		}
	}
	return s.repository.updateDefaultFlags(ctx, target)
}

func normalizeCreateInput(input CreateInput) (CreateInput, error) {
	provider, err := NormalizeProvider(input.Provider, false)
	if err != nil {
		return CreateInput{}, err
	}
	result := CreateInput{
		Provider:    provider,
		DisplayName: strings.TrimSpace(input.DisplayName),
		AuthToken:   strings.TrimSpace(input.AuthToken),
		BaseURL:     strings.TrimSpace(input.BaseURL),
		Model:       strings.TrimSpace(input.Model),
		Enabled:     input.Enabled,
		IsDefault:   input.IsDefault,
	}
	if result.DisplayName == "" {
		return CreateInput{}, errors.New("display_name 不能为空")
	}
	if result.AuthToken == "" {
		return CreateInput{}, errors.New("auth_token 不能为空")
	}
	if result.BaseURL == "" {
		return CreateInput{}, errors.New("base_url 不能为空")
	}
	if result.Model == "" {
		return CreateInput{}, errors.New("model 不能为空")
	}
	return result, nil
}

func normalizeUpdateInput(current entity, input UpdateInput) (entity, error) {
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		return entity{}, errors.New("display_name 不能为空")
	}
	baseURL := strings.TrimSpace(input.BaseURL)
	if baseURL == "" {
		return entity{}, errors.New("base_url 不能为空")
	}
	model := strings.TrimSpace(input.Model)
	if model == "" {
		return entity{}, errors.New("model 不能为空")
	}
	authToken := current.AuthToken
	if input.AuthToken != nil {
		authToken = strings.TrimSpace(*input.AuthToken)
	}
	if authToken == "" {
		return entity{}, errors.New("auth_token 不能为空")
	}
	current.DisplayName = displayName
	current.AuthToken = authToken
	current.BaseURL = baseURL
	current.Model = model
	current.Enabled = input.Enabled
	current.IsDefault = input.IsDefault
	return current, nil
}

func preferredDefault(item entity) string {
	if item.Enabled && item.IsDefault {
		return item.Provider
	}
	return ""
}

func toRecord(item entity, usageCount int) Record {
	createdAt := item.CreatedAt
	updatedAt := item.UpdatedAt
	return Record{
		ID:              item.ID,
		Provider:        item.Provider,
		DisplayName:     item.DisplayName,
		AuthTokenMasked: maskToken(item.AuthToken),
		BaseURL:         item.BaseURL,
		Model:           item.Model,
		Enabled:         item.Enabled,
		IsDefault:       item.IsDefault,
		UsageCount:      usageCount,
		CreatedAt:       &createdAt,
		UpdatedAt:       &updatedAt,
	}
}

func maskToken(token string) string {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 4 {
		return strings.Repeat("*", len(trimmed))
	}
	return strings.Repeat("*", len(trimmed)-4) + trimmed[len(trimmed)-4:]
}

func newProviderID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UTC().UnixNano())
}
