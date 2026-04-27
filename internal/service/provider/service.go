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
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

var providerPattern = regexp.MustCompile(`[^a-z0-9]+`)

// Service 提供 Provider 配置管理与运行时解析。
type Service struct {
	repository *providerstore.Repository
	now        func() time.Time
	idFactory  func(string) string
}

// NewServiceWithDB 使用共享 DB 创建 Provider 配置服务。
func NewServiceWithDB(cfg config.Config, db *sql.DB) *Service {
	return &Service{
		repository: providerstore.NewRepository(cfg, db),
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
	usageCounts, err := s.repository.ListUsageCounts(ctx)
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
	existing, err := s.repository.GetByProvider(ctx, normalized.Provider)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("provider 已存在: %s", normalized.Provider)
	}
	now := s.now()
	item := providerstore.Entity{
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
	if err = s.repository.Create(ctx, item); err != nil {
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
	current, err := s.repository.GetByProvider(ctx, normalizedProvider)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("provider 不存在: %s", normalizedProvider)
	}
	usageCount, err := s.repository.UsageCount(ctx, normalizedProvider)
	if err != nil {
		return nil, err
	}
	if usageCount > 0 && !input.Enabled {
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
	current, err := s.repository.GetByProvider(ctx, normalizedProvider)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("provider 不存在: %s", normalizedProvider)
	}
	usageCount, err := s.repository.UsageCount(ctx, normalizedProvider)
	if err != nil {
		return err
	}
	if usageCount > 0 {
		return fmt.Errorf("provider=%s 仍被 %d 个 Agent 使用，不能删除", normalizedProvider, usageCount)
	}
	if err = s.repository.Delete(ctx, normalizedProvider); err != nil {
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
	if _, err = s.listAndNormalize(ctx); err != nil {
		return nil, err
	}
	item, err := s.repository.GetByProvider(ctx, normalizedProvider)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("provider 不存在: %s", normalizedProvider)
	}
	usageCount, err := s.repository.UsageCount(ctx, item.Provider)
	if err != nil {
		return nil, err
	}
	record := toRecord(*item, usageCount)
	return &record, nil
}

// ResolveRuntimeConfig 解析 Agent 最终运行时要使用的 Provider 配置。
func (s *Service) ResolveRuntimeConfig(ctx context.Context, provider string) (*clientopts.RuntimeConfig, error) {
	items, err := s.listAndNormalize(ctx)
	if err != nil {
		return nil, err
	}
	targetProvider, err := NormalizeProvider(provider, true)
	if err != nil {
		return nil, err
	}

	var target *providerstore.Entity
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
	return &clientopts.RuntimeConfig{
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

func (s *Service) listAndNormalize(ctx context.Context) ([]providerstore.Entity, error) {
	items, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	changed, err := s.ensureDefaultFromItems(ctx, items, "")
	if err != nil {
		return nil, err
	}
	if changed {
		return s.repository.List(ctx)
	}
	return items, nil
}

func (s *Service) ensureDefault(ctx context.Context, preferred string) error {
	items, err := s.repository.List(ctx)
	if err != nil {
		return err
	}
	_, err = s.ensureDefaultFromItems(ctx, items, preferred)
	return err
}

func (s *Service) ensureDefaultFromItems(ctx context.Context, items []providerstore.Entity, preferred string) (bool, error) {
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
	currentDefaultCount := 0
	currentTargetIsDefault := false
	for _, item := range items {
		if !item.IsDefault {
			continue
		}
		currentDefaultCount++
		if item.Enabled && item.Provider == target {
			currentTargetIsDefault = true
		}
	}
	if target == "" {
		if currentDefaultCount == 0 {
			return false, nil
		}
		return true, s.repository.UpdateDefaultFlags(ctx, "")
	}
	if currentTargetIsDefault && currentDefaultCount == 1 {
		return false, nil
	}
	return true, s.repository.UpdateDefaultFlags(ctx, target)
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

func normalizeUpdateInput(current providerstore.Entity, input UpdateInput) (providerstore.Entity, error) {
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		return providerstore.Entity{}, errors.New("display_name 不能为空")
	}
	baseURL := strings.TrimSpace(input.BaseURL)
	if baseURL == "" {
		return providerstore.Entity{}, errors.New("base_url 不能为空")
	}
	model := strings.TrimSpace(input.Model)
	if model == "" {
		return providerstore.Entity{}, errors.New("model 不能为空")
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
	current.Model = model
	current.Enabled = input.Enabled
	current.IsDefault = input.IsDefault
	return current, nil
}

func preferredDefault(item providerstore.Entity) string {
	if item.Enabled && item.IsDefault {
		return item.Provider
	}
	return ""
}

func toRecord(item providerstore.Entity, usageCount int) Record {
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
