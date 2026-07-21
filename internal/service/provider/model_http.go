package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

const (
	providerEndpointModels            = "models"
	providerEndpointChatCompletions   = APIFormatChatCompletions
	providerEndpointResponses         = APIFormatResponses
	providerEndpointAnthropicMessages = APIFormatAnthropicMessages
	providerTestResponsesMaxTokens    = 16
	// 新版 Azure 模型可能在产生响应前拒绝只有一个 token 的探针。
	azureModelCheckMaxCompletionTokens = 64
)

func endpointURL(item providerstore.Entity, endpointKey string) string {
	if endpointKey == providerEndpointModels {
		return joinEndpointURL(item.BaseURL, item.ModelsPath)
	}
	if item.ProviderKind == ProviderKindImageGeneration {
		switch normalizeAPIFormat(item.APIFormat) {
		case APIFormatDashScopeImageGeneration:
			return dashScopeEndpointURL(item.BaseURL)
		case APIFormatModelScopeImageGeneration:
			return modelScopeEndpointURL(item.BaseURL)
		}
		return joinEndpointURL(item.BaseURL, "/images/generations")
	}
	switch endpointKey {
	case providerEndpointResponses:
		if IsAzureOpenAIEndpoint(item.BaseURL) {
			return azureResponsesEndpointURL(item.BaseURL)
		}
		return joinEndpointURL(item.BaseURL, "/responses")
	case providerEndpointAnthropicMessages:
		return joinEndpointURL(item.BaseURL, "/v1/messages")
	default:
		return joinEndpointURL(item.BaseURL, "/chat/completions")
	}
}

// ResolveResponsesEndpoint 复用模型测试的 Azure/Foundry 校验和路径归一化规则。
func ResolveResponsesEndpoint(baseURL string) (string, error) {
	item := providerstore.Entity{
		APIFormat: APIFormatResponses,
		BaseURL:   baseURL,
	}
	if err := validateModelEndpoint(item); err != nil {
		return "", err
	}
	return endpointURL(item, providerEndpointResponses), nil
}

// azureResponsesEndpointURL 把 Azure 资源根或 project endpoint 归一化为 v1 Responses operation。
func azureResponsesEndpointURL(baseURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return joinEndpointURL(baseURL, "/responses")
	}
	basePath := strings.TrimRight(parsed.Path, "/")
	lowerPath := strings.ToLower(basePath)
	switch {
	case strings.HasSuffix(lowerPath, "/responses"):
		parsed.Path = basePath
	case strings.HasSuffix(lowerPath, "/openai/v1"):
		parsed.Path = basePath + "/responses"
	case strings.HasSuffix(lowerPath, "/openai"):
		parsed.Path = basePath + "/v1/responses"
	case basePath == "":
		parsed.Path = "/openai/v1/responses"
	case strings.Contains(strings.ToLower(parsed.Hostname()), ".services.ai.azure.com"):
		parsed.Path = basePath + "/openai/v1/responses"
	default:
		return joinEndpointURL(baseURL, "/responses")
	}
	parsed.RawPath = ""
	return parsed.String()
}

func joinEndpointURL(baseURL string, endpointPath string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	endpoint := strings.TrimSpace(endpointPath)
	if endpoint == "" {
		return base
	}
	endpointURL, endpointErr := url.Parse(endpoint)
	if endpointErr == nil && endpointURL.IsAbs() {
		return endpointURL.String()
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || endpointErr != nil {
		return base + "/" + strings.TrimLeft(endpoint, "/")
	}
	endpointURL.Path = "/" + strings.TrimLeft(endpointURL.Path, "/")
	basePath := strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(basePath, endpointURL.Path) {
		parsed.Path = basePath + endpointURL.Path
	}
	parsed.RawPath = ""
	if endpointURL.RawQuery != "" {
		parsed.RawQuery = endpointURL.RawQuery
	}
	return parsed.String()
}

func dashScopeEndpointURL(baseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return trimmed
	}
	if strings.HasSuffix(parsed.Path, "/generation") {
		return parsed.String()
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api/v1/services/aigc/multimodal-generation/generation"
	return parsed.String()
}

func modelScopeEndpointURL(baseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return trimmed
	}
	if strings.HasSuffix(parsed.Path, "/images/generations") {
		return parsed.String()
	}
	if strings.Trim(parsed.Path, "/") == "" {
		parsed.Path = "/v1/images/generations"
		return parsed.String()
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/images/generations"
	return parsed.String()
}

func applyProviderHeaders(request *http.Request, item providerstore.Entity) {
	token := strings.TrimSpace(item.AuthToken)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
		if IsAzureOpenAIEndpoint(item.BaseURL) {
			request.Header.Set("api-key", token)
		}
	}
	if normalizeAPIFormat(item.APIFormat) == APIFormatAnthropicMessages {
		if token != "" {
			request.Header.Set("x-api-key", token)
		}
		request.Header.Set("anthropic-version", "2023-06-01")
	}
	if request.Method == http.MethodPost && normalizeAPIFormat(item.APIFormat) == APIFormatModelScopeImageGeneration {
		request.Header.Set("X-ModelScope-Async-Mode", "true")
	}
}

// validateModelEndpoint 在发请求前拒绝已知协议与 endpoint 错配，避免把上游 404 当成模型故障。
func validateModelEndpoint(item providerstore.Entity) error {
	if normalizeAPIFormat(item.APIFormat) != APIFormatResponses || !IsAzureOpenAIEndpoint(item.BaseURL) {
		return nil
	}
	parsed, err := url.Parse(strings.TrimSpace(item.BaseURL))
	if err != nil {
		return fmt.Errorf("Azure Responses base_url 无效: %w", err)
	}
	path := strings.ToLower(strings.TrimRight(parsed.Path, "/"))
	if strings.Contains(path, "/deployments/") ||
		strings.HasSuffix(path, "/images/generations") ||
		strings.HasSuffix(path, "/chat/completions") {
		return errors.New("Azure Responses 不能使用 /deployments/...、/images/generations 或 /chat/completions operation URL；请填写 Azure 资源根、/openai 或 /openai/v1")
	}
	return nil
}

// IsAzureOpenAIEndpoint 识别 Azure OpenAI 与 Foundry 的兼容数据面主机。
func IsAzureOpenAIEndpoint(baseURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return strings.HasSuffix(host, ".openai.azure.com") ||
		strings.HasSuffix(host, ".cognitiveservices.azure.com") ||
		strings.HasSuffix(host, ".services.ai.azure.com")
}

func minimalPayload(item providerstore.Entity, modelID string) ([]byte, error) {
	modelID = normalizeModelID(modelID)
	if modelID == "" {
		return nil, errors.New("model 不能为空")
	}
	if item.ProviderKind == ProviderKindImageGeneration {
		switch normalizeAPIFormat(item.APIFormat) {
		case APIFormatDashScopeImageGeneration:
			return json.Marshal(map[string]any{
				"model": modelID,
				"input": map[string]any{
					"messages": []map[string]any{
						{
							"role": "user",
							"content": []map[string]string{
								{"text": "ping"},
							},
						},
					},
				},
				"parameters": map[string]any{
					"n":         1,
					"size":      "1K",
					"watermark": false,
				},
			})
		case APIFormatModelScopeImageGeneration:
			return json.Marshal(map[string]any{
				"model":  modelID,
				"prompt": "ping",
			})
		default:
			size := "1024x1024"
			usesSeedreamDefaults := shouldUseSeedreamDefaults(modelID)
			if usesSeedreamDefaults {
				size = "2K"
			}
			payload := map[string]any{
				"model":  modelID,
				"prompt": "ping",
				"n":      1,
				"size":   size,
			}
			if usesSeedreamDefaults {
				payload["watermark"] = false
			}
			return json.Marshal(payload)
		}
	}
	switch normalizeAPIFormat(item.APIFormat) {
	case APIFormatResponses:
		return json.Marshal(map[string]any{
			"model":             modelID,
			"input":             "ping",
			"max_output_tokens": providerTestResponsesMaxTokens,
			"store":             false,
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
		payload := map[string]any{
			"model":  modelID,
			"stream": false,
			"messages": []map[string]string{
				{"role": "user", "content": "ping"},
			},
		}
		if usesMaxCompletionTokens(item) {
			payload["max_completion_tokens"] = azureModelCheckMaxCompletionTokens
		} else {
			payload["max_tokens"] = 1
		}
		return json.Marshal(payload)
	}
}

// usesMaxCompletionTokens 识别 Azure Chat Completions，包括没有内置 preset 元数据的自定义配置。
func usesMaxCompletionTokens(item providerstore.Entity) bool {
	return strings.TrimSpace(item.PresetKey) == presetAzure || IsAzureOpenAIEndpoint(item.BaseURL)
}

func shouldUseSeedreamDefaults(modelID string) bool {
	model := strings.ToLower(strings.TrimSpace(modelID))
	return strings.Contains(model, "seedream")
}
