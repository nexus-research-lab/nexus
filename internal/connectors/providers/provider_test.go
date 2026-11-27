package providers

import (
	"context"
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
