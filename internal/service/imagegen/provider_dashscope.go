package imagegen

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

const dashScopeGenerationPath = "/api/v1/services/aigc/multimodal-generation/generation"

type dashScopeContent struct {
	Text  string `json:"text,omitempty"`
	Image string `json:"image,omitempty"`
}

type dashScopeMessage struct {
	Role    string             `json:"role"`
	Content []dashScopeContent `json:"content"`
}

type dashScopeRequest struct {
	Model      string         `json:"model"`
	Input      dashScopeInput `json:"input"`
	Parameters map[string]any `json:"parameters,omitempty"`
}

type dashScopeInput struct {
	Messages []dashScopeMessage `json:"messages"`
}

type dashScopeResponse struct {
	RequestID string `json:"request_id"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Output    struct {
		Finished bool `json:"finished"`
		Choices  []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Role    string `json:"role"`
				Content []struct {
					Type  string `json:"type"`
					Image string `json:"image"`
					Text  string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	} `json:"output"`
}

func (s *Service) callDashScopeGenerateProvider(
	ctx context.Context,
	config *providercfg.ImageConfig,
	input GenerateInput,
) ([]byte, string, string, error) {
	endpoint, err := dashScopeEndpointURL(config.BaseURL)
	if err != nil {
		return nil, "", "", err
	}
	request := dashScopeRequest{
		Model: config.Model,
		Input: dashScopeInput{
			Messages: []dashScopeMessage{{
				Role:    "user",
				Content: []dashScopeContent{{Text: input.Prompt}},
			}},
		},
		Parameters: dashScopeParameters(config.ProviderOptions, input.Size),
	}
	var response dashScopeResponse
	if err := s.postJSONWithRetries(ctx, endpoint, config.AuthToken, request, &response); err != nil {
		return nil, "", "", err
	}
	return s.extractDashScopeImage(ctx, response, input.OutputFormat)
}

func (s *Service) callDashScopeEditProvider(
	ctx context.Context,
	config *providercfg.ImageConfig,
	input EditInput,
) ([]byte, string, string, error) {
	endpoint, err := dashScopeEndpointURL(config.BaseURL)
	if err != nil {
		return nil, "", "", err
	}
	imageReference, err := dashScopeWorkspaceImage(input.WorkspacePath, input.ImagePath)
	if err != nil {
		return nil, "", "", err
	}
	content := []dashScopeContent{{Image: imageReference}}
	if strings.TrimSpace(input.MaskPath) != "" {
		maskReference, maskErr := dashScopeWorkspaceImage(input.WorkspacePath, input.MaskPath)
		if maskErr != nil {
			return nil, "", "", maskErr
		}
		content = append(content, dashScopeContent{Image: maskReference})
	}
	content = append(content, dashScopeContent{Text: input.Prompt})

	request := dashScopeRequest{
		Model: config.Model,
		Input: dashScopeInput{
			Messages: []dashScopeMessage{{
				Role:    "user",
				Content: content,
			}},
		},
		Parameters: dashScopeParameters(config.ProviderOptions, input.Size),
	}
	var response dashScopeResponse
	if err := s.postJSONWithRetries(ctx, endpoint, config.AuthToken, request, &response); err != nil {
		return nil, "", "", err
	}
	return s.extractDashScopeImage(ctx, response, input.OutputFormat)
}

func dashScopeEndpointURL(baseURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("base_url 格式不正确")
	}
	if err := validateProviderURL(parsed); err != nil {
		return "", err
	}
	if strings.HasSuffix(parsed.Path, "/generation") {
		return parsed.String(), nil
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + dashScopeGenerationPath
	return parsed.String(), nil
}

func dashScopeParameters(providerOptions map[string]any, size string) map[string]any {
	parameters := map[string]any{}
	for key, value := range dashScopeParameterSource(providerOptions) {
		if strings.TrimSpace(key) != "" {
			parameters[key] = value
		}
	}
	parameters["size"] = dashScopeSize(size)
	parameters["n"] = 1
	if _, exists := parameters["watermark"]; !exists {
		parameters["watermark"] = false
	}
	return parameters
}

func dashScopeParameterSource(providerOptions map[string]any) map[string]any {
	if len(providerOptions) == 0 {
		return nil
	}
	if nested, ok := providerOptions["parameters"].(map[string]any); ok {
		return nested
	}
	return providerOptions
}

func dashScopeSize(size string) string {
	normalized := strings.TrimSpace(size)
	if normalized == "" {
		return "1K"
	}
	switch strings.ToUpper(normalized) {
	case "1K", "2K", "4K":
		return strings.ToUpper(normalized)
	default:
		return strings.ReplaceAll(strings.ToLower(normalized), "x", "*")
	}
}

func dashScopeWorkspaceImage(workspacePath string, imagePath string) (string, error) {
	fullPath, err := resolveWorkspaceFile(workspacePath, imagePath)
	if err != nil {
		return "", err
	}
	payload, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	if len(payload) == 0 {
		return "", errors.New("图片文件为空")
	}
	if len(payload) > maxImageBytes {
		return "", errors.New("图片超过大小限制")
	}
	mimeType := detectMIMEType(payload, strings.TrimPrefix(strings.ToLower(filepath.Ext(fullPath)), "."))
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(payload)), nil
}

func (s *Service) extractDashScopeImage(
	ctx context.Context,
	response dashScopeResponse,
	outputFormat string,
) ([]byte, string, string, error) {
	if strings.TrimSpace(response.Code) != "" {
		return nil, "", "", fmt.Errorf("DashScope 图片接口返回 %s: %s", response.Code, response.Message)
	}
	for _, choice := range response.Output.Choices {
		revisedPrompt := make([]string, 0)
		for _, content := range choice.Message.Content {
			if strings.TrimSpace(content.Text) != "" {
				revisedPrompt = append(revisedPrompt, strings.TrimSpace(content.Text))
			}
			if strings.TrimSpace(content.Image) == "" {
				continue
			}
			payload, mimeType, err := s.resolveDashScopeImageReference(ctx, content.Image, outputFormat)
			if err != nil {
				return nil, "", "", err
			}
			return payload, strings.Join(revisedPrompt, "\n"), mimeType, nil
		}
	}
	if !response.Output.Finished {
		return nil, "", "", errors.New("DashScope 图片任务未完成")
	}
	return nil, "", "", errors.New("DashScope 图片接口响应缺少 output.choices[].message.content[].image")
}

func (s *Service) resolveDashScopeImageReference(
	ctx context.Context,
	reference string,
	outputFormat string,
) ([]byte, string, error) {
	trimmed := strings.TrimSpace(reference)
	if strings.HasPrefix(trimmed, "data:") {
		payload, mimeType, err := decodeDataImage(trimmed)
		return payload, mimeType, err
	}
	payload, err := s.downloadImage(ctx, trimmed)
	if err != nil {
		return nil, "", err
	}
	return payload, detectMIMEType(payload, outputFormat), nil
}

func decodeDataImage(value string) ([]byte, string, error) {
	header, encoded, ok := strings.Cut(value, ",")
	if !ok || !strings.Contains(header, ";base64") {
		return nil, "", errors.New("图片 data URL 格式不正确")
	}
	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, "", fmt.Errorf("解析图片 data URL 失败: %w", err)
	}
	mimeType := strings.TrimPrefix(strings.TrimSuffix(header, ";base64"), "data:")
	if mimeType == "" {
		mimeType = detectMIMEType(payload, "")
	}
	return payload, mimeType, nil
}
