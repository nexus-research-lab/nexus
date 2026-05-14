package imagegen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

const (
	defaultSize         = "1024x1024"
	defaultOutputFormat = "png"
	maxImageBytes       = 25 * 1024 * 1024
	requestTimeout      = 120 * time.Second
	defaultMaxAttempts  = 3
)

var safeFileNamePattern = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// ProviderResolver 是图片生成服务依赖的 provider 配置解析子集。
type ProviderResolver interface {
	ResolveImageConfig(ctx context.Context, provider string) (*providercfg.ImageConfig, error)
}

// Service 提供 Provider 驱动的图片生成能力。
type Service struct {
	providers ProviderResolver
	now       func() time.Time
	client    *http.Client
}

// NewService 创建图片生成服务。
func NewService(providers ProviderResolver) *Service {
	return &Service{
		providers: providers,
		now:       func() time.Time { return time.Now().UTC() },
		client:    &http.Client{Timeout: requestTimeout},
	}
}

// GenerateImage 调用图片生成 Provider 并保存图片。
func (s *Service) GenerateImage(ctx context.Context, input GenerateInput) (*Result, []byte, error) {
	if s == nil || s.providers == nil {
		return nil, nil, errors.New("图片生成服务未初始化")
	}
	normalized, err := normalizeInput(input)
	if err != nil {
		return nil, nil, err
	}
	config, err := s.providers.ResolveImageConfig(ctx, normalized.Provider)
	if err != nil {
		return nil, nil, err
	}
	payload, revisedPrompt, mimeType, err := s.callGenerateProvider(ctx, config, normalized)
	if err != nil {
		return nil, nil, err
	}
	if len(payload) == 0 {
		return nil, nil, errors.New("图片生成接口未返回图片数据")
	}
	if len(payload) > maxImageBytes {
		return nil, nil, fmt.Errorf("图片过大: %d bytes", len(payload))
	}
	if mimeType == "" {
		mimeType = detectMIMEType(payload, normalized.OutputFormat)
	}
	relativePath, err := s.writeImage(normalized, payload, mimeType)
	if err != nil {
		return nil, nil, err
	}
	result := &Result{
		Provider:      config.Provider,
		Model:         config.Model,
		Path:          relativePath,
		MIMEType:      mimeType,
		Size:          normalized.Size,
		RevisedPrompt: revisedPrompt,
		Markdown:      fmt.Sprintf("![generated image](%s)", relativePath),
	}
	return result, payload, nil
}

// EditImage 调用图片编辑 Provider 并保存图片。
func (s *Service) EditImage(ctx context.Context, input EditInput) (*Result, []byte, error) {
	if s == nil || s.providers == nil {
		return nil, nil, errors.New("图片生成服务未初始化")
	}
	normalized, err := normalizeEditInput(input)
	if err != nil {
		return nil, nil, err
	}
	config, err := s.providers.ResolveImageConfig(ctx, normalized.Provider)
	if err != nil {
		return nil, nil, err
	}
	payload, revisedPrompt, mimeType, err := s.callEditProvider(ctx, config, normalized)
	if err != nil {
		return nil, nil, err
	}
	if len(payload) == 0 {
		return nil, nil, errors.New("图片编辑接口未返回图片数据")
	}
	if len(payload) > maxImageBytes {
		return nil, nil, fmt.Errorf("图片过大: %d bytes", len(payload))
	}
	if mimeType == "" {
		mimeType = detectMIMEType(payload, normalized.OutputFormat)
	}
	generateInput := GenerateInput{
		Prompt:        normalized.Prompt,
		WorkspacePath: normalized.WorkspacePath,
		OutputFormat:  normalized.OutputFormat,
		FileName:      normalized.FileName,
	}
	relativePath, err := s.writeImage(generateInput, payload, mimeType)
	if err != nil {
		return nil, nil, err
	}
	result := &Result{
		Provider:      config.Provider,
		Model:         config.Model,
		Path:          relativePath,
		MIMEType:      mimeType,
		Size:          normalized.Size,
		RevisedPrompt: revisedPrompt,
		Markdown:      fmt.Sprintf("![edited image](%s)", relativePath),
	}
	return result, payload, nil
}

func (s *Service) callGenerateProvider(
	ctx context.Context,
	config *providercfg.ImageConfig,
	input GenerateInput,
) ([]byte, string, string, error) {
	endpoint, err := endpointURL(config.BaseURL, "generations")
	if err != nil {
		return nil, "", "", err
	}
	fields := map[string]any{
		"prompt": input.Prompt,
		"n":      1,
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
	if input.OutputFormat != "" {
		fields["output_format"] = input.OutputFormat
	}
	if input.OutputCompression != nil {
		fields["output_compression"] = *input.OutputCompression
	}
	if input.Background != "" {
		fields["background"] = input.Background
	}

	var response imageResponse
	if err := s.postJSONWithRetries(ctx, endpoint, config.AuthToken, fields, &response); err != nil {
		return nil, "", "", err
	}
	return s.extractImage(ctx, response, input.OutputFormat)
}

func (s *Service) callEditProvider(
	ctx context.Context,
	config *providercfg.ImageConfig,
	input EditInput,
) ([]byte, string, string, error) {
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

func (s *Service) postJSONWithRetries(ctx context.Context, endpoint string, token string, payload any, output any) error {
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
		return s.readJSONResponse(request, output)
	})
}

func (s *Service) postMultipartWithRetries(
	ctx context.Context,
	endpoint string,
	token string,
	fields map[string]string,
	files map[string]string,
	output any,
) error {
	return s.doWithRetries(func() error {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		for name, value := range fields {
			if err := writer.WriteField(name, value); err != nil {
				return err
			}
		}
		for name, path := range files {
			if err := appendMultipartFile(writer, name, path); err != nil {
				return err
			}
		}
		if err := writer.Close(); err != nil {
			return err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", writer.FormDataContentType())
		return s.readJSONResponse(request, output)
	})
}

func appendMultipartFile(writer *multipart.Writer, name string, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	part, err := writer.CreateFormFile(name, filepath.Base(path))
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	return err
}

type retryableError struct {
	err       error
	retryable bool
}

func (e retryableError) Error() string {
	return e.err.Error()
}

func (e retryableError) Unwrap() error {
	return e.err
}

func (s *Service) doWithRetries(run func() error) error {
	var lastErr error
	for attempt := 1; attempt <= defaultMaxAttempts; attempt++ {
		err := run()
		if err == nil {
			return nil
		}
		lastErr = err
		var retryable retryableError
		if !errors.As(err, &retryable) || !retryable.retryable || attempt == defaultMaxAttempts {
			return err
		}
		time.Sleep(time.Duration(1<<attempt) * time.Second)
	}
	return lastErr
}

func (s *Service) readJSONResponse(request *http.Request, output any) error {
	response, err := s.client.Do(request)
	if err != nil {
		return retryableError{err: fmt.Errorf("图片接口请求失败: %w", err), retryable: true}
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, maxImageBytes+1)
	payload, err := io.ReadAll(limited)
	if err != nil {
		return retryableError{err: fmt.Errorf("读取图片接口响应失败: %w", err), retryable: true}
	}
	if len(payload) > maxImageBytes {
		return errors.New("图片接口响应超过大小限制")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message := strings.TrimSpace(string(payload))
		return retryableError{
			err:       fmt.Errorf("图片接口返回 %d: %s", response.StatusCode, message),
			retryable: response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= http.StatusInternalServerError,
		}
	}
	if err := json.Unmarshal(payload, output); err != nil {
		return fmt.Errorf("解析图片接口响应失败: %w", err)
	}
	return nil
}

func (s *Service) extractImage(ctx context.Context, response imageResponse, outputFormat string) ([]byte, string, string, error) {
	if len(response.Data) == 0 {
		return nil, "", "", errors.New("图片接口响应缺少 data")
	}
	item := response.Data[0]
	var payload []byte
	if strings.TrimSpace(item.B64JSON) != "" {
		decoded, err := base64.StdEncoding.DecodeString(item.B64JSON)
		if err != nil {
			return nil, "", "", fmt.Errorf("解析图片 base64 失败: %w", err)
		}
		payload = decoded
	} else if strings.TrimSpace(item.URL) != "" {
		downloaded, err := s.downloadImage(ctx, item.URL)
		if err != nil {
			return nil, "", "", err
		}
		payload = downloaded
	} else {
		return nil, "", "", errors.New("图片接口响应缺少 b64_json 或 url")
	}
	if len(payload) > maxImageBytes {
		return nil, "", "", errors.New("图片超过大小限制")
	}
	return payload, strings.TrimSpace(item.RevisedPrompt), detectMIMEType(payload, outputFormat), nil
}

func (s *Service) downloadImage(ctx context.Context, rawURL string) ([]byte, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("图片 URL 格式不正确")
	}
	if err := validateProviderURL(parsed); err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("下载图片失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("下载图片返回 %d", response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > maxImageBytes {
		return nil, errors.New("图片超过大小限制")
	}
	return payload, nil
}

func (s *Service) writeImage(input GenerateInput, payload []byte, mimeType string) (string, error) {
	ext := extensionFor(mimeType, input.OutputFormat)
	name := strings.TrimSpace(input.FileName)
	if name == "" {
		name = fmt.Sprintf("%s-%s", s.now().Format("20060102-150405"), promptSlug(input.Prompt))
	}
	name = strings.TrimSuffix(sanitizeFileName(name), filepath.Ext(name))
	if name == "" {
		name = "generated-image"
	}
	relativePath := filepath.ToSlash(filepath.Join("output", "imagegen", name+ext))
	fullPath := filepath.Join(input.WorkspacePath, relativePath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(fullPath, payload, 0o644); err != nil {
		return "", err
	}
	return relativePath, nil
}

func normalizeInput(input GenerateInput) (GenerateInput, error) {
	input.Provider = strings.TrimSpace(input.Provider)
	input.Prompt = strings.TrimSpace(input.Prompt)
	input.WorkspacePath = strings.TrimSpace(input.WorkspacePath)
	input.Size = strings.TrimSpace(input.Size)
	input.Quality = strings.TrimSpace(input.Quality)
	input.Background = strings.TrimSpace(input.Background)
	input.OutputFormat = strings.ToLower(strings.TrimSpace(input.OutputFormat))
	input.FileName = strings.TrimSpace(input.FileName)
	if input.OutputCompression != nil {
		if *input.OutputCompression < 0 || *input.OutputCompression > 100 {
			return GenerateInput{}, errors.New("output_compression 必须在 0 到 100 之间")
		}
	}
	if input.Prompt == "" {
		return GenerateInput{}, errors.New("prompt 不能为空")
	}
	if input.WorkspacePath == "" {
		return GenerateInput{}, errors.New("workspace_path 不能为空")
	}
	if input.Size == "" {
		input.Size = defaultSize
	}
	if input.OutputFormat == "" {
		input.OutputFormat = defaultOutputFormat
	}
	switch input.OutputFormat {
	case "png", "jpeg", "jpg", "webp":
	default:
		return GenerateInput{}, errors.New("output_format 只支持 png、jpeg、jpg、webp")
	}
	if input.OutputFormat == "jpg" {
		input.OutputFormat = "jpeg"
	}
	return input, nil
}

func normalizeEditInput(input EditInput) (EditInput, error) {
	input.Provider = strings.TrimSpace(input.Provider)
	input.Prompt = strings.TrimSpace(input.Prompt)
	input.WorkspacePath = strings.TrimSpace(input.WorkspacePath)
	input.ImagePath = strings.TrimSpace(input.ImagePath)
	input.MaskPath = strings.TrimSpace(input.MaskPath)
	input.Size = strings.TrimSpace(input.Size)
	input.Quality = strings.TrimSpace(input.Quality)
	input.OutputFormat = strings.ToLower(strings.TrimSpace(input.OutputFormat))
	input.FileName = strings.TrimSpace(input.FileName)
	if input.Prompt == "" {
		return EditInput{}, errors.New("prompt 不能为空")
	}
	if input.WorkspacePath == "" {
		return EditInput{}, errors.New("workspace_path 不能为空")
	}
	if input.ImagePath == "" {
		return EditInput{}, errors.New("image_path 不能为空")
	}
	if input.OutputFormat == "" {
		input.OutputFormat = defaultOutputFormat
	}
	switch input.OutputFormat {
	case "png", "jpeg", "jpg", "webp":
	default:
		return EditInput{}, errors.New("output_format 只支持 png、jpeg、jpg、webp")
	}
	if input.OutputFormat == "jpg" {
		input.OutputFormat = "jpeg"
	}
	if input.OutputCompression != nil {
		if *input.OutputCompression < 0 || *input.OutputCompression > 100 {
			return EditInput{}, errors.New("output_compression 必须在 0 到 100 之间")
		}
	}
	return input, nil
}

func resolveWorkspaceFile(workspacePath string, relativePath string) (string, error) {
	cleanWorkspace := filepath.Clean(strings.TrimSpace(workspacePath))
	cleanRelative := filepath.Clean(strings.TrimSpace(relativePath))
	if cleanRelative == "." || cleanRelative == "" {
		return "", errors.New("图片路径不能为空")
	}
	if filepath.IsAbs(cleanRelative) || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) || cleanRelative == ".." {
		return "", errors.New("图片路径必须在当前 workspace 内")
	}
	fullPath := filepath.Join(cleanWorkspace, cleanRelative)
	rel, err := filepath.Rel(cleanWorkspace, fullPath)
	if err != nil || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", errors.New("图片路径必须在当前 workspace 内")
	}
	return fullPath, nil
}

func detectMIMEType(payload []byte, outputFormat string) string {
	if len(payload) > 0 {
		detected := http.DetectContentType(payload)
		if strings.HasPrefix(detected, "image/") {
			return detected
		}
	}
	switch outputFormat {
	case "jpeg", "jpg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	default:
		return "image/png"
	}
}

func extensionFor(mimeType string, outputFormat string) string {
	if exts, err := mime.ExtensionsByType(strings.TrimSpace(mimeType)); err == nil && len(exts) > 0 {
		return exts[0]
	}
	switch outputFormat {
	case "jpeg", "jpg":
		return ".jpg"
	case "webp":
		return ".webp"
	default:
		return ".png"
	}
}

func promptSlug(prompt string) string {
	words := strings.Fields(strings.ToLower(prompt))
	if len(words) == 0 {
		return "image"
	}
	joined := strings.Join(words, "-")
	if len(joined) > 40 {
		joined = joined[:40]
	}
	return sanitizeFileName(joined)
}

func sanitizeFileName(name string) string {
	cleaned := safeFileNamePattern.ReplaceAllString(strings.TrimSpace(name), "-")
	cleaned = strings.Trim(cleaned, ".-_")
	if len(cleaned) > 80 {
		cleaned = cleaned[:80]
	}
	return cleaned
}
