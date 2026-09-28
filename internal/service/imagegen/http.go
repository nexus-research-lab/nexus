// INPUT: Image requests, workspace file references or synthetic probe bytes.
// OUTPUT: Bounded HTTP responses and typed failures; probe submissions do not retry.
// POS: Production image transport shared by normal requests and verification.
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
	files map[string]multipartFileRef,
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
		for name, fileRef := range files {
			if err := s.appendMultipartFile(ctx, writer, name, fileRef); err != nil {
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

type multipartFileRef struct {
	Data          []byte
	WorkspacePath string
	RelativePath  string
}

func (s *Service) appendMultipartFile(
	ctx context.Context,
	writer *multipart.Writer,
	name string,
	fileRef multipartFileRef,
) error {
	if fileRef.Data != nil {
		part, err := writer.CreateFormFile(name, "capability-probe.png")
		if err != nil {
			return err
		}
		_, err = part.Write(fileRef.Data)
		return err
	}
	root, err := s.openWorkspace(ctx, fileRef.WorkspacePath, false)
	if err != nil {
		return err
	}
	defer root.Close()
	file, err := root.OpenFileNoSymlink(fileRef.RelativePath, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	part, err := writer.CreateFormFile(name, filepath.Base(fileRef.RelativePath))
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
	if s.singleAttempt {
		return run()
	}
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
			err:       newImageResponseError(response.StatusCode, payload, message),
			retryable: response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= http.StatusInternalServerError,
		}
	}
	if err := json.Unmarshal(payload, output); err != nil {
		return fmt.Errorf("解析图片接口响应失败: %w", err)
	}
	return nil
}

// imageResponseError 保留协议状态和机器错误码，能力探测不按错误正文猜测不支持。
type imageResponseError struct {
	status        int
	code, message string
}

func (e *imageResponseError) Error() string {
	return fmt.Sprintf("图片接口返回 %d: %s", e.status, e.message)
}
func newImageResponseError(status int, payload []byte, message string) error {
	var envelope struct {
		Code  string `json:"code"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(payload, &envelope)
	code := envelope.Error.Code
	if code == "" {
		code = envelope.Code
	}
	return &imageResponseError{status: status, code: code, message: message}
}
