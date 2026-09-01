// INPUT: Provider/model 测试目标、网络响应与期望 configuration_version。
// OUTPUT: 脱敏测试结果，以及与模型启用/默认选择同事务的测试状态。
// POS: Provider 连通性测试到持久化配置聚合的提交边界。
package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
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
			return s.persistTestResult(ctx, item, "", err, expectedVersion)
		}
	}
	modelID := s.pickTestModel(ctx, item, models)
	if modelID == "" {
		return s.persistTestResult(ctx, item, "", errors.New("未找到可测试模型"), expectedVersion)
	}
	testErr := s.sendMinimalModelRequest(ctx, item, modelID)
	return s.persistTestResult(ctx, item, modelID, testErr, expectedVersion)
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
	testErr := s.sendMinimalModelRequest(ctx, item, modelID)
	return s.persistTestResult(ctx, item, modelID, testErr, expectedVersion)
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
			CapabilitiesAutoJSON:     encodeModelCapabilities(capabilities),
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

func (s *Service) sendMinimalModelRequest(ctx context.Context, item providerstore.Entity, modelID string) error {
	if err := validateModelEndpoint(item); err != nil {
		return err
	}
	endpoint := endpointURL(item, item.APIFormat)
	payload, err := minimalPayload(item, modelID)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	applyProviderHeaders(request, item)
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return sanitizeHTTPError(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return sanitizeHTTPError(err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("模型请求失败: status=%d body=%s", response.StatusCode, sanitizeHTTPBody(body, item.AuthToken))
	}
	return nil
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
) (*TestResult, error) {
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
	if testErr == nil {
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
			if testErr == nil {
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
