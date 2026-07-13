package imagegen

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"

	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

func (s *Service) callGenerateProvider(
	ctx context.Context,
	config *providercfg.ImageConfig,
	input GenerateInput,
) ([]byte, string, string, error) {
	if config != nil {
		switch strings.TrimSpace(config.APIFormat) {
		case providercfg.APIFormatDashScopeImageGeneration:
			return s.callDashScopeGenerateProvider(ctx, config, input)
		case providercfg.APIFormatModelScopeImageGeneration:
			return s.callModelScopeGenerateProvider(ctx, config, input)
		}
	}
	endpoint, err := endpointURL(config.BaseURL, "generations")
	if err != nil {
		return nil, "", "", err
	}
	fields := openAICompatibleGeneratePayload(config, input, endpoint)
	if isSeedreamModel(config) {
		if _, exists := fields["watermark"]; !exists {
			fields["watermark"] = false
		}
	}

	var response imageResponse
	if err := s.postJSONWithRetries(ctx, endpoint, config.AuthToken, fields, &response); err != nil {
		return nil, "", "", err
	}
	return s.extractImage(ctx, response, input.OutputFormat)
}

func openAICompatibleGeneratePayload(
	config *providercfg.ImageConfig,
	input GenerateInput,
	endpoint string,
) map[string]any {
	var providerOptions map[string]any
	if config != nil {
		providerOptions = config.ProviderOptions
	}
	fields := cloneProviderOptions(providerOptions)
	fields["prompt"] = input.Prompt
	fields["n"] = 1
	if config != nil && !isAzureDeployment(endpoint) {
		fields["model"] = config.Model
	}
	if input.Size != "" {
		fields["size"] = input.Size
	}
	if input.Quality != "" {
		fields["quality"] = input.Quality
	}
	if input.OutputFormat != "" {
		fields["output_format"] = input.OutputFormat
	}
	if input.OutputCompression != nil {
		fields["output_compression"] = *input.OutputCompression
	}
	if input.Background != "" {
		fields["background"] = input.Background
	}
	return fields
}

func (s *Service) callEditProvider(
	ctx context.Context,
	config *providercfg.ImageConfig,
	input EditInput,
) ([]byte, string, string, error) {
	if config != nil {
		switch strings.TrimSpace(config.APIFormat) {
		case providercfg.APIFormatDashScopeImageGeneration:
			return s.callDashScopeEditProvider(ctx, config, input)
		case providercfg.APIFormatModelScopeImageGeneration:
			return nil, "", "", errors.New("ModelScope 图片生成分支暂不支持 edit 操作")
		}
	}
	endpoint, err := endpointURL(config.BaseURL, "edits")
	if err != nil {
		return nil, "", "", err
	}
	imagePath, err := resolveWorkspaceFile(input.WorkspacePath, input.ImagePath)
	if err != nil {
		return nil, "", "", err
	}
	fields := map[string]string{
		"prompt":        input.Prompt,
		"n":             "1",
		"output_format": input.OutputFormat,
	}
	if !isAzureDeployment(endpoint) {
		fields["model"] = config.Model
	}
	if input.Size != "" {
		fields["size"] = input.Size
	}
	if input.Quality != "" {
		fields["quality"] = input.Quality
	}
	if input.OutputCompression != nil {
		fields["output_compression"] = strconv.Itoa(*input.OutputCompression)
	}
	files := map[string]string{"image": imagePath}
	if input.MaskPath != "" {
		maskPath, pathErr := resolveWorkspaceFile(input.WorkspacePath, input.MaskPath)
		if pathErr != nil {
			return nil, "", "", pathErr
		}
		files["mask"] = maskPath
	}

	var response imageResponse
	if err := s.postMultipartWithRetries(ctx, endpoint, config.AuthToken, fields, files, &response); err != nil {
		return nil, "", "", err
	}
	return s.extractImage(ctx, response, input.OutputFormat)
}

type imageResponse struct {
	Data []struct {
		B64JSON       string `json:"b64_json"`
		URL           string `json:"url"`
		RevisedPrompt string `json:"revised_prompt"`
	} `json:"data"`
}

func endpointURL(baseURL string, operation string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("base_url 格式不正确")
	}
	if err := validateProviderURL(parsed); err != nil {
		return "", err
	}
	targetSuffix := "/images/" + operation
	path := parsed.Path
	if strings.HasSuffix(path, targetSuffix) {
		return parsed.String(), nil
	}
	for _, existing := range []string{"/images/generations", "/images/edits"} {
		if strings.HasSuffix(path, existing) {
			parsed.Path = strings.TrimSuffix(path, existing) + targetSuffix
			return parsed.String(), nil
		}
	}
	parsed.Path = strings.TrimRight(path, "/") + targetSuffix
	return parsed.String(), nil
}

func isAzureDeployment(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(parsed.Path), "/openai/deployments/")
}

func validateProviderURL(parsed *url.URL) error {
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme == "http" {
		host := strings.ToLower(parsed.Hostname())
		if host == "localhost" || host == "127.0.0.1" || host == "::1" {
			return nil
		}
	}
	return errors.New("图片生成 Provider 只允许 https 或 localhost 调试地址")
}
