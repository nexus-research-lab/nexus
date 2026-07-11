package provider

import (
	"encoding/json"
	"errors"
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

func shouldUseSeedreamDefaults(modelID string) bool {
	model := strings.ToLower(strings.TrimSpace(modelID))
	return strings.Contains(model, "seedream")
}
