package providers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

const (
	defaultGitHubAuthURL       = "https://github.com/login/oauth/authorize"
	defaultGitHubTokenURL      = "https://github.com/login/oauth/access_token"
	defaultGitHubDeviceCodeURL = "https://github.com/login/device/code"
)

type githubProvider struct {
	authURL       string
	tokenURL      string
	deviceCodeURL string
}

// NewGitHubProvider 创建 GitHub Provider，GitHub 会在 200 JSON 中返回 error 字段。
func NewGitHubProvider(authURL string, tokenURL string) Provider {
	return NewGitHubProviderWithDeviceURL(authURL, tokenURL, defaultGitHubDeviceCodeURL)
}

// NewGitHubProviderWithDeviceURL 创建可覆盖 Device Flow endpoint 的 GitHub Provider。
func NewGitHubProviderWithDeviceURL(authURL string, tokenURL string, deviceCodeURL string) Provider {
	return githubProvider{authURL: authURL, tokenURL: tokenURL, deviceCodeURL: deviceCodeURL}
}

func init() {
	Register(NewGitHubProvider(defaultGitHubAuthURL, defaultGitHubTokenURL))
}

func (p githubProvider) ConnectorID() string {
	return "github"
}

func (p githubProvider) APIBaseURL() string {
	return "https://api.github.com"
}

func (p githubProvider) RequiresPKCE() bool {
	return false
}

func (p githubProvider) RequiredExtraKeys() []string {
	return nil
}

func (p githubProvider) BuildAuthURL(_ context.Context, req AuthRequest) (string, error) {
	authURL, params, err := withCommonAuthParams(p.authURL, req)
	if err != nil {
		return "", err
	}
	params.Set("response_type", "code")
	authURL.RawQuery = params.Encode()
	return authURL.String(), nil
}

func (p githubProvider) ExchangeToken(ctx context.Context, httpClient *http.Client, req TokenRequest) ([]byte, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", req.Code)
	form.Set("redirect_uri", req.RedirectURI)
	form.Set("client_id", req.ClientID)
	form.Set("client_secret", req.ClientSecret)
	payload, err := PostForm(ctx, httpClient, p.tokenURL, form, "", "")
	if err != nil {
		return nil, err
	}
	if err = SniffJSONError(payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (p githubProvider) RequestDeviceCode(ctx context.Context, httpClient *http.Client, req DeviceCodeRequest) (*DeviceCodeResponse, error) {
	form := url.Values{}
	form.Set("client_id", strings.TrimSpace(req.ClientID))
	if len(req.Scopes) > 0 {
		form.Set("scope", strings.Join(req.Scopes, " "))
	}
	payload, err := PostForm(ctx, httpClient, p.deviceCodeURL, form, "", "")
	if err != nil {
		return nil, err
	}
	if err = SniffJSONError(payload); err != nil {
		return nil, err
	}
	var result DeviceCodeResponse
	if err = json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	result.DeviceCode = strings.TrimSpace(result.DeviceCode)
	result.UserCode = strings.TrimSpace(result.UserCode)
	result.VerificationURI = strings.TrimSpace(result.VerificationURI)
	if result.DeviceCode == "" || result.UserCode == "" || result.VerificationURI == "" {
		return nil, errors.New("GitHub Device Flow 响应不完整")
	}
	if result.Interval <= 0 {
		result.Interval = 5
	}
	return &result, nil
}

func (p githubProvider) ExchangeDeviceToken(ctx context.Context, httpClient *http.Client, req DeviceTokenRequest) ([]byte, error) {
	form := url.Values{}
	form.Set("client_id", strings.TrimSpace(req.ClientID))
	form.Set("device_code", strings.TrimSpace(req.DeviceCode))
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	payload, err := PostForm(ctx, httpClient, p.tokenURL, form, "", "")
	if err != nil {
		return nil, err
	}
	if err = SniffJSONError(payload); err != nil {
		return nil, err
	}
	return payload, nil
}
