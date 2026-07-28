// INPUT: 飞书官方应用注册协议、预填应用信息及增量权限/事件。
// OUTPUT: 允许选择已有应用或创建新应用的二维码，以及轮询取得的应用凭据。
// POS: appregistration 内的飞书协议适配，不决定凭据的业务归属与持久化。
package appregistration

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const defaultFeishuRegistrationURL = "https://accounts.feishu.cn/oauth/v1/app/registration"

// FeishuOptions describes the app shown on the official QR registration page.
type FeishuOptions struct {
	RegistrationURL string
	Name            string
	Description     string
	TenantScopes    []string
	UserScopes      []string
	Events          []string
}

type feishuClient struct {
	httpClient *http.Client
	options    FeishuOptions
}

// NewFeishuClient creates a client for Feishu's official app choose-or-create flow.
func NewFeishuClient(httpClient *http.Client, options FeishuOptions) Client {
	if strings.TrimSpace(options.RegistrationURL) == "" {
		options.RegistrationURL = defaultFeishuRegistrationURL
	}
	return &feishuClient{httpClient: effectiveHTTPClient(httpClient), options: options}
}

func (c *feishuClient) Start(ctx context.Context) (*StartResult, error) {
	form := url.Values{
		"action":            {"begin"},
		"archetype":         {"PersonalAgent"},
		"auth_method":       {"client_secret"},
		"request_user_info": {"open_id"},
	}
	payload, err := postForm(ctx, c.httpClient, c.options.RegistrationURL, form)
	if err != nil {
		return nil, err
	}
	var result struct {
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationURI         string `json:"verification_uri"`
		VerificationURIComplete string `json:"verification_uri_complete"`
		ExpiresIn               int    `json:"expires_in"`
		Interval                int    `json:"interval"`
		Error                   string `json:"error"`
		ErrorDescription        string `json:"error_description"`
	}
	if err = json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	if result.Error != "" {
		return nil, registrationError(result.Error, result.ErrorDescription)
	}
	if strings.TrimSpace(result.DeviceCode) == "" || strings.TrimSpace(result.VerificationURIComplete) == "" {
		return nil, errors.New("飞书应用注册响应不完整")
	}
	qrURL, err := url.Parse(result.VerificationURIComplete)
	if err != nil {
		return nil, err
	}
	query := qrURL.Query()
	query.Set("from", "sdk")
	query.Set("source", "node-sdk/nexus")
	query.Set("tp", "sdk")
	if c.options.Name != "" {
		query.Set("name", c.options.Name)
	}
	if c.options.Description != "" {
		query.Set("desc", c.options.Description)
	}
	if addons, encodeErr := encodeFeishuAddons(c.options); encodeErr != nil {
		return nil, encodeErr
	} else if addons != "" {
		query.Set("addons", addons)
	}
	qrURL.RawQuery = query.Encode()
	return &StartResult{
		DeviceCode:              strings.TrimSpace(result.DeviceCode),
		UserCode:                strings.TrimSpace(result.UserCode),
		VerificationURI:         strings.TrimSpace(result.VerificationURI),
		VerificationURIComplete: qrURL.String(),
		ExpiresIn:               positiveOr(result.ExpiresIn, 600),
		Interval:                positiveOr(result.Interval, 5),
	}, nil
}

func (c *feishuClient) Poll(ctx context.Context, deviceCode string) (*PollResult, error) {
	payload, err := postForm(ctx, c.httpClient, c.options.RegistrationURL, url.Values{
		"action":      {"poll"},
		"device_code": {strings.TrimSpace(deviceCode)},
	})
	if err != nil {
		return nil, err
	}
	var result struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		Error        string `json:"error"`
		Description  string `json:"error_description"`
		UserInfo     struct {
			OpenID      string `json:"open_id"`
			TenantBrand string `json:"tenant_brand"`
		} `json:"user_info"`
	}
	if err = json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	if result.UserInfo.TenantBrand == "lark" {
		return &PollResult{Status: StatusFailed, Message: "当前连接器仅支持飞书中国版账号"}, nil
	}
	if result.ClientID != "" && result.ClientSecret != "" {
		return &PollResult{
			Status: StatusSucceeded,
			Credentials: map[string]string{
				"client_id":     strings.TrimSpace(result.ClientID),
				"client_secret": strings.TrimSpace(result.ClientSecret),
			},
			UserID: strings.TrimSpace(result.UserInfo.OpenID),
		}, nil
	}
	switch result.Error {
	case "authorization_pending", "":
		return &PollResult{Status: StatusPending, Message: "等待飞书扫码确认"}, nil
	case "slow_down":
		return &PollResult{Status: StatusSlowDown, Message: "飞书要求降低轮询频率"}, nil
	case "expired_token":
		return &PollResult{Status: StatusExpired, Message: "飞书二维码已过期"}, nil
	case "access_denied":
		return &PollResult{Status: StatusFailed, Message: "用户取消了飞书应用连接"}, nil
	default:
		return nil, registrationError(result.Error, result.Description)
	}
}

func encodeFeishuAddons(options FeishuOptions) (string, error) {
	if len(options.TenantScopes)+len(options.UserScopes)+len(options.Events) == 0 {
		return "", nil
	}
	payload := map[string]any{}
	if len(options.TenantScopes)+len(options.UserScopes) > 0 {
		scopes := map[string]any{}
		if len(options.TenantScopes) > 0 {
			scopes["tenant"] = options.TenantScopes
		}
		if len(options.UserScopes) > 0 {
			scopes["user"] = options.UserScopes
		}
		payload["scopes"] = scopes
	}
	if len(options.Events) > 0 {
		payload["events"] = map[string]any{
			"items": map[string]any{"tenant": options.Events},
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err = writer.Write(raw); err != nil {
		return "", err
	}
	if err = writer.Close(); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(compressed.Bytes()), nil
}

func postForm(ctx context.Context, client *http.Client, endpoint string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest && !json.Valid(payload) {
		return nil, fmt.Errorf("registration HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	return payload, nil
}

func registrationError(code string, description string) error {
	if strings.TrimSpace(description) == "" {
		return errors.New(strings.TrimSpace(code))
	}
	return fmt.Errorf("%s: %s", strings.TrimSpace(code), strings.TrimSpace(description))
}

func positiveOr(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
