// INPUT: Provider 配置请求与可信 owner。
// OUTPUT: 私有 Provider、模型变更及写后存在性核对。
// POS: Provider 输入、读取、校验与写入的同域实现。
package configuration

import (
	"context"
	"errors"
	"fmt"
	"strings"

	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
)

func validateProvidersChange(request ChangeRequest) error {
	switch request.Operation {
	case "create":
		var input providerCreateRequest
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		switch strings.TrimSpace(input.Visibility) {
		case "", "private":
			return nil
		default:
			return errors.New("对话配置只能创建当前 owner 的私有 Provider；公共订阅 Provider 必须由人类运营界面管理")
		}
	case "update":
		if err := request.requireTarget(); err != nil {
			return err
		}
		var input providerUpdateRequest
		if err := request.decodeInput(&input); err != nil {
			return err
		}
		if input.ProviderKind == nil && input.PresetKey == nil && input.APIFormat == nil &&
			input.DisplayName == nil && input.AuthToken == nil && input.BaseURL == nil &&
			input.ModelsPath == nil && input.Enabled == nil {
			return errors.New("providers.update 至少要提供一个待修改字段")
		}
		return nil
	case "delete":
		if err := request.requireTarget(); err != nil {
			return err
		}
		return request.decodeInput(&providersvc.DeleteInput{})
	case "fetch_models", "test_provider":
		return request.requireTarget()
	case "update_model":
		if err := request.requireTarget(); err != nil {
			return err
		}
		return request.decodeInput(&providerModelMutation{})
	case "set_default_model", "test_model":
		if err := request.requireTarget(); err != nil {
			return err
		}
		return request.decodeInput(&providerModelTarget{})
	default:
		return unsupportedChange(request)
	}
}

func (s *Service) executeProvidersChange(ctx context.Context, actor *resolvedActor, request ChangeRequest, stateVersion int64) (any, error) {
	ctx = privateProviderMutationContext(ctx, actor.Actor)
	switch request.Operation {
	case "create":
		var input providerCreateRequest
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		return s.providers.Create(ctx, input.serviceInput())
	case "update":
		var input providerUpdateRequest
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Provider 更新缺少 configuration_version；请重新 plan")
		}
		return s.providers.PatchAtVersion(ctx, request.Target, input.patchInput(), stateVersion)
	case "delete":
		var input providersvc.DeleteInput
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Provider 删除缺少 configuration_version；请重新 plan")
		}
		return s.providers.DeleteAtVersion(ctx, request.Target, input, stateVersion)
	case "fetch_models":
		if stateVersion <= 0 {
			return nil, errors.New("Provider 模型同步缺少 configuration_version；请重新 plan")
		}
		return s.providers.FetchModelsAtVersion(ctx, request.Target, stateVersion)
	case "update_model":
		var input providerModelMutation
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Provider 模型更新缺少 configuration_version；请重新 plan")
		}
		return s.providers.UpdateModelAtVersion(ctx, request.Target, input.ModelID, input.Input, stateVersion)
	case "set_default_model":
		var input providerModelTarget
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Provider 默认模型更新缺少 configuration_version；请重新 plan")
		}
		return s.providers.SetDefaultModelAtVersion(ctx, request.Target, input.ModelID, stateVersion)
	case "test_provider":
		if stateVersion <= 0 {
			return nil, errors.New("Provider 测试缺少 configuration_version；请重新 plan")
		}
		return s.providers.TestProviderAtVersion(ctx, request.Target, stateVersion)
	case "test_model":
		var input providerModelTarget
		if err := request.decodeAppliedInput(&input); err != nil {
			return nil, err
		}
		if stateVersion <= 0 {
			return nil, errors.New("Provider 模型测试缺少 configuration_version；请重新 plan")
		}
		return s.providers.TestModelAtVersion(ctx, request.Target, input.ModelID, stateVersion)
	default:
		return nil, unsupportedChange(request)
	}
}

func providerChecks(values providerDomainValues, err error) []Check {
	if err != nil {
		return []Check{errorCheck(DomainProviders, "providers_readable", err)}
	}
	checks := []Check{okCheck(DomainProviders, "providers_readable", "Provider 配置、preset、模型卡与 runtime 默认选择可读取；远端连通性仅在显式 test 操作时验证")}
	enabled := 0
	for _, item := range values.Items {
		if item.Enabled {
			enabled++
		}
		if item.LastTestStatus == providersvc.TestStatusFailed {
			checks = append(checks, Check{
				Code: "provider_last_test_failed", Status: "warning",
				Message: item.LastTestError, Domain: DomainProviders, Target: item.Provider,
				Remedy: "核对 endpoint/token/model 后显式执行 test_provider", Verified: true,
			})
		}
	}
	switch {
	case len(values.Items) == 0:
		checks = append(checks, Check{
			Code: "provider_missing", Status: "error", Message: "尚未配置 Provider",
			Domain: DomainProviders, Remedy: "先 plan/apply providers.create", Verified: true,
		})
	case enabled == 0:
		checks = append(checks, Check{
			Code: "provider_enabled_missing", Status: "error", Message: "Provider 全部处于禁用状态",
			Domain: DomainProviders, Remedy: "启用至少一个有效 Provider", Verified: true,
		})
	case values.RuntimeOptions == nil || values.RuntimeOptions.DefaultSelection == nil:
		checks = append(checks, Check{
			Code: "provider_default_missing", Status: "warning", Message: "当前 runtime 没有默认模型选择",
			Domain: DomainProviders, Remedy: "设置一个已启用模型为默认", Verified: true,
		})
	}
	return checks
}

type providerModelMutation struct {
	ModelID string                       `json:"model_id"`
	Input   providersvc.UpdateModelInput `json:"input"`
}

type providerCreateRequest struct {
	ProviderKind string `json:"provider_kind"`
	Provider     string `json:"provider"`
	Visibility   string `json:"visibility,omitempty"`
	PresetKey    string `json:"preset_key"`
	APIFormat    string `json:"api_format"`
	DisplayName  string `json:"display_name"`
	AuthToken    string `json:"auth_token"`
	BaseURL      string `json:"base_url"`
	ModelsPath   string `json:"models_path"`
	Enabled      *bool  `json:"enabled,omitempty"`
}

func (r providerCreateRequest) serviceInput() providersvc.CreateInput {
	enabled := true
	if r.Enabled != nil {
		enabled = *r.Enabled
	}
	return providersvc.CreateInput{
		ProviderKind: r.ProviderKind, Provider: r.Provider, Visibility: r.Visibility,
		PresetKey: r.PresetKey, APIFormat: r.APIFormat, DisplayName: r.DisplayName,
		AuthToken: r.AuthToken, BaseURL: r.BaseURL, ModelsPath: r.ModelsPath, Enabled: enabled,
	}
}

type providerUpdateRequest struct {
	ProviderKind *string `json:"provider_kind,omitempty"`
	PresetKey    *string `json:"preset_key,omitempty"`
	APIFormat    *string `json:"api_format,omitempty"`
	DisplayName  *string `json:"display_name,omitempty"`
	AuthToken    *string `json:"auth_token,omitempty"`
	BaseURL      *string `json:"base_url,omitempty"`
	ModelsPath   *string `json:"models_path,omitempty"`
	Enabled      *bool   `json:"enabled,omitempty"`
}

func (r providerUpdateRequest) patchInput() providersvc.PatchInput {
	return providersvc.PatchInput{
		ProviderKind: r.ProviderKind,
		PresetKey:    r.PresetKey,
		APIFormat:    r.APIFormat,
		DisplayName:  r.DisplayName,
		AuthToken:    r.AuthToken,
		BaseURL:      r.BaseURL,
		ModelsPath:   r.ModelsPath,
		Enabled:      r.Enabled,
	}
}

// serviceInput 保留给完整快照 API 的兼容测试；配置执行路径必须使用 patchInput。
func (r providerUpdateRequest) serviceInput(current providersvc.Record) providersvc.UpdateInput {
	input := providersvc.UpdateInput{
		ProviderKind: current.ProviderKind,
		PresetKey:    current.PresetKey,
		APIFormat:    current.APIFormat,
		DisplayName:  current.DisplayName,
		BaseURL:      current.BaseURL,
		ModelsPath:   current.ModelsPath,
		Enabled:      current.Enabled,
		AuthToken:    r.AuthToken,
	}
	if r.ProviderKind != nil {
		input.ProviderKind = *r.ProviderKind
	}
	if r.PresetKey != nil {
		input.PresetKey = *r.PresetKey
	}
	if r.APIFormat != nil {
		input.APIFormat = *r.APIFormat
	}
	if r.DisplayName != nil {
		input.DisplayName = *r.DisplayName
	}
	if r.BaseURL != nil {
		input.BaseURL = *r.BaseURL
	}
	if r.ModelsPath != nil {
		input.ModelsPath = *r.ModelsPath
	}
	if r.Enabled != nil {
		input.Enabled = *r.Enabled
	}
	return input
}

type providerModelTarget struct {
	ModelID string `json:"model_id"`
}

func (s *Service) readProvidersConfiguration(ctx context.Context, actor *resolvedActor, target string) (any, []Check, int64, ScopeRef, error) {
	scope := actor.Context
	if actor.isSelfDM() {
		preferences, preferencesErr := s.prefs.Get(ctx, actor.OwnerUserID)
		if preferencesErr != nil {
			return nil, nil, 0, ScopeRef{Kind: ScopeKindAgent, ID: actor.AgentID}, preferencesErr
		}
		options, optionsErr := s.providers.ListOptionsForRuntime(ctx, preferences.AgentRuntimeKind)
		checks := []Check{okCheck(
			DomainProviders,
			"agent_runtime_model_catalog_readable",
			"仅返回当前 Agent 可选择的已启用 runtime Provider 与模型；端点、凭据、测试错误和使用关系不可见",
		)}
		if optionsErr != nil {
			checks = []Check{errorCheck(DomainProviders, "agent_runtime_model_catalog_readable", optionsErr)}
		}
		return map[string]any{
				"runtime_kind": preferences.AgentRuntimeKind,
				"catalog":      options,
			},
			checks,
			0,
			ScopeRef{Kind: ScopeKindAgent, ID: actor.AgentID},
			optionsErr
	}
	if target != "" {
		item, err := s.providers.GetPrivate(ctx, target)
		values := providerDomainValues{}
		if item != nil {
			values.Items = []providersvc.Record{*item}
		}
		var version int64
		if item != nil {
			version = item.ConfigurationVersion
		}
		return item, providerChecks(values, err), version, scope, err
	}
	items, err := s.providers.ListPrivate(ctx)
	if err != nil {
		return nil, providerChecks(providerDomainValues{}, err), 0, scope, err
	}
	preferences, err := s.prefs.Get(ctx, actor.OwnerUserID)
	if err != nil {
		return nil, providerChecks(providerDomainValues{}, err), 0, scope, err
	}
	options, err := s.providers.ListOptionsForRuntime(ctx, preferences.AgentRuntimeKind)
	values := providerDomainValues{
		Items: items, Presets: s.providers.ListPresets(), RuntimeOptions: options,
	}
	return values, providerChecks(values, err), 0, scope, err
}

func (s *Service) validateScopedProvidersChange(ctx context.Context, actor *resolvedActor, request ChangeRequest) error {
	if request.Operation == "create" {
		return nil
	}
	if _, err := s.providers.GetPrivate(ctx, request.Target); err != nil {
		return fmt.Errorf(
			"对话配置只能管理当前 owner 的私有 Provider；公共订阅 Provider 必须由人类运营界面管理: %w",
			err,
		)
	}
	return nil

}

func (s *Service) verifyDeletedProviders(ctx context.Context, actor *resolvedActor, request ChangeRequest) error {
	items, err := s.providers.List(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if strings.TrimSpace(item.Provider) == request.Target && item.CanManage {
			return fmt.Errorf("Provider %s 的可管理配置删除后仍存在", request.Target)
		}
	}

	return nil
}
