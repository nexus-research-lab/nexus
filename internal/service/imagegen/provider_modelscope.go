package imagegen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

const (
	modelScopeGenerationPath = "/images/generations"
	modelScopeTasksPath      = "/tasks"
	modelScopeDefaultPrefix  = "/v1"
	modelScopePollInterval   = 5 * time.Second
	modelScopeMaxPolls       = 60
)

type modelScopeCreateResponse struct {
	TaskID  string `json:"task_id"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type modelScopeTaskResponse struct {
	TaskStatus   string   `json:"task_status"`
	OutputImages []string `json:"output_images"`
	Code         string   `json:"code"`
	Message      string   `json:"message"`
	Error        string   `json:"error"`
}

func (s *Service) callModelScopeGenerateProvider(
	ctx context.Context,
	config *providercfg.ImageConfig,
	input GenerateInput,
) ([]byte, string, string, error) {
	endpoint, err := modelScopeGenerationEndpointURL(config.BaseURL)
	if err != nil {
		return nil, "", "", err
	}
	request := modelScopePayload(config.ProviderOptions, config.Model, input)
	var response modelScopeCreateResponse
	if err := s.postModelScopeJSON(ctx, endpoint, config.AuthToken, request, &response); err != nil {
		return nil, "", "", err
	}
	if strings.TrimSpace(response.Code) != "" {
		return nil, "", "", fmt.Errorf("ModelScope 图片接口返回 %s: %s", response.Code, response.Message)
	}
	taskID := strings.TrimSpace(response.TaskID)
	if taskID == "" {
		return nil, "", "", errors.New("ModelScope 图片接口响应缺少 task_id")
	}
	return s.waitModelScopeImage(ctx, config, taskID, input.OutputFormat)
}

func modelScopePayload(providerOptions map[string]any, model string, input GenerateInput) map[string]any {
	payload := map[string]any{}
	for key, value := range providerOptions {
		if strings.TrimSpace(key) != "" {
			payload[key] = value
		}
	}
	if input.Size != "" && input.Size != defaultSize {
		payload["size"] = input.Size
	}
	payload["model"] = model
	payload["prompt"] = input.Prompt
	return payload
}

func (s *Service) waitModelScopeImage(
	ctx context.Context,
	config *providercfg.ImageConfig,
	taskID string,
	outputFormat string,
) ([]byte, string, string, error) {
	endpoint, err := modelScopeTaskEndpointURL(config.BaseURL, taskID)
	if err != nil {
		return nil, "", "", err
	}
	lastStatus := ""
	for attempt := 0; attempt < modelScopeMaxPolls; attempt++ {
		response, err := s.getModelScopeTask(ctx, endpoint, config.AuthToken)
		if err != nil {
			return nil, "", "", err
		}
		lastStatus = strings.TrimSpace(response.TaskStatus)
		switch strings.ToUpper(lastStatus) {
		case "SUCCEED":
			if len(response.OutputImages) == 0 || strings.TrimSpace(response.OutputImages[0]) == "" {
				return nil, "", "", errors.New("ModelScope 图片任务响应缺少 output_images[0]")
			}
			payload, mimeType, err := s.resolveModelScopeImageReference(ctx, response.OutputImages[0], outputFormat)
			return payload, "", mimeType, err
		case "FAILED":
			return nil, "", "", fmt.Errorf("ModelScope 图片任务失败: %s", response.failureMessage())
		}
		if attempt == modelScopeMaxPolls-1 {
			break
		}
		timer := time.NewTimer(modelScopePollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return nil, "", "", ctx.Err()
		case <-timer.C:
		}
	}
	return nil, "", "", fmt.Errorf("ModelScope 图片任务超时: task_id=%s status=%s", taskID, lastStatus)
}

func (response modelScopeTaskResponse) failureMessage() string {
	parts := make([]string, 0, 3)
	for _, value := range []string{response.Code, response.Message, response.Error} {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	if len(parts) == 0 {
		return "任务失败"
	}
	return strings.Join(parts, ": ")
}

func (s *Service) postModelScopeJSON(ctx context.Context, endpoint string, token string, payload any, output any) error {
	return s.doWithRetries(func() error {
		body, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-ModelScope-Async-Mode", "true")
		return s.readJSONResponse(request, output)
	})
}

func (s *Service) getModelScopeTask(ctx context.Context, endpoint string, token string) (modelScopeTaskResponse, error) {
	var output modelScopeTaskResponse
	err := s.doWithRetries(func() error {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("X-ModelScope-Task-Type", "image_generation")
		return s.readJSONResponse(request, &output)
	})
	return output, err
}

func modelScopeGenerationEndpointURL(baseURL string) (string, error) {
	parsed, err := parseModelScopeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	path := strings.TrimRight(parsed.Path, "/")
	if strings.HasSuffix(path, modelScopeGenerationPath) {
		return parsed.String(), nil
	}
	if strings.Trim(path, "/") == "" {
		parsed.Path = modelScopeDefaultPrefix + modelScopeGenerationPath
		return parsed.String(), nil
	}
	parsed.Path = path + modelScopeGenerationPath
	return parsed.String(), nil
}

func modelScopeTaskEndpointURL(baseURL string, taskID string) (string, error) {
	parsed, err := parseModelScopeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	path := strings.TrimRight(parsed.Path, "/")
	if strings.HasSuffix(path, modelScopeGenerationPath) {
		path = strings.TrimSuffix(path, modelScopeGenerationPath)
	}
	if strings.Trim(path, "/") == "" {
		path = modelScopeDefaultPrefix
	}
	parsed.Path = strings.TrimRight(path, "/") + modelScopeTasksPath + "/" + url.PathEscape(taskID)
	return parsed.String(), nil
}

func parseModelScopeBaseURL(baseURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("base_url 格式不正确")
	}
	if err := validateProviderURL(parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func (s *Service) resolveModelScopeImageReference(
	ctx context.Context,
	reference string,
	outputFormat string,
) ([]byte, string, error) {
	trimmed := strings.TrimSpace(reference)
	if strings.HasPrefix(trimmed, "data:") {
		return decodeDataImage(trimmed)
	}
	payload, err := s.downloadImage(ctx, trimmed)
	if err != nil {
		return nil, "", err
	}
	return payload, detectMIMEType(payload, outputFormat), nil
}
