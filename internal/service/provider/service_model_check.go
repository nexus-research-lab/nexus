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
	var models []remoteModel
	if strings.TrimSpace(item.ModelsPath) != "" {
		models, err = s.fetchRemoteModels(ctx, *item)
		if err != nil {
			return s.persistTestResult(ctx, *item, "", err)
		}
	}
	modelID := s.pickTestModel(ctx, *item, models)
	if modelID == "" {
		return s.persistTestResult(ctx, *item, "", errors.New("未找到可测试模型"))
	}
	testErr := s.sendMinimalModelRequest(ctx, *item, modelID)
	if testErr == nil {
		if readyErr := s.ensureTestedModelReady(ctx, *item, modelID); readyErr != nil {
			return nil, readyErr
		}
	}
	return s.persistTestResult(ctx, *item, modelID, testErr)
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
	modelID = normalizeModelID(modelID)
	if modelID == "" {
		return nil, errors.New("model_id 不能为空")
	}
	testErr := s.sendMinimalModelRequest(ctx, *item, modelID)
	if testErr == nil {
		if readyErr := s.ensureTestedModelReady(ctx, *item, modelID); readyErr != nil {
			return nil, readyErr
		}
	}
	return s.persistTestResult(ctx, *item, modelID, testErr)
}

func (s *Service) ensureTestedModelReady(
	ctx context.Context,
	item providerstore.Entity,
	modelID string,
) error {
	modelID = normalizeModelID(modelID)
	if modelID == "" {
		return nil
	}
	model, err := s.getModelByID(ctx, item.ID, modelID)
	if err != nil {
		return err
	}
	if model == nil {
		capabilities, category, contextWindow, maxOutput := defaultModelCard()
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
		if err = s.repository.UpsertModels(ctx, []providerstore.ModelEntity{*model}); err != nil {
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
			if err = s.repository.UpdateModel(ctx, *model); err != nil {
				return err
			}
		}
	}
	return s.autoDefaultDiscoveredModel(ctx, item, []remoteModel{{ID: modelID}})
}

func (s *Service) sendMinimalModelRequest(ctx context.Context, item providerstore.Entity, modelID string) error {
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

func (s *Service) persistTestResult(ctx context.Context, item providerstore.Entity, modelID string, testErr error) (*TestResult, error) {
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
	if err := s.repository.UpdateTestState(ctx, item); err != nil {
		return nil, err
	}
	return &TestResult{
		Provider: item.Provider,
		Model:    normalizeModelID(modelID),
		Success:  success,
		Status:   item.LastTestStatus,
		Error:    item.LastTestError,
		TestedAt: &now,
	}, nil
}
