package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
)

// Client 提供后端轻量 LLM 调用能力，不依赖 Agent SDK。
type Client struct {
	HTTPClient *http.Client
}

// Message 表示一次轻量 LLM 请求中的单条消息。
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// GenerateTextRequest 描述一次非流式文本生成请求。
type GenerateTextRequest struct {
	Config      *clientopts.RuntimeConfig
	System      string
	Messages    []Message
	MaxTokens   int
	Temperature float64
}

// NewClient 创建轻量 LLM client。
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{HTTPClient: httpClient}
}

// GenerateText 按 Provider API format 发起非流式文本生成请求。
func (c *Client) GenerateText(ctx context.Context, request GenerateTextRequest) (string, error) {
	if request.Config == nil {
		return "", errors.New("llm config 不能为空")
	}
	endpoint, err := buildEndpoint(request.Config.BaseURL, request.Config.APIFormat)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(requestPayload(request))
	if err != nil {
		return "", err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	applyHeaders(httpRequest, request.Config)

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	response, err := httpClient.Do(httpRequest)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("llm api 返回异常状态: %d %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return parseTextResponse(request.Config.APIFormat, responseBody)
}

func requestPayload(request GenerateTextRequest) any {
	config := request.Config
	model := strings.TrimSpace(config.Model)
	messages := normalizeMessages(request.Messages)
	systemPrompt := strings.TrimSpace(request.System)
	maxTokens := request.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	switch normalizeAPIFormat(config.APIFormat) {
	case providersvc.APIFormatResponses:
		return responsesRequest{
			Model:           model,
			Input:           messagesWithSystem(systemPrompt, messages),
			MaxOutputTokens: maxTokens,
			Temperature:     request.Temperature,
			Stream:          false,
		}
	case providersvc.APIFormatChatCompletions:
		return chatCompletionsRequest{
			Model:       model,
			MaxTokens:   maxTokens,
			Temperature: request.Temperature,
			Stream:      false,
			Messages:    messagesWithSystem(systemPrompt, messages),
		}
	default:
		return anthropicMessagesRequest{
			Model:       model,
			MaxTokens:   maxTokens,
			Temperature: request.Temperature,
			System:      systemPrompt,
			Messages:    messages,
		}
	}
}

func normalizeMessages(messages []Message) []Message {
	result := make([]Message, 0, len(messages))
	for _, message := range messages {
		role := strings.TrimSpace(message.Role)
		content := strings.TrimSpace(message.Content)
		if role == "" || content == "" {
			continue
		}
		result = append(result, Message{Role: role, Content: content})
	}
	return result
}

func messagesWithSystem(systemPrompt string, messages []Message) []Message {
	result := make([]Message, 0, len(messages)+1)
	if systemPrompt != "" {
		result = append(result, Message{Role: "system", Content: systemPrompt})
	}
	result = append(result, messages...)
	return result
}

func applyHeaders(request *http.Request, config *clientopts.RuntimeConfig) {
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	token := strings.TrimSpace(config.AuthToken)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if normalizeAPIFormat(config.APIFormat) == providersvc.APIFormatAnthropicMessages {
		if token != "" {
			request.Header.Set("x-api-key", token)
		}
		request.Header.Set("anthropic-version", "2023-06-01")
	}
}

func buildEndpoint(baseURL string, apiFormat string) (string, error) {
	switch normalizeAPIFormat(apiFormat) {
	case providersvc.APIFormatResponses:
		return joinEndpoint(baseURL, "/responses")
	case providersvc.APIFormatChatCompletions:
		return joinEndpoint(baseURL, "/chat/completions")
	default:
		return buildMessagesEndpoint(baseURL)
	}
}

func joinEndpoint(baseURL string, endpointPath string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		return "", errors.New("provider base_url 不能为空")
	}
	path := "/" + strings.TrimLeft(strings.TrimSpace(endpointPath), "/")
	if strings.HasSuffix(trimmed, path) {
		return trimmed, nil
	}
	return trimmed + path, nil
}

func buildMessagesEndpoint(baseURL string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		return "", errors.New("provider base_url 不能为空")
	}
	switch {
	case strings.HasSuffix(trimmed, "/v1/messages"):
		return trimmed, nil
	case strings.HasSuffix(trimmed, "/v1"):
		return trimmed + "/messages", nil
	default:
		return trimmed + "/v1/messages", nil
	}
}

func normalizeAPIFormat(apiFormat string) string {
	switch strings.TrimSpace(apiFormat) {
	case providersvc.APIFormatChatCompletions:
		return providersvc.APIFormatChatCompletions
	case providersvc.APIFormatResponses:
		return providersvc.APIFormatResponses
	default:
		return providersvc.APIFormatAnthropicMessages
	}
}

type anthropicMessagesRequest struct {
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature,omitempty"`
	System      string    `json:"system,omitempty"`
	Messages    []Message `json:"messages"`
}

type chatCompletionsRequest struct {
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream"`
	Messages    []Message `json:"messages"`
}

type responsesRequest struct {
	Model           string    `json:"model"`
	Input           []Message `json:"input"`
	MaxOutputTokens int       `json:"max_output_tokens"`
	Temperature     float64   `json:"temperature,omitempty"`
	Stream          bool      `json:"stream"`
}
