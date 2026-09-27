// INPUT: Provider/model 测试目标、网络响应与期望 configuration_version。
// OUTPUT: 脱敏测试结果，以及与模型启用/默认选择同事务的测试状态和配置绑定能力证据。
// POS: Provider 连通性测试到持久化配置聚合的提交边界。
package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

// TestProvider 测试 Provider 的模型列表端点和最小生成请求。
func (s *Service) TestProvider(ctx context.Context, provider string) (*TestResult, error) {
	item, err := s.requireProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	if err = s.requireProviderManagement(ctx, *item); err != nil {
		return nil, err
	}
	return s.testProviderForItem(ctx, *item, item.ConfigurationVersion)
}

// TestProviderAtVersion 只把测试结果写回被测试的 Provider 版本。
func (s *Service) TestProviderAtVersion(
	ctx context.Context,
	provider string,
	expectedVersion int64,
) (*TestResult, error) {
	item, err := s.requireProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	if err = s.requireProviderManagement(ctx, *item); err != nil {
		return nil, err
	}
	return s.testProviderForItem(ctx, *item, expectedVersion)
}

// TestPublicProvider 测试公共 Provider 的模型列表端点和最小生成请求。
func (s *Service) TestPublicProvider(ctx context.Context, provider string) (*TestResult, error) {
	item, err := s.requirePublicProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	return s.testProviderForItem(ctx, *item, item.ConfigurationVersion)
}

func (s *Service) testProviderForItem(
	ctx context.Context,
	item providerstore.Entity,
	expectedVersion int64,
) (*TestResult, error) {
	var models []remoteModel
	if strings.TrimSpace(item.ModelsPath) != "" {
		var err error
		models, err = s.fetchRemoteModels(ctx, item)
		if err != nil {
			return s.persistTestResult(ctx, item, "", err, expectedVersion, nil)
		}
	}
	modelID := s.pickTestModel(ctx, item, models)
	if modelID == "" {
		return s.persistTestResult(ctx, item, "", errors.New("未找到可测试模型"), expectedVersion, nil)
	}
	return s.runModelTest(ctx, item, modelID, expectedVersion)
}

// TestModel 测试指定模型的最小生成请求。
func (s *Service) TestModel(ctx context.Context, provider string, modelID string) (*TestResult, error) {
	item, err := s.requireProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	if err = s.requireProviderManagement(ctx, *item); err != nil {
		return nil, err
	}
	return s.testModelForItem(ctx, *item, modelID, item.ConfigurationVersion)
}

// TestModelAtVersion 只把模型测试结果写回被测试的 Provider 版本。
func (s *Service) TestModelAtVersion(
	ctx context.Context,
	provider string,
	modelID string,
	expectedVersion int64,
) (*TestResult, error) {
	item, err := s.requireProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	if err = s.requireProviderManagement(ctx, *item); err != nil {
		return nil, err
	}
	return s.testModelForItem(ctx, *item, modelID, expectedVersion)
}

// TestPublicModel 测试公共 Provider 指定模型的最小生成请求。
func (s *Service) TestPublicModel(ctx context.Context, provider string, modelID string) (*TestResult, error) {
	item, err := s.requirePublicProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	return s.testModelForItem(ctx, *item, modelID, item.ConfigurationVersion)
}

func (s *Service) testModelForItem(
	ctx context.Context,
	item providerstore.Entity,
	modelID string,
	expectedVersion int64,
) (*TestResult, error) {
	modelID = normalizeModelID(modelID)
	if modelID == "" {
		return nil, fmt.Errorf("%w: model_id 不能为空", ErrInvalidInput)
	}
	return s.runModelTest(ctx, item, modelID, expectedVersion)
}

func (s *Service) ensureTestedModelReadyInMutation(
	ctx context.Context,
	item providerstore.Entity,
	modelID string,
	mutation *providerstore.Mutation,
	shouldAutoDefault bool,
) error {
	modelID = normalizeModelID(modelID)
	if modelID == "" {
		return nil
	}
	model, err := mutation.GetModel(ctx, modelID)
	if err != nil {
		return err
	}
	if model == nil {
		capabilities, category, contextWindow, maxOutput := defaultModelCard(modelID, item.ProviderKind)
		now := s.now()
		model = &providerstore.ModelEntity{
			ID:                       s.idFactory("provider_model"),
			ProviderID:               item.ID,
			ModelID:                  modelID,
			DisplayName:              modelID,
			Category:                 category,
			Enabled:                  true,
			IsDefault:                false,
			CapabilitiesAutoJSON:     encodeModelAutoCapabilities(capabilities),
			CapabilitiesOverrideJSON: "{}",
			ContextWindow:            contextWindow,
			MaxOutputTokens:          maxOutput,
			ProviderOptionsJSON:      "{}",
			LastSeenAt:               now,
			CreatedAt:                now,
			UpdatedAt:                now,
		}
		if err = mutation.UpsertModels(ctx, []providerstore.ModelEntity{*model}); err != nil {
			return err
		}
	} else {
		identityChanged := normalizeModelEntityIdentity(model, modelID)
		enabledChanged := !model.Enabled
		if enabledChanged {
			model.Enabled = true
		}
		if identityChanged || enabledChanged {
			model.UpdatedAt = s.now()
			if err = mutation.UpdateModel(ctx, *model); err != nil {
				return err
			}
		}
	}
	if !shouldAutoDefault {
		return nil
	}
	hasDefault, err := mutation.HasDefaultModelInScope(ctx)
	if err != nil || hasDefault {
		return err
	}
	return mutation.UpdateDefaultModel(ctx, modelID, s.now())
}

func (s *Service) pickTestModel(ctx context.Context, item providerstore.Entity, remoteModels []remoteModel) string {
	localModels, err := s.repository.ListModelsByProviderID(ctx, item.ID)
	if err == nil {
		for _, model := range localModels {
			modelID := normalizeModelID(model.ModelID)
			if model.Enabled && modelID != "" {
				return modelID
			}
		}
	}
	for _, model := range remoteModels {
		modelID := normalizeModelID(model.ID)
		if modelID != "" {
			return modelID
		}
	}
	return ""
}

func (s *Service) persistTestResult(
	ctx context.Context,
	item providerstore.Entity,
	modelID string,
	testErr error,
	expectedVersion int64,
	probe *modelProbeEvidence,
) (*TestResult, error) {
	if probe != nil && probe.PreserveSelection {
		// Serialize only the short write phase. Other models may advance the
		// aggregate revision; exact route/options/model evidence must remain intact.
		s.probeCommitMu.Lock()
		defer s.probeCommitMu.Unlock()
		lookup := s.requireProvider
		if item.Visibility == providerstore.VisibilityPublic {
			lookup = s.requirePublicProvider
		}
		fresh, err := lookup(ctx, item.Provider)
		if err != nil {
			return nil, err
		}
		if fresh.ID != item.ID {
			return nil, ErrConfigurationVersionConflict
		}
		item = *fresh
		expectedVersion = item.ConfigurationVersion
	}
	now := s.now()
	item.LastTestAt = &now
	item.LastTestError = ""
	item.LastTestStatus = TestStatusSuccess
	success := true
	if testErr != nil {
		success = false
		item.LastTestStatus = TestStatusFailed
		item.LastTestError = sanitizeErrorMessage(testErr.Error(), item.AuthToken)
	}
	shouldAutoDefault := false
	var err error
	if testErr == nil && (probe == nil || !probe.PreserveSelection) {
		shouldAutoDefault, err = s.shouldAutoDefaultDiscoveredModel(ctx, item)
		if err != nil {
			return nil, err
		}
	}
	committedVersion, err := s.repository.WithProviderMutation(
		ctx,
		item.ID,
		expectedVersion,
		func(mutation *providerstore.Mutation) error {
			if testErr == nil && (probe == nil || !probe.PreserveSelection) {
				if readyErr := s.ensureTestedModelReadyInMutation(
					ctx,
					item,
					modelID,
					mutation,
					shouldAutoDefault,
				); readyErr != nil {
					return readyErr
				}
			}
			if probe != nil {
				model, readErr := mutation.GetModel(ctx, modelID)
				if readErr != nil {
					return readErr
				}
				if probe.PreserveSelection && (model == nil || model.ID != probe.BaselineModelID || model.CapabilitiesAutoJSON != probe.BaselineAutoJSON) {
					return ErrConfigurationVersionConflict
				}
				if model != nil {
					if probe.Fingerprint != modelProbeFingerprint(item, *model) {
						return ErrConfigurationVersionConflict
					}
					model.CapabilitiesAutoJSON = withModelProbe(model.CapabilitiesAutoJSON, *probe)
					model.UpdatedAt = now
					if writeErr := mutation.UpdateModelFacts(ctx, *model); writeErr != nil {
						return writeErr
					}
				}
			}
			return mutation.UpdateTestState(ctx, item)
		},
	)
	if err != nil {
		return nil, err
	}
	return &TestResult{
		Provider:             item.Provider,
		Model:                normalizeModelID(modelID),
		Success:              success,
		Status:               item.LastTestStatus,
		Error:                item.LastTestError,
		TestedAt:             &now,
		ConfigurationVersion: committedVersion,
	}, nil
}
