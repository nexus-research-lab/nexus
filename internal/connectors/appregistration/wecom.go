// INPUT: 企业微信官方智能机器人二维码生成与结果查询接口。
// OUTPUT: 企业微信绑定二维码及标准化机器人凭据。
// POS: appregistration 内的企业微信协议适配，不承载频道业务状态。
package appregistration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"
)

const defaultWeComRegistrationURL = "https://work.weixin.qq.com"

type weComClient struct {
	httpClient *http.Client
	baseURL    string
}

// NewWeComClient creates a client for WeCom's official intelligent-bot QR binding.
func NewWeComClient(httpClient *http.Client, baseURL string) Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultWeComRegistrationURL
	}
	return &weComClient{httpClient: effectiveHTTPClient(httpClient), baseURL: strings.TrimRight(baseURL, "/")}
}

func (c *weComClient) Start(ctx context.Context) (*StartResult, error) {
	var result struct {
		Data struct {
			SCode   string `json:"scode"`
			AuthURL string `json:"auth_url"`
		} `json:"data"`
	}
	query := url.Values{"source": {"wecom-cli"}, "plat": {weComPlatformCode()}}
	if err := c.getJSON(ctx, "/ai/qc/generate?"+query.Encode(), &result); err != nil {
		return nil, err
	}
	if strings.TrimSpace(result.Data.SCode) == "" || strings.TrimSpace(result.Data.AuthURL) == "" {
		return nil, errors.New("企业微信二维码响应不完整")
	}
	return &StartResult{
		DeviceCode:              strings.TrimSpace(result.Data.SCode),
		VerificationURI:         c.baseURL + "/ai/qc/gen?source=wecom-cli&scode=" + url.QueryEscape(result.Data.SCode),
		VerificationURIComplete: strings.TrimSpace(result.Data.AuthURL),
		ExpiresIn:               300,
		Interval:                3,
	}, nil
}

func (c *weComClient) Poll(ctx context.Context, deviceCode string) (*PollResult, error) {
	var result struct {
		Data struct {
			Status  string `json:"status"`
			BotInfo struct {
				BotID  string `json:"botid"`
				Secret string `json:"secret"`
			} `json:"bot_info"`
		} `json:"data"`
	}
	if err := c.getJSON(ctx, "/ai/qc/query_result?scode="+url.QueryEscape(strings.TrimSpace(deviceCode)), &result); err != nil {
		return nil, err
	}
	if strings.EqualFold(strings.TrimSpace(result.Data.Status), "success") {
		if result.Data.BotInfo.BotID == "" || result.Data.BotInfo.Secret == "" {
			return nil, errors.New("企业微信扫码成功但未返回机器人凭据")
		}
		return &PollResult{
			Status: StatusSucceeded,
			Credentials: map[string]string{
				"bot_id": strings.TrimSpace(result.Data.BotInfo.BotID),
				"secret": strings.TrimSpace(result.Data.BotInfo.Secret),
			},
		}, nil
	}
	return &PollResult{Status: StatusPending, Message: "等待企业微信扫码确认"}, nil
}

func (c *weComClient) getJSON(ctx context.Context, path string, output any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
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

func weComPlatformCode() string {
	switch runtime.GOOS {
	case "darwin":
		return "1"
	case "windows":
		return "2"
	case "linux":
		return "3"
	default:
		return "0"
	}
}
