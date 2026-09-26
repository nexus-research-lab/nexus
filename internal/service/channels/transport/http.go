package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func DoJSON(
	ctx context.Context,
	client *http.Client,
	method string,
	endpoint string,
	body any,
	headers map[string]string,
) (*http.Response, error) {
	if client == nil {
		client = DefaultHTTPClient
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	if body != nil && request.Header.Get("Content-Type") == "" {
		request.Header.Set("Content-Type", "application/json")
	}
	return client.Do(request)
}

func DoJSONExpectSuccess(
	ctx context.Context,
	client *http.Client,
	method string,
	endpoint string,
	body any,
	headers map[string]string,
) error {
	return DoJSONExpectSuccessDecode(ctx, client, method, endpoint, body, headers, nil)
}

func DoJSONExpectSuccessDecode(
	ctx context.Context,
	client *http.Client,
	method string,
	endpoint string,
	body any,
	headers map[string]string,
	output any,
) error {
	for attempt := 0; ; attempt++ {
		response, err := DoJSON(ctx, client, method, endpoint, body, headers)
		if err != nil {
			return err
		}
		err = ExpectSuccessDecode(response, output)
		var rejected *HTTPError
		// 只重试平台明确拒绝的限流请求，网络错误与 5xx 均保留未知结果。
		if !errors.As(err, &rejected) || rejected.StatusCode != http.StatusTooManyRequests ||
			attempt >= 2 || rejected.RetryAfter <= 0 || rejected.RetryAfter > 30*time.Second {
			return err
		}
		timer := time.NewTimer(rejected.RetryAfter)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(ctx.Err(), err)
		case <-timer.C:
		}
	}
}

func ExpectSuccess(response *http.Response) error {
	return ExpectSuccessDecode(response, nil)
}

func ExpectSuccessDecode(response *http.Response, output any) error {
	defer response.Body.Close()
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		if output == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			return nil
		}
		if err := json.NewDecoder(response.Body).Decode(output); err != nil && err != io.EOF {
			return err
		}
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return &HTTPError{StatusCode: response.StatusCode, Body: strings.TrimSpace(string(body)), RetryAfter: retryAfter(response.Header.Get("Retry-After"), body)}
}

// HTTPError 保留平台拒绝语义，避免把限流和传输结果未知混为一谈。
type HTTPError struct {
	StatusCode int
	Body       string
	RetryAfter time.Duration
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("delivery request failed: status=%d body=%s", e.StatusCode, e.Body)
}

func retryAfter(header string, body []byte) time.Duration {
	var payload struct {
		RetryAfter float64 `json:"retry_after"`
		Parameters struct {
			RetryAfter float64 `json:"retry_after"`
		} `json:"parameters"`
	}
	_ = json.Unmarshal(body, &payload)
	seconds := max(payload.RetryAfter, payload.Parameters.RetryAfter)
	if value, err := strconv.ParseFloat(header, 64); err == nil {
		seconds = max(seconds, value)
	}
	if deadline, err := http.ParseTime(header); err == nil {
		seconds = max(seconds, time.Until(deadline).Seconds())
	}
	if seconds > 0 && seconds <= 86400 {
		return time.Duration(seconds * float64(time.Second))
	}
	return 0
}
