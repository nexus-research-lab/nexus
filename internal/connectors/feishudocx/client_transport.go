package feishudocx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
)

func (c *Client) ensureAccessToken() error {
	if c.accessToken == "" {
		return errors.New("飞书连接缺少 access token")
	}
	return nil
}

func (c *Client) authOption() larkcore.RequestOptionFunc {
	return larkcore.WithUserAccessToken(c.accessToken)
}

func (c *Client) doJSON(ctx context.Context, method string, apiPath string, payload any, out any) error {
	if err := c.ensureAccessToken(); err != nil {
		return err
	}
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	endpoint := strings.TrimRight(c.apiBaseURL, "/") + apiPath
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > maxResponseBytes {
		return errors.New("飞书 API 响应过大")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("飞书 API HTTP 错误 %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var envelope struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	if envelope.Code != 0 {
		return sdkCodeError(envelope.Code, envelope.Msg, resp.Header.Get("X-Request-Id"))
	}
	if out == nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	return json.Unmarshal(envelope.Data, out)
}

func newSDKConfig(baseURL string, httpClient *http.Client) *larkcore.Config {
	config := &larkcore.Config{
		BaseUrl:          strings.TrimRight(baseURL, "/"),
		AppId:            sdkAppID,
		HttpClient:       limitedHTTPClient{base: httpClient, maxBytes: maxResponseBytes},
		EnableTokenCache: false,
	}
	larkcore.NewLogger(config)
	larkcore.NewSerialization(config)
	larkcore.NewHttpClient(config)
	return config
}

func sdkCodeError(code int, msg string, requestID string) error {
	if strings.TrimSpace(msg) == "" {
		msg = "unknown"
	}
	if strings.TrimSpace(requestID) != "" {
		return fmt.Errorf("飞书 API 返回错误 %d: %s (request_id: %s)", code, msg, requestID)
	}
	return fmt.Errorf("飞书 API 返回错误 %d: %s", code, msg)
}

type limitedHTTPClient struct {
	base     larkcore.HttpClient
	maxBytes int64
}

func (client limitedHTTPClient) Do(request *http.Request) (*http.Response, error) {
	resp, err := client.base.Do(request)
	if err != nil || resp == nil || resp.Body == nil || client.maxBytes <= 0 {
		return resp, err
	}
	resp.Body = &maxBytesReadCloser{body: resp.Body, maxBytes: client.maxBytes}
	return resp, nil
}

type maxBytesReadCloser struct {
	body     io.ReadCloser
	maxBytes int64
	read     int64
}

func (reader *maxBytesReadCloser) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return reader.body.Read(buffer)
	}
	if reader.read >= reader.maxBytes {
		var probe [1]byte
		n, err := reader.body.Read(probe[:])
		if n > 0 {
			return 0, errors.New("飞书 API 响应过大")
		}
		return 0, err
	}
	remaining := reader.maxBytes - reader.read
	if int64(len(buffer)) > remaining {
		buffer = buffer[:remaining]
	}
	n, err := reader.body.Read(buffer)
	reader.read += int64(n)
	return n, err
}

func (reader *maxBytesReadCloser) Close() error {
	return reader.body.Close()
}
