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
