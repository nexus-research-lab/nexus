package imagegen

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

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
