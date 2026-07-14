package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

// ResolveRuntimeConfig 解析 Agent 最终运行时要使用的 Provider 配置。
func (s *Service) ResolveRuntimeConfig(ctx context.Context, provider string, model string) (*clientopts.RuntimeConfig, error) {
	return s.ResolveRuntimeConfigForRuntime(ctx, provider, model, "claude")
}

// ResolveRuntimeConfigForRuntime 按 Agent runtime 类型解析最终 Provider 配置。
func (s *Service) ResolveRuntimeConfigForRuntime(ctx context.Context, provider string, model string, runtimeKind string) (*clientopts.RuntimeConfig, error) {
	items, err := s.listAndNormalize(ctx)
	if err != nil {
		return nil, err
	}
	runtimeKind = normalizeRuntimeKind(runtimeKind)
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
		defaultTarget, defaultErr := s.defaultRuntimeSelectionForRuntime(ctx, runtimeKind)
		if defaultErr != nil {
			return nil, defaultErr
		}
		if defaultTarget != nil {
			target = &defaultTarget.provider
			targetModel = defaultTarget.model.ModelID
		}
	}
	return s.runtimeConfigFromTarget(ctx, target, targetModel, runtimeKind)
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

func (s *Service) runtimeConfigFromTarget(
	ctx context.Context,
	target *providerstore.Entity,
	targetModel string,
	runtimeKind string,
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
	if !isAgentRuntimeProviderForRuntime(*target, runtimeKind) {
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
	targetModel = strings.TrimSpace(targetModel)
	var modelRecord *providerstore.ModelEntity
	var err error
	if targetModel == "" {
		modelRecord, err = s.defaultOrFirstEnabledModel(ctx, target.ID)
	} else {
		modelRecord, err = s.getModelByID(ctx, target.ID, targetModel)
	}
	if err != nil {
		return nil, err
	}
	if modelRecord == nil {
		if targetModel == "" {
			return nil, fmt.Errorf("provider=%s 缺少 model，请先选择该 Provider 下的模型", target.Provider)
		}
		return nil, fmt.Errorf("provider=%s 模型不存在: %s", target.Provider, targetModel)
	}
	if !modelRecord.Enabled {
		return nil, fmt.Errorf("provider=%s model=%s 已禁用", target.Provider, modelRecord.ModelID)
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
		Provider:      target.Provider,
		DisplayName:   target.DisplayName,
		AuthToken:     target.AuthToken,
		BaseURL:       target.BaseURL,
		Model:         normalizeModelID(modelRecord.ModelID),
		APIFormat:     target.APIFormat,
		Reasoning:     modelHasReasoningCapability(*modelRecord),
		Vision:        modelHasVisionCapability(*modelRecord),
		ContextWindow: modelContextWindow(modelRecord),
	}, nil
}

// modelContextWindow 把模型卡的可选值投影为运行时零值语义。
func modelContextWindow(model *providerstore.ModelEntity) int {
	if model == nil {
		return 0
	}
	contextWindow := contextWindowOrKnown(model.ModelID, model.ContextWindow)
	if contextWindow == nil || *contextWindow <= 0 {
		return 0
	}
	return *contextWindow
}
