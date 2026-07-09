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
	Config           *clientopts.RuntimeConfig
	System           string
	Messages         []Message
	MaxTokens        int
	Temperature      float64
	DisableReasoning bool
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
	text, err := parseTextResponse(request.Config.APIFormat, responseBody)
	if err != nil {
		return "", fmt.Errorf("llm api 解析响应失败: %w body=%s", err, trimResponseBody(responseBody))
	}
	return text, nil
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
		payload := responsesRequest{
			Model:           model,
			Input:           messagesWithSystem(systemPrompt, messages),
			MaxOutputTokens: maxTokens,
			Temperature:     request.Temperature,
			Stream:          false,
		}
		applyResponsesReasoningDisableOptions(&payload, config, request)
		return payload
	case providersvc.APIFormatChatCompletions:
		payload := chatCompletionsRequest{
			Model:       model,
			MaxTokens:   maxTokens,
			Temperature: request.Temperature,
			Stream:      false,
			Messages:    messagesWithSystem(systemPrompt, messages),
		}
		applyChatCompletionsReasoningDisableOptions(&payload, config, request)
		return payload
	default:
		payload := anthropicMessagesRequest{
			Model:       model,
			MaxTokens:   maxTokens,
			Temperature: request.Temperature,
			System:      systemPrompt,
			Messages:    messages,
		}
		applyAnthropicMessagesReasoningDisableOptions(&payload, config, request)
		return payload
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

func applyChatCompletionsReasoningDisableOptions(
	payload *chatCompletionsRequest,
	config *clientopts.RuntimeConfig,
	request GenerateTextRequest,
) {
	if !shouldDisableReasoning(config, request) {
		return
	}
	// chat_completions 协议：标准方式是 thinking.type=disabled（OpenAI-compatible）
	// 部分 provider 使用非标字段，先处理这些特例
	switch {
	case shouldUseEnableThinkingDisable(config):
		payload.EnableThinking = boolPointer(false)
	case shouldUseChatTemplateThinkingDisable(config):
		payload.ChatTemplateKwargs = map[string]bool{"enable_thinking": false}
	case shouldUseOpenAIReasoningEffortNone(config):
		payload.ReasoningEffort = "none"
	case isKimiAlwaysThinkingModel(config):
		// Kimi 始终推理模型，不支持关闭 thinking
		return
	default:
		// 协议通用 fallback：仅对明确启用推理的模型生效
		if config.Reasoning {
			payload.Thinking = map[string]string{"type": "disabled"}
		}
	}
}

func applyResponsesReasoningDisableOptions(
	payload *responsesRequest,
	config *clientopts.RuntimeConfig,
	request GenerateTextRequest,
) {
	if !shouldDisableReasoning(config, request) {
		return
	}
	// responses 协议：标准方式是 reasoning.effort=none（OpenAI Responses API）
	switch {
	case shouldUseEnableThinkingDisable(config):
		payload.EnableThinking = boolPointer(false)
	case shouldUseThinkingDisable(config):
		payload.Thinking = map[string]string{"type": "disabled"}
	case isKimiAlwaysThinkingModel(config):
		return
	default:
		if config.Reasoning {
			payload.Reasoning = &responsesReasoning{Effort: "none"}
		}
	}
}

func applyAnthropicMessagesReasoningDisableOptions(
	payload *anthropicMessagesRequest,
	config *clientopts.RuntimeConfig,
	request GenerateTextRequest,
) {
	if !shouldDisableReasoning(config, request) {
		return
	}
	// anthropic_messages 协议下按 provider 家族分派关闭方式：
	// 标准是 thinking.type=disabled；Qwen/DashScope 系即便走 anthropic 兼容端点仍用
	// 非标的 enable_thinking=false；Kimi 始终推理模型无法关闭，仅靠 max_tokens 兜底。
	switch {
	case shouldUseEnableThinkingDisable(config):
		payload.EnableThinking = boolPointer(false)
	case isKimiAlwaysThinkingModel(config):
		return
	case shouldUseThinkingDisable(config):
		payload.Thinking = map[string]string{"type": "disabled"}
	default:
		if config.Reasoning {
			payload.Thinking = map[string]string{"type": "disabled"}
		}
	}
}

func shouldDisableReasoning(config *clientopts.RuntimeConfig, request GenerateTextRequest) bool {
	return request.DisableReasoning && config != nil
}

func shouldUseThinkingDisable(config *clientopts.RuntimeConfig) bool {
	return isGLMRuntimeConfig(config) ||
		isVolcengineRuntimeConfig(config) ||
		isKimiRuntimeConfigWithDisableSupport(config)
}

func shouldUseEnableThinkingDisable(config *clientopts.RuntimeConfig) bool {
	return isDashScopeRuntimeConfig(config) ||
		isQwenTokenPlanRuntimeConfig(config) ||
		isModelScopeQwenRuntimeConfig(config)
}

func shouldUseChatTemplateThinkingDisable(config *clientopts.RuntimeConfig) bool {
	return isLocalOpenAICompatibleRuntimeConfig(config) && isQwenModel(config)
}

func shouldUseOpenAIReasoningEffortNone(config *clientopts.RuntimeConfig) bool {
	if !config.Reasoning || !isOpenAIRuntimeConfig(config) {
		return false
	}
	return openAIModelSupportsReasoningEffortNone(config.Model)
}

// isKimiAlwaysThinkingModel 判断是否为始终推理且无法关闭的 Kimi 模型。
func isKimiAlwaysThinkingModel(config *clientopts.RuntimeConfig) bool {
	if !runtimeContains(config, "kimi", "moonshot") {
		return false
	}
	return !isKimiRuntimeConfigWithDisableSupport(config)
}

func isGLMRuntimeConfig(config *clientopts.RuntimeConfig) bool {
	return runtimeContains(config, "bigmodel.cn") ||
		providerOrDisplayContains(config, "glm")
}

func isVolcengineRuntimeConfig(config *clientopts.RuntimeConfig) bool {
	return runtimeContains(config, "volcengine", "volces.com", "doubao", "火山", "方舟")
}

func isKimiRuntimeConfigWithDisableSupport(config *clientopts.RuntimeConfig) bool {
	if !runtimeContains(config, "kimi", "moonshot") {
		return false
	}
	model := normalizeMatchText(config.Model)
	return !strings.Contains(model, "kimi-for-coding") &&
		!strings.Contains(model, "kimi-k2.7-code")
}

func isDashScopeRuntimeConfig(config *clientopts.RuntimeConfig) bool {
	return runtimeContains(config, "dashscope", "bailian", "aliyun", "alibaba")
}

func isQwenTokenPlanRuntimeConfig(config *clientopts.RuntimeConfig) bool {
	return runtimeContains(config, "qwen-token-plan", "token-plan.cn") ||
		(providerOrDisplayContains(config, "qwen") && isQwenModel(config))
}

func isModelScopeQwenRuntimeConfig(config *clientopts.RuntimeConfig) bool {
	return runtimeContains(config, "modelscope") && isQwenModel(config)
}

func isLocalOpenAICompatibleRuntimeConfig(config *clientopts.RuntimeConfig) bool {
	return runtimeContains(
		config,
		"vllm",
		"sglang",
		"llama",
		"localhost",
		"127.0.0.1",
		"0.0.0.0",
		"[::1]",
		"://10.",
		"://192.168.",
		"://172.16.",
		"://172.17.",
		"://172.18.",
		"://172.19.",
		"://172.20.",
		"://172.21.",
		"://172.22.",
		"://172.23.",
		"://172.24.",
		"://172.25.",
		"://172.26.",
		"://172.27.",
		"://172.28.",
		"://172.29.",
		"://172.30.",
		"://172.31.",
	)
}

func isOpenAIRuntimeConfig(config *clientopts.RuntimeConfig) bool {
	provider := normalizeMatchText(config.Provider)
	displayName := normalizeMatchText(config.DisplayName)
	return provider == "openai" ||
		displayName == "openai" ||
		runtimeContains(config, "api.openai.com")
}

func isQwenModel(config *clientopts.RuntimeConfig) bool {
	model := normalizeMatchText(config.Model)
	return strings.Contains(model, "qwen") || strings.Contains(model, "qwq")
}

func openAIModelSupportsReasoningEffortNone(model string) bool {
	normalized := normalizeMatchText(model)
	if strings.Contains(normalized, "pro") {
		return false
	}
	return strings.HasPrefix(normalized, "gpt-5.1") ||
		strings.HasPrefix(normalized, "gpt-5.2") ||
		strings.HasPrefix(normalized, "gpt-5.3") ||
		strings.HasPrefix(normalized, "gpt-5.4") ||
		strings.HasPrefix(normalized, "gpt-5.5")
}

func runtimeContains(config *clientopts.RuntimeConfig, terms ...string) bool {
	for _, value := range []string{config.Provider, config.DisplayName, config.BaseURL} {
		normalized := normalizeMatchText(value)
		for _, term := range terms {
			if strings.Contains(normalized, normalizeMatchText(term)) {
				return true
			}
		}
	}
	return false
}

func providerOrDisplayContains(config *clientopts.RuntimeConfig, terms ...string) bool {
	for _, value := range []string{config.Provider, config.DisplayName} {
		normalized := normalizeMatchText(value)
		for _, term := range terms {
			if strings.Contains(normalized, normalizeMatchText(term)) {
				return true
			}
		}
	}
	return false
}

func normalizeMatchText(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func boolPointer(value bool) *bool {
	return &value
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

func trimResponseBody(body []byte) string {
	const limit = 1024
	text := strings.TrimSpace(string(body))
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "...(truncated)"
}

type anthropicMessagesRequest struct {
	Model          string            `json:"model"`
	MaxTokens      int               `json:"max_tokens"`
	Temperature    float64           `json:"temperature,omitempty"`
	System         string            `json:"system,omitempty"`
	Messages       []Message         `json:"messages"`
	Thinking       map[string]string `json:"thinking,omitempty"`
	EnableThinking *bool             `json:"enable_thinking,omitempty"`
}

type chatCompletionsRequest struct {
	Model              string            `json:"model"`
	MaxTokens          int               `json:"max_tokens"`
	Temperature        float64           `json:"temperature,omitempty"`
	Stream             bool              `json:"stream"`
	Messages           []Message         `json:"messages"`
	Thinking           map[string]string `json:"thinking,omitempty"`
	EnableThinking     *bool             `json:"enable_thinking,omitempty"`
	ChatTemplateKwargs map[string]bool   `json:"chat_template_kwargs,omitempty"`
	ReasoningEffort    string            `json:"reasoning_effort,omitempty"`
}

type responsesRequest struct {
	Model           string              `json:"model"`
	Input           []Message           `json:"input"`
	MaxOutputTokens int                 `json:"max_output_tokens"`
	Temperature     float64             `json:"temperature,omitempty"`
	Stream          bool                `json:"stream"`
	Thinking        map[string]string   `json:"thinking,omitempty"`
	EnableThinking  *bool               `json:"enable_thinking,omitempty"`
	Reasoning       *responsesReasoning `json:"reasoning,omitempty"`
}

type responsesReasoning struct {
	Effort string `json:"effort,omitempty"`
}
