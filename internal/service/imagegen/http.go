package imagegen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
