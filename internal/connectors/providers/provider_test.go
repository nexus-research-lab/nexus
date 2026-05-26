package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestGitHubExchangeTokenSniffsJSONError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"error":"bad_verification_code","error_description":"code expired"}`))
	}))
	defer server.Close()

	provider := NewGitHubProvider("https://github.test/authorize", server.URL)
	_, err := provider.ExchangeToken(context.Background(), server.Client(), TokenRequest{
		ClientID:     "client",
		ClientSecret: "secret",
		RedirectURI:  "http://localhost/callback",
		Code:         "bad-code",
	})
	if err == nil || !strings.Contains(err.Error(), "bad_verification_code") {
		t.Fatalf("expected GitHub JSON error to be surfaced, got %v", err)
	}
}

func TestTwitterExchangeTokenUsesBasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		user, pass, ok := request.BasicAuth()
		if !ok || user != "twitter-client" || pass != "twitter-secret" {
			t.Fatalf("basic auth missing or wrong: ok=%v user=%q pass=%q", ok, user, pass)
		}
		if err := request.ParseForm(); err != nil {
			t.Fatalf("parse token form: %v", err)
		}
		if request.Form.Get("client_secret") != "" {
			t.Fatalf("client_secret must not be sent in Twitter form: %v", request.Form)
		}
		if request.Form.Get("client_id") != "twitter-client" {
			t.Fatalf("client_id missing from Twitter form: %v", request.Form)
		}
		if request.Form.Get("code_verifier") != "stored-verifier" {
			t.Fatalf("code_verifier not sent from stored state: %v", request.Form)
		}
		_, _ = writer.Write([]byte(`{"access_token":"ok","refresh_token":"refresh"}`))
	}))
	defer server.Close()

	provider := NewTwitterProvider("https://twitter.test/authorize", server.URL)
	payload, err := provider.ExchangeToken(context.Background(), server.Client(), TokenRequest{
		ClientID:     "twitter-client",
		ClientSecret: "twitter-secret",
		RedirectURI:  "http://localhost/callback",
		Code:         "code",
		CodeVerifier: "stored-verifier",
	})
	if err != nil {
		t.Fatalf("twitter token exchange failed: %v", err)
	}
	if !strings.Contains(string(payload), "access_token") {
		t.Fatalf("unexpected payload: %s", payload)
	}
}

func TestShopifyBuildAuthURLRequiresValidShop(t *testing.T) {
	provider := NewShopifyProvider()
	_, err := provider.BuildAuthURL(context.Background(), AuthRequest{
		ClientID:    "client",
		RedirectURI: "http://localhost/callback",
		State:       "state",
		Scopes:      []string{"read_products"},
	})
	if err == nil || !strings.Contains(err.Error(), "shop 参数缺失") {
		t.Fatalf("expected missing shop error, got %v", err)
	}

	authURL, err := provider.BuildAuthURL(context.Background(), AuthRequest{
		ClientID:    "client",
		RedirectURI: "http://localhost/callback",
		State:       "state",
		Scopes:      []string{"read_products"},
		Extra:       map[string]string{"shop": "demo-store"},
	})
	if err != nil {
		t.Fatalf("build shopify auth url: %v", err)
	}
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse shopify auth url: %v", err)
	}
	if parsed.Host != "demo-store.myshopify.com" {
		t.Fatalf("unexpected shopify host: %s", authURL)
	}
	if parsed.Query().Get("response_type") != "" {
		t.Fatalf("shopify auth url should not include response_type: %s", authURL)
	}
}

func TestFeishuDocxBuildAuthURLUsesAuthorizationCode(t *testing.T) {
	provider := NewFeishuDocxProvider("https://accounts.feishu.test/open-apis/authen/v1/authorize", "https://open.feishu.test/token", "https://open.feishu.test")
	authURL, err := provider.BuildAuthURL(context.Background(), AuthRequest{
		ClientID:    "feishu-docx-client",
		RedirectURI: "http://localhost:3000/callback",
		State:       "state-token",
		Scopes:      []string{"docx:document", "offline_access"},
	})
	if err != nil {
		t.Fatalf("build feishu auth url: %v", err)
	}
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse feishu auth url: %v", err)
	}
	query := parsed.Query()
	if query.Get("response_type") != "code" || query.Get("client_id") != "feishu-docx-client" {
		t.Fatalf("飞书授权地址参数不正确: %s", authURL)
	}
	if !strings.Contains(query.Get("scope"), "offline_access") {
		t.Fatalf("飞书授权地址未带离线权限: %s", authURL)
	}
}

func TestFeishuDocxExchangeTokenUsesJSONAndFlattensData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if contentType := request.Header.Get("Content-Type"); !strings.Contains(contentType, "application/json") {
			t.Fatalf("飞书 token 请求应使用 JSON，实际: %q", contentType)
		}
		var body map[string]string
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("解析飞书 token 请求失败: %v", err)
		}
		if body["grant_type"] != "authorization_code" || body["client_id"] != "feishu-docx-client" || body["client_secret"] != "feishu-docx-secret" {
			t.Fatalf("飞书 token 请求体不正确: %+v", body)
		}
		_, _ = writer.Write([]byte(`{"code":0,"msg":"success","data":{"access_token":"feishu-docx-token","refresh_token":"refresh","expires_in":7200}}`))
	}))
	defer server.Close()

	provider := NewFeishuDocxProvider("https://accounts.feishu.test/authorize", server.URL, "https://open.feishu.test")
	payload, err := provider.ExchangeToken(context.Background(), server.Client(), TokenRequest{
		ClientID:     "feishu-docx-client",
		ClientSecret: "feishu-docx-secret",
		RedirectURI:  "http://localhost:3000/callback",
		Code:         "code",
	})
	if err != nil {
		t.Fatalf("飞书 token exchange 失败: %v", err)
	}
	if !strings.Contains(string(payload), `"access_token":"feishu-docx-token"`) || strings.Contains(string(payload), `"data"`) {
		t.Fatalf("飞书 token 响应应被压平: %s", payload)
	}
}

func TestFeishuDocxRefreshTokenUsesJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("解析飞书 refresh 请求失败: %v", err)
		}
		if body["grant_type"] != "refresh_token" || body["refresh_token"] != "old-refresh" {
			t.Fatalf("飞书 refresh 请求体不正确: %+v", body)
		}
		_, _ = writer.Write([]byte(`{"access_token":"new-token","refresh_token":"new-refresh","expires_in":7200}`))
	}))
	defer server.Close()

	provider := NewFeishuDocxProvider("https://accounts.feishu.test/authorize", server.URL, "https://open.feishu.test")
	payload, err := provider.(RefreshTokenProvider).RefreshToken(context.Background(), server.Client(), TokenRefreshRequest{
		ClientID:     "feishu-docx-client",
		ClientSecret: "feishu-docx-secret",
		RefreshToken: "old-refresh",
	})
	if err != nil {
		t.Fatalf("飞书 refresh 失败: %v", err)
	}
	if !strings.Contains(string(payload), `"access_token":"new-token"`) {
		t.Fatalf("飞书 refresh 响应不正确: %s", payload)
	}
}

func TestFeishuDocxTokenResponseSurfacesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"code":99991663,"msg":"invalid redirect_uri"}`))
	}))
	defer server.Close()

	provider := NewFeishuDocxProvider("https://accounts.feishu.test/authorize", server.URL, "https://open.feishu.test")
	_, err := provider.ExchangeToken(context.Background(), server.Client(), TokenRequest{
		ClientID:     "feishu-docx-client",
		ClientSecret: "feishu-docx-secret",
		RedirectURI:  "http://localhost:3000/callback",
		Code:         "code",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid redirect_uri") {
		t.Fatalf("飞书 token 错误未透出: %v", err)
	}
}
