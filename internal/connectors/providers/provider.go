package providers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// AuthRequest 是 Provider 构造 authorize URL 时需要的上下文。
type AuthRequest struct {
	ClientID     string
	RedirectURI  string
	Scopes       []string
	State        string
	CodeVerifier string
	Extra        map[string]string
}

// TokenRequest 是 Provider 兑换 token 时需要的上下文。
type TokenRequest struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Code         string
	CodeVerifier string
	Extra        map[string]string
}

// Provider 定义单个 OAuth 供应商。
type Provider interface {
	ConnectorID() string
	APIBaseURL() string
	RequiresPKCE() bool
	RequiredExtraKeys() []string
	BuildAuthURL(ctx context.Context, req AuthRequest) (string, error)
	ExchangeToken(ctx context.Context, httpClient *http.Client, req TokenRequest) ([]byte, error)
}

// GeneratePKCE 返回 verifier 和 S256 challenge。
func GeneratePKCE() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	verifier := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

// RandomState 生成 hex state token。
func RandomState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", buf), nil
}

// PostForm 是标准 token 端点 POST x-www-form-urlencoded 的辅助。
func PostForm(ctx context.Context, httpClient *http.Client, endpoint string, form url.Values, basicAuthUser, basicAuthPass string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if basicAuthUser != "" {
		req.SetBasicAuth(basicAuthUser, basicAuthPass)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("token exchange HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	return payload, nil
}

// SniffJSONError 检查 200 响应里是否夹带 error 字段。
func SniffJSONError(payload []byte) error {
	if !json.Valid(payload) {
		return nil
	}
	var parsed struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return nil
	}
	if parsed.Error == "" {
		return nil
	}
	if parsed.ErrorDescription != "" {
		return errors.New(parsed.Error + ": " + parsed.ErrorDescription)
	}
	return errors.New(parsed.Error)
}

// Clock 用于测试注入固定时间。
type Clock func() time.Time

var _ = Clock(time.Now)

func withCommonAuthParams(rawURL string, req AuthRequest) (*url.URL, url.Values, error) {
	authURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, err
	}
	params := authURL.Query()
	params.Set("client_id", req.ClientID)
	params.Set("redirect_uri", req.RedirectURI)
	params.Set("state", req.State)
	if len(req.Scopes) > 0 {
		params.Set("scope", strings.Join(req.Scopes, " "))
	}
	return authURL, params, nil
}
