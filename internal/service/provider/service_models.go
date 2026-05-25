package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

const (
	providerEndpointModels            = "models"
	providerEndpointChatCompletions   = APIFormatChatCompletions
	providerEndpointResponses         = APIFormatResponses
	providerEndpointAnthropicMessages = APIFormatAnthropicMessages
)

var bearerTokenPattern = regexp.MustCompile(`(?i)(bearer\s+)[a-z0-9._\-]+`)

type remoteModel struct {
	ID              string
	DisplayName     string
	Category        string
	Capabilities    ModelCapabilities
	ContextWindow   *int
	MaxOutputTokens *int
}

// FetchModels 从远端 /models 端点拉取模型列表并合并到本地模型卡。
func (s *Service) FetchModels(ctx context.Context, provider string) (*FetchModelsResult, error) {
	item, err := s.requireProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	models, err := s.fetchRemoteModels(ctx, *item)
	if err != nil {
		return nil, err
	}
	now := s.now()
	entities := make([]providerstore.ModelEntity, 0, len(models))
	for _, model := range models {
		modelID := strings.TrimSpace(model.ID)
		if modelID == "" {
			continue
		}
		capabilities, category, contextWindow, maxOutput := model.modelCard()
		displayName := strings.TrimSpace(model.DisplayName)
		if displayName == "" {
			displayName = modelID
		}
		entities = append(entities, providerstore.ModelEntity{
			ID:                       s.idFactory("provider_model"),
			ProviderID:               item.ID,
			ModelID:                  modelID,
			DisplayName:              displayName,
			Category:                 category,
			Enabled:                  false,
			IsDefault:                false,
			CapabilitiesAutoJSON:     encodeModelCapabilities(capabilities),
			CapabilitiesOverrideJSON: "{}",
			ContextWindow:            contextWindow,
			MaxOutputTokens:          maxOutput,
			ProviderOptionsJSON:      "{}",
			LastSeenAt:               now,
			CreatedAt:                now,
			UpdatedAt:                now,
		})
	}
	if len(entities) == 0 {
		return nil, errors.New("远端没有返回可用模型")
	}
	if err = s.repository.UpsertModels(ctx, entities); err != nil {
		return nil, err
	}
	saved, err := s.modelsForRecord(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	return &FetchModelsResult{
		Provider: item.Provider,
		Models:   saved,
		Count:    len(saved),
	}, nil
}

// UpdateModel 更新模型开关、能力覆盖和 Provider 原生 options。
func (s *Service) UpdateModel(ctx context.Context, provider string, modelID string, input UpdateModelInput) (*ModelRecord, error) {
	item, err := s.requireProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return nil, errors.New("model_id 不能为空")
	}
	if input.IsDefault {
		if item.ProviderKind == ProviderKindLLM && (!item.Enabled || !isAgentRuntimeProvider(*item)) {
			return nil, fmt.Errorf("provider=%s 暂不可设为 Agent 默认模型", item.Provider)
		}
		if item.ProviderKind != ProviderKindLLM && item.ProviderKind != ProviderKindImageGeneration {
			return nil, fmt.Errorf("provider=%s 暂不可设置默认模型", item.Provider)
		}
	}
	model, err := s.repository.GetModel(ctx, item.ID, modelID)
	if err != nil {
		return nil, err
	}
	if model == nil {
		capabilities, category, contextWindow, maxOutput := defaultModelCard()
		now := s.now()
		if input.ContextWindow != nil {
			contextWindow = input.ContextWindow
		}
		if input.MaxOutputTokens != nil {
			maxOutput = input.MaxOutputTokens
		}
		model = &providerstore.ModelEntity{
			ID:                       s.idFactory("provider_model"),
			ProviderID:               item.ID,
			ModelID:                  modelID,
			DisplayName:              modelID,
			Category:                 category,
			Enabled:                  input.Enabled || input.IsDefault,
			IsDefault:                input.IsDefault,
			CapabilitiesAutoJSON:     encodeModelCapabilities(capabilities),
			CapabilitiesOverrideJSON: encodeModelCapabilities(input.CapabilitiesOverride),
			ContextWindow:            contextWindow,
			MaxOutputTokens:          maxOutput,
			ProviderOptionsJSON:      encodeProviderOptions(input.ProviderOptions),
			LastSeenAt:               now,
			CreatedAt:                now,
			UpdatedAt:                now,
		}
		if err = s.repository.UpsertModels(ctx, []providerstore.ModelEntity{*model}); err != nil {
			return nil, err
		}
	} else {
		if model.IsDefault && !input.Enabled && !input.IsDefault {
			return nil, fmt.Errorf("默认模型不能禁用: %s", modelID)
		}
		model.Enabled = input.Enabled || input.IsDefault || model.IsDefault
		model.IsDefault = input.IsDefault || model.IsDefault
		model.CapabilitiesOverrideJSON = encodeModelCapabilities(input.CapabilitiesOverride)
		model.ContextWindow = input.ContextWindow
		model.MaxOutputTokens = input.MaxOutputTokens
		model.ProviderOptionsJSON = encodeProviderOptions(input.ProviderOptions)
		model.UpdatedAt = s.now()
		if err = s.repository.UpdateModel(ctx, *model); err != nil {
			return nil, err
		}
	}
	if input.IsDefault {
		if err = s.repository.UpdateDefaultModel(ctx, item.ID, model.ModelID, s.now()); err != nil {
			return nil, err
		}
	}
	updated, err := s.repository.GetModel(ctx, item.ID, modelID)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, fmt.Errorf("模型不存在: %s", modelID)
	}
	record := toModelRecord(*updated)
	return &record, nil
}

// SetDefaultModel 把指定模型设置为当前 Provider 类型的默认模型，不改写模型卡其它字段。
func (s *Service) SetDefaultModel(ctx context.Context, provider string, modelID string) (*ModelRecord, error) {
	item, err := s.requireProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return nil, errors.New("model_id 不能为空")
	}
	if item.ProviderKind == ProviderKindLLM && (!item.Enabled || !isAgentRuntimeProvider(*item)) {
		return nil, fmt.Errorf("provider=%s 暂不可设为 Agent 默认模型", item.Provider)
	}
	model, err := s.repository.GetModel(ctx, item.ID, modelID)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, fmt.Errorf("模型不存在: %s", modelID)
	}
	if err = s.repository.UpdateDefaultModel(ctx, item.ID, model.ModelID, s.now()); err != nil {
		return nil, err
	}
	updated, err := s.repository.GetModel(ctx, item.ID, modelID)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, fmt.Errorf("模型不存在: %s", modelID)
	}
	record := toModelRecord(*updated)
	return &record, nil
}

// TestProvider 测试 Provider 的模型列表端点和最小生成请求。
func (s *Service) TestProvider(ctx context.Context, provider string) (*TestResult, error) {
	item, err := s.requireProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	models, modelsErr := s.fetchRemoteModels(ctx, *item)
	if modelsErr != nil {
		return s.persistTestResult(ctx, *item, "", modelsErr)
	}
	modelID := s.pickTestModel(ctx, *item, models)
	if modelID == "" {
		return s.persistTestResult(ctx, *item, "", errors.New("未找到可测试模型"))
	}
	testErr := s.sendMinimalModelRequest(ctx, *item, modelID)
	return s.persistTestResult(ctx, *item, modelID, testErr)
}

// TestModel 测试指定模型的最小生成请求。
func (s *Service) TestModel(ctx context.Context, provider string, modelID string) (*TestResult, error) {
	item, err := s.requireProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return nil, errors.New("model_id 不能为空")
	}
	testErr := s.sendMinimalModelRequest(ctx, *item, modelID)
	return s.persistTestResult(ctx, *item, modelID, testErr)
}

func (s *Service) modelsForRecord(ctx context.Context, providerID string) ([]ModelRecord, error) {
	items, err := s.repository.ListModelsByProviderID(ctx, providerID)
	if err != nil {
		return nil, err
	}
	result := make([]ModelRecord, 0, len(items))
	for _, item := range items {
		result = append(result, toModelRecord(item))
	}
	return result, nil
}

func (s *Service) requireProvider(ctx context.Context, provider string) (*providerstore.Entity, error) {
	normalizedProvider, err := NormalizeProvider(provider, false)
	if err != nil {
		return nil, err
	}
	item, err := s.repository.GetByProvider(ctx, normalizedProvider)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("provider 不存在: %s", normalizedProvider)
	}
	if strings.TrimSpace(item.AuthToken) == "" {
		return nil, fmt.Errorf("provider=%s 缺少 auth_token", item.Provider)
	}
	if strings.TrimSpace(item.BaseURL) == "" {
		return nil, fmt.Errorf("provider=%s 缺少 base_url", item.Provider)
	}
	return item, nil
}

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
		"model_ids", previewRemoteModelIDs(models, 40),
		"body_preview", sanitizedBodyPreview(body, item.AuthToken),
	)
	return models, nil
}

func (s *Service) sendMinimalModelRequest(ctx context.Context, item providerstore.Entity, modelID string) error {
	endpoint := endpointURL(item, item.APIFormat)
	payload, err := minimalPayload(item.APIFormat, modelID)
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
			if model.Enabled && strings.TrimSpace(model.ModelID) != "" {
				return model.ModelID
			}
		}
	}
	for _, model := range remoteModels {
		if strings.TrimSpace(model.ID) != "" {
			return strings.TrimSpace(model.ID)
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
		Model:    strings.TrimSpace(modelID),
		Success:  success,
		Status:   item.LastTestStatus,
		Error:    item.LastTestError,
		TestedAt: &now,
	}, nil
}

func toModelRecord(item providerstore.ModelEntity) ModelRecord {
	createdAt := item.CreatedAt
	updatedAt := item.UpdatedAt
	lastSeenAt := item.LastSeenAt
	return ModelRecord{
		ID:                   item.ID,
		ProviderID:           item.ProviderID,
		ModelID:              item.ModelID,
		DisplayName:          item.DisplayName,
		Category:             item.Category,
		Enabled:              item.Enabled,
		IsDefault:            item.IsDefault,
		CapabilitiesAuto:     decodeModelCapabilities(item.CapabilitiesAutoJSON),
		CapabilitiesOverride: decodeModelCapabilities(item.CapabilitiesOverrideJSON),
		ContextWindow:        item.ContextWindow,
		MaxOutputTokens:      item.MaxOutputTokens,
		ProviderOptions:      decodeProviderOptions(item.ProviderOptionsJSON),
		LastSeenAt:           &lastSeenAt,
		CreatedAt:            &createdAt,
		UpdatedAt:            &updatedAt,
	}
}

func endpointURL(item providerstore.Entity, endpointKey string) string {
	switch endpointKey {
	case providerEndpointModels:
		return joinEndpointURL(item.BaseURL, item.ModelsPath)
	case providerEndpointResponses:
		return joinEndpointURL(item.BaseURL, "/responses")
	case providerEndpointAnthropicMessages:
		return joinEndpointURL(item.BaseURL, "/v1/messages")
	default:
		return joinEndpointURL(item.BaseURL, "/chat/completions")
	}
}

func joinEndpointURL(baseURL string, endpointPath string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	path := strings.TrimSpace(endpointPath)
	if path == "" {
		return base
	}
	if parsed, err := url.Parse(path); err == nil && parsed.IsAbs() {
		return path
	}
	return base + "/" + strings.TrimLeft(path, "/")
}

func applyProviderHeaders(request *http.Request, item providerstore.Entity) {
	token := strings.TrimSpace(item.AuthToken)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if normalizeAPIFormat(item.APIFormat) == APIFormatAnthropicMessages {
		if token != "" {
			request.Header.Set("x-api-key", token)
		}
		request.Header.Set("anthropic-version", "2023-06-01")
	}
}

func minimalPayload(apiFormat string, modelID string) ([]byte, error) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return nil, errors.New("model 不能为空")
	}
	switch normalizeAPIFormat(apiFormat) {
	case APIFormatResponses:
		return json.Marshal(map[string]any{
			"model":             modelID,
			"input":             "ping",
			"max_output_tokens": 1,
			"stream":            false,
		})
	case APIFormatAnthropicMessages:
		return json.Marshal(map[string]any{
			"model":      modelID,
			"max_tokens": 1,
			"stream":     false,
			"messages": []map[string]string{
				{"role": "user", "content": "ping"},
			},
		})
	default:
		return json.Marshal(map[string]any{
			"model":      modelID,
			"max_tokens": 1,
			"stream":     false,
			"messages": []map[string]string{
				{"role": "user", "content": "ping"},
			},
		})
	}
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
		modelID := strings.TrimSpace(model.ID)
		if modelID == "" {
			continue
		}
		model.ID = modelID
		result = append(result, model)
	}
	return result, nil
}

func defaultModelCard() (ModelCapabilities, string, *int, *int) {
	return ModelCapabilities{}, "chat", nil, nil
}

func (model remoteModel) modelCard() (ModelCapabilities, string, *int, *int) {
	category := strings.TrimSpace(model.Category)
	if category == "" {
		category = "chat"
	}
	return model.Capabilities, category, model.ContextWindow, model.MaxOutputTokens
}

func remoteModelFromCard(card map[string]any) remoteModel {
	capabilities := modelCapabilitiesFromCard(card)
	return remoteModel{
		ID:              firstStringField(card, "id", "model", "name"),
		DisplayName:     firstStringField(card, "display_name", "displayName", "name"),
		Category:        modelCategoryFromCard(card, capabilities),
		Capabilities:    capabilities,
		ContextWindow:   firstIntField(card, "context_length", "context_window", "max_context_length", "input_token_limit", "max_input_tokens"),
		MaxOutputTokens: firstIntField(card, "max_output_tokens", "output_token_limit", "max_tokens", "max_completion_tokens"),
	}
}

func modelCapabilitiesFromCard(card map[string]any) ModelCapabilities {
	return ModelCapabilities{
		Vision: capabilityPointerFromCard(
			card,
			"vision",
			"image_input",
			"image_in",
			"supports_vision",
			"supports_image_input",
			"supports_image_in",
			"supports_video_in",
		),
		ImageOutput: capabilityPointerFromCard(
			card,
			"image_output",
			"image_out",
			"supports_image_output",
			"supports_image_out",
		),
		ToolCalling: capabilityPointerFromCard(
			card,
			"tool_calling",
			"tools",
			"function_calling",
			"supports_tool_calling",
			"supports_tools",
			"supports_function_calling",
		),
		Reasoning: capabilityPointerFromCard(
			card,
			"reasoning",
			"thinking",
			"supports_reasoning",
			"supports_thinking",
		),
		Embedding: capabilityPointerFromCard(
			card,
			"embedding",
			"embeddings",
			"supports_embedding",
			"supports_embeddings",
		),
	}
}

func modelCategoryFromCard(card map[string]any, capabilities ModelCapabilities) string {
	for _, value := range []string{
		firstStringField(card, "category", "model_category"),
		firstStringField(card, "model_type", "mode"),
		firstStringField(card, "type"),
	} {
		if category := normalizeModelCategory(value); category != "" {
			return category
		}
	}
	if capabilities.Embedding != nil && *capabilities.Embedding {
		return "embedding"
	}
	if capabilities.ImageOutput != nil && *capabilities.ImageOutput {
		return "image"
	}
	return "chat"
}

func normalizeModelCategory(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" || normalized == "model" {
		return ""
	}
	switch {
	case strings.Contains(normalized, "embed"):
		return "embedding"
	case strings.Contains(normalized, "image"):
		return "image"
	case strings.Contains(normalized, "audio"):
		return "audio"
	case strings.Contains(normalized, "video"):
		return "video"
	case strings.Contains(normalized, "rerank"):
		return "rerank"
	default:
		return "chat"
	}
}

func capabilityPointerFromCard(card map[string]any, keys ...string) *bool {
	for _, source := range modelCardSources(card) {
		for _, key := range keys {
			if value, exists := source[key]; exists {
				if parsed, ok := boolFromAny(value); ok {
					return boolPointer(parsed)
				}
			}
		}
	}
	tokens := capabilityTokensFromCard(card)
	for _, token := range tokens {
		for _, key := range keys {
			if token == normalizeCapabilityToken(key) {
				return boolPointer(true)
			}
		}
	}
	return nil
}

func modelCardSources(card map[string]any) []map[string]any {
	result := []map[string]any{card}
	for _, key := range []string{"capabilities", "features", "limits"} {
		if nested, ok := mapFromAny(card[key]); ok {
			result = append(result, nested)
		}
	}
	return result
}

func capabilityTokensFromCard(card map[string]any) []string {
	result := []string{}
	for _, key := range []string{"capabilities", "features", "supported_features"} {
		values, ok := stringSliceFromAny(card[key])
		if !ok {
			continue
		}
		for _, value := range values {
			token := normalizeCapabilityToken(value)
			if token != "" {
				result = append(result, token)
			}
		}
	}
	return result
}

func firstStringField(card map[string]any, keys ...string) string {
	for _, source := range modelCardSources(card) {
		for _, key := range keys {
			if value, ok := stringFromAny(source[key]); ok {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func firstIntField(card map[string]any, keys ...string) *int {
	for _, source := range modelCardSources(card) {
		for _, key := range keys {
			if value, ok := intFromAny(source[key]); ok {
				return &value
			}
		}
	}
	return nil
}

func stringFromAny(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return typed, true
	default:
		return "", false
	}
}

func stringSliceFromAny(value any) ([]string, bool) {
	items, ok := value.([]any)
	if !ok {
		return nil, false
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if value, ok := stringFromAny(item); ok {
			result = append(result, value)
		}
	}
	return result, true
}

func mapFromAny(value any) (map[string]any, bool) {
	typed, ok := value.(map[string]any)
	return typed, ok
}

func boolFromAny(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "yes", "y", "1", "supported", "enabled":
			return true, true
		case "false", "no", "n", "0", "unsupported", "disabled":
			return false, true
		default:
			return false, false
		}
	case float64:
		if typed == 1 {
			return true, true
		}
		if typed == 0 {
			return false, true
		}
	case map[string]any:
		return boolFromNestedCapability(typed)
	}
	return false, false
}

func boolFromNestedCapability(value map[string]any) (bool, bool) {
	for _, key := range []string{"supported", "enabled", "available"} {
		if parsed, ok := boolFromAny(value[key]); ok {
			return parsed, true
		}
	}
	return false, false
}

func intFromAny(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		if typed <= 0 {
			return 0, false
		}
		return int(typed), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil || parsed <= 0 {
			return 0, false
		}
		return parsed, true
	}
	return 0, false
}

func normalizeCapabilityToken(value string) string {
	replacer := strings.NewReplacer("_", "", "-", "", " ", "")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(value)))
}

func encodeModelCapabilities(input ModelCapabilities) string {
	payload, err := json.Marshal(input)
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func decodeModelCapabilities(raw string) ModelCapabilities {
	var result ModelCapabilities
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &result); err != nil {
		return ModelCapabilities{}
	}
	return result
}

func encodeProviderOptions(input map[string]any) string {
	if len(input) == 0 {
		return "{}"
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func decodeProviderOptions(raw string) map[string]any {
	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &result); err != nil || result == nil {
		return map[string]any{}
	}
	return result
}

func sanitizeHTTPError(err error) error {
	if err == nil {
		return nil
	}
	return errors.New(sanitizeErrorMessage(err.Error()))
}

func sanitizeHTTPBody(body []byte, secrets ...string) string {
	value := sanitizeErrorMessage(string(body), secrets...)
	if len(value) > 400 {
		return value[:400] + "..."
	}
	return value
}

func sanitizedBodyPreview(body []byte, secrets ...string) string {
	return logx.PreviewText(sanitizeErrorMessage(string(body), secrets...), 2000)
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
		modelID := strings.TrimSpace(models[index].ID)
		if modelID == "" {
			continue
		}
		result = append(result, modelID)
	}
	return result
}

func sanitizeErrorMessage(message string, secrets ...string) string {
	sanitized := bearerTokenPattern.ReplaceAllString(message, "${1}<redacted>")
	for _, marker := range []string{"Authorization", "authorization", "x-api-key", "api-key"} {
		sanitized = strings.ReplaceAll(sanitized, marker, "<redacted-header>")
	}
	for _, secret := range secrets {
		trimmed := strings.TrimSpace(secret)
		if trimmed == "" {
			continue
		}
		sanitized = strings.ReplaceAll(sanitized, trimmed, "<redacted>")
	}
	return strings.TrimSpace(sanitized)
}
