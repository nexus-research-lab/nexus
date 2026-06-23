package providers

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const (
	defaultFeishuDocxAuthURL  = "https://accounts.feishu.cn/open-apis/authen/v1/authorize"
	defaultFeishuDocxTokenURL = "https://open.feishu.cn/open-apis/authen/v2/oauth/token"
	defaultFeishuDocxAPIURL   = "https://open.feishu.cn"
)

type feishuDocxProvider struct {
	authURL  string
	tokenURL string
	apiURL   string
}

// NewFeishuDocxProvider 创建飞书云文档 OAuth Provider。
func NewFeishuDocxProvider(authURL string, tokenURL string, apiURL string) Provider {
	return feishuDocxProvider{
		authURL:  cmp.Or(authURL, defaultFeishuDocxAuthURL),
		tokenURL: cmp.Or(tokenURL, defaultFeishuDocxTokenURL),
		apiURL:   cmp.Or(apiURL, defaultFeishuDocxAPIURL),
	}
}

func init() {
	Register(NewFeishuDocxProvider(defaultFeishuDocxAuthURL, defaultFeishuDocxTokenURL, defaultFeishuDocxAPIURL))
}

func (p feishuDocxProvider) ConnectorID() string {
	return "feishu-docx"
}

func (p feishuDocxProvider) APIBaseURL() string {
	return p.apiURL
}

func (p feishuDocxProvider) RequiresPKCE() bool {
	return false
}

func (p feishuDocxProvider) RequiredExtraKeys() []string {
	return nil
}

func (p feishuDocxProvider) BuildAuthURL(_ context.Context, req AuthRequest) (string, error) {
	authURL, params, err := withCommonAuthParams(p.authURL, req)
	if err != nil {
		return "", err
	}
	params.Set("response_type", "code")
	authURL.RawQuery = params.Encode()
	return authURL.String(), nil
}

func (p feishuDocxProvider) ExchangeToken(ctx context.Context, httpClient *http.Client, req TokenRequest) ([]byte, error) {
	payload := map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     req.ClientID,
		"client_secret": req.ClientSecret,
		"code":          req.Code,
		"redirect_uri":  req.RedirectURI,
	}
	return postFeishuDocxTokenJSON(ctx, httpClient, p.tokenURL, payload)
}

func (p feishuDocxProvider) RefreshToken(ctx context.Context, httpClient *http.Client, req TokenRefreshRequest) ([]byte, error) {
	payload := map[string]string{
		"grant_type":    "refresh_token",
		"client_id":     req.ClientID,
		"client_secret": req.ClientSecret,
		"refresh_token": req.RefreshToken,
	}
	return postFeishuDocxTokenJSON(ctx, httpClient, p.tokenURL, payload)
}

func postFeishuDocxTokenJSON(ctx context.Context, httpClient *http.Client, endpoint string, payload map[string]string) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	responsePayload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("token exchange HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(responsePayload)))
	}
	return normalizeFeishuDocxTokenResponse(responsePayload)
}

func normalizeFeishuDocxTokenResponse(payload []byte) ([]byte, error) {
	if !json.Valid(payload) {
		return payload, nil
	}
	var parsed map[string]any
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return nil, err
	}
	if errMsg := feishuDocxResponseError(parsed); errMsg != "" {
		return nil, errors.New(errMsg)
	}
	if data, ok := parsed["data"].(map[string]any); ok {
		return json.Marshal(data)
	}
	return payload, nil
}

func feishuDocxResponseError(parsed map[string]any) string {
	if raw, ok := parsed["error"].(string); ok && raw != "" {
		if desc, ok := parsed["error_description"].(string); ok && desc != "" {
			return raw + ": " + desc
		}
		return raw
	}
	code, hasCode := numericValue(parsed["code"])
	if !hasCode || code == 0 {
		return ""
	}
	if msg, ok := parsed["msg"].(string); ok && msg != "" {
		return msg
	}
	if msg, ok := parsed["message"].(string); ok && msg != "" {
		return msg
	}
	return fmt.Sprintf("Feishu Docx OAuth error code %.0f", code)
}

func numericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case json.Number:
		n, err := typed.Float64()
		return n, err == nil
	case string:
		if strings.TrimSpace(typed) == "" {
			return 0, false
		}
		n, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return n, err == nil
	default:
		return 0, false
	}
}
