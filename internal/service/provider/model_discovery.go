// INPUT: Provider 端点、认证配置与远端模型目录响应。
// OUTPUT: 已清洗的远端模型卡，以及是否允许自动选择默认模型的决策。
// POS: Provider 外部模型发现的只读网络阶段；持久化由 model_mutation 负责。
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

func (s *Service) fetchRemoteModels(ctx context.Context, item providerstore.Entity) ([]remoteModel, error) {
	endpoint := endpointURL(item, providerEndpointModels)
	logger := s.loggerFor(ctx)
	logger.Info(
		"请求 Provider 模型列表",
		"provider", item.Provider,
		"preset_key", item.PresetKey,
		"api_format", item.APIFormat,
		"endpoint", endpoint,
		"models_path", item.ModelsPath,
	)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	applyProviderHeaders(request, item)
	response, err := s.client.Do(request)
	if err != nil {
		logger.Warn(
			"Provider 模型列表请求失败",
			"provider", item.Provider,
			"endpoint", endpoint,
			"err", sanitizeErrorMessage(err.Error(), item.AuthToken),
		)
		return nil, sanitizeHTTPError(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		logger.Warn(
			"Provider 模型列表响应读取失败",
			"provider", item.Provider,
			"endpoint", endpoint,
			"status", response.StatusCode,
			"err", sanitizeErrorMessage(err.Error(), item.AuthToken),
		)
		return nil, sanitizeHTTPError(err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		logger.Warn(
			"Provider 模型列表请求返回错误",
			"provider", item.Provider,
			"endpoint", endpoint,
			"status", response.StatusCode,
			"body_preview", sanitizedBodyPreview(body, item.AuthToken),
		)
		return nil, fmt.Errorf("models 请求失败: status=%d body=%s", response.StatusCode, sanitizeHTTPBody(body, item.AuthToken))
	}
	models, err := parseModelList(body)
	if err != nil {
		logger.Warn(
			"Provider 模型列表响应解析失败",
			"provider", item.Provider,
			"endpoint", endpoint,
			"status", response.StatusCode,
			"body_preview", sanitizedBodyPreview(body, item.AuthToken),
			"err", err,
		)
		return nil, err
	}
	logger.Info(
		"Provider 模型列表请求成功",
		"provider", item.Provider,
		"endpoint", endpoint,
		"status", response.StatusCode,
		"model_count", len(models),
		"model_ids", previewRemoteModelIDs(models, 10),
	)
	return models, nil
}

func (s *Service) shouldAutoDefaultDiscoveredModel(
	ctx context.Context,
	item providerstore.Entity,
) (bool, error) {
	if !item.Enabled {
		return false, nil
	}
	switch item.ProviderKind {
	case ProviderKindLLM:
		if !isAnyAgentRuntimeProvider(item) {
			return false, nil
		}
		target, err := s.defaultRuntimeSelection(ctx)
		if err != nil {
			return false, err
		}
		if target != nil {
			return false, nil
		}
	case ProviderKindImageGeneration:
		target, err := s.defaultImageSelection(ctx)
		if err != nil {
			return false, err
		}
		if target != nil {
			return false, nil
		}
	default:
		return false, nil
	}
	return true, nil
}

func parseModelList(body []byte) ([]remoteModel, error) {
	var payload struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("models 响应不是合法 JSON: %w", err)
	}
	result := make([]remoteModel, 0, len(payload.Data))
	for _, item := range payload.Data {
		model := remoteModelFromCard(item)
		modelID := normalizeModelID(model.ID)
		if modelID == "" {
			continue
		}
		model.ID = modelID
		result = append(result, model)
	}
	return result, nil
}

func firstRemoteModelID(models []remoteModel) string {
	for _, model := range models {
		modelID := normalizeModelID(model.ID)
		if modelID != "" {
			return modelID
		}
	}
	return ""
}

func previewRemoteModelIDs(models []remoteModel, limit int) []string {
	if limit <= 0 || len(models) == 0 {
		return []string{}
	}
	if len(models) < limit {
		limit = len(models)
	}
	result := make([]string, 0, limit)
	for index := 0; index < limit; index++ {
		modelID := normalizeModelID(models[index].ID)
		if modelID == "" {
			continue
		}
		result = append(result, modelID)
	}
	return result
}
