// INPUT: 钉钉官方应用注册初始化、开始与轮询接口。
// OUTPUT: 钉钉机器人创建二维码及标准化应用凭据。
// POS: appregistration 内的钉钉协议适配，不承载频道业务状态。
package appregistration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const defaultDingTalkRegistrationURL = "https://oapi.dingtalk.com"

type dingTalkClient struct {
	httpClient *http.Client
	baseURL    string
}

// NewDingTalkClient creates a client for DingTalk's official one-click robot registration.
func NewDingTalkClient(httpClient *http.Client, baseURL string) Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultDingTalkRegistrationURL
	}
	return &dingTalkClient{httpClient: effectiveHTTPClient(httpClient), baseURL: strings.TrimRight(baseURL, "/")}
}

func (c *dingTalkClient) Start(ctx context.Context) (*StartResult, error) {
	var initialized struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Nonce   string `json:"nonce"`
	}
	if err := c.postJSON(ctx, "/app/registration/init", map[string]string{"source": "DING_DWS_CLAW"}, &initialized); err != nil {
		return nil, err
	}
	if initialized.ErrCode != 0 || strings.TrimSpace(initialized.Nonce) == "" {
		return nil, fmt.Errorf("钉钉应用注册初始化失败: %s", initialized.ErrMsg)
	}
	var begun struct {
		ErrCode                 int    `json:"errcode"`
		ErrMsg                  string `json:"errmsg"`
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationURI         string `json:"verification_uri"`
		VerificationURIComplete string `json:"verification_uri_complete"`
		ExpiresIn               int    `json:"expires_in"`
		Interval                int    `json:"interval"`
	}
	if err := c.postJSON(ctx, "/app/registration/begin", map[string]string{"nonce": initialized.Nonce}, &begun); err != nil {
		return nil, err
	}
	if begun.ErrCode != 0 || strings.TrimSpace(begun.DeviceCode) == "" || strings.TrimSpace(begun.VerificationURIComplete) == "" {
		return nil, fmt.Errorf("钉钉应用注册启动失败: %s", begun.ErrMsg)
	}
	return &StartResult{
		DeviceCode:              strings.TrimSpace(begun.DeviceCode),
		UserCode:                strings.TrimSpace(begun.UserCode),
		VerificationURI:         strings.TrimSpace(begun.VerificationURI),
		VerificationURIComplete: strings.TrimSpace(begun.VerificationURIComplete),
		ExpiresIn:               positiveOr(begun.ExpiresIn, 7200),
		Interval:                positiveOr(begun.Interval, 3),
	}, nil
}

func (c *dingTalkClient) Poll(ctx context.Context, deviceCode string) (*PollResult, error) {
	var result struct {
		ErrCode      int    `json:"errcode"`
		ErrMsg       string `json:"errmsg"`
		Status       string `json:"status"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		FailReason   string `json:"fail_reason"`
	}
	if err := c.postJSON(ctx, "/app/registration/poll", map[string]string{"device_code": strings.TrimSpace(deviceCode)}, &result); err != nil {
		return nil, err
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("钉钉应用注册轮询失败: %s", result.ErrMsg)
	}
	switch strings.ToUpper(strings.TrimSpace(result.Status)) {
	case "WAITING", "":
		return &PollResult{Status: StatusPending, Message: "等待钉钉扫码确认"}, nil
	case "SUCCESS":
		if result.ClientID == "" || result.ClientSecret == "" {
			return nil, errors.New("钉钉扫码成功但未返回应用凭据")
		}
		return &PollResult{
			Status: StatusSucceeded,
			Credentials: map[string]string{
				"client_id":     strings.TrimSpace(result.ClientID),
				"client_secret": strings.TrimSpace(result.ClientSecret),
			},
		}, nil
	case "EXPIRED":
		return &PollResult{Status: StatusExpired, Message: "钉钉二维码已过期"}, nil
	case "FAIL":
		return &PollResult{Status: StatusFailed, Message: firstMessage(result.FailReason, "钉钉应用创建失败")}, nil
	default:
		return nil, fmt.Errorf("未知钉钉应用注册状态: %s", result.Status)
	}
}

func (c *dingTalkClient) postJSON(ctx context.Context, path string, input any, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("registration HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	return json.Unmarshal(payload, output)
}

func firstMessage(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}
