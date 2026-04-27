package providers

import (
	"context"
	"net/http"
	"net/url"
)

const (
	defaultGoogleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultGoogleTokenURL = "https://oauth2.googleapis.com/token"
)

type googleProvider struct {
	authURL  string
	tokenURL string
}

// NewGoogleProvider 创建 Google Provider，Gmail 使用 PKCE 并强制离线授权。
func NewGoogleProvider(authURL string, tokenURL string) Provider {
	return googleProvider{authURL: authURL, tokenURL: tokenURL}
}

func init() {
	Register(NewGoogleProvider(defaultGoogleAuthURL, defaultGoogleTokenURL))
}

func (p googleProvider) ConnectorID() string {
	return "gmail"
}

func (p googleProvider) APIBaseURL() string {
	return "https://gmail.googleapis.com"
}

func (p googleProvider) RequiresPKCE() bool {
	return true
}

func (p googleProvider) RequiredExtraKeys() []string {
	return nil
}

func (p googleProvider) BuildAuthURL(_ context.Context, req AuthRequest) (string, error) {
	authURL, params, err := withCommonAuthParams(p.authURL, req)
	if err != nil {
		return "", err
	}
	params.Set("response_type", "code")
	params.Set("access_type", "offline")
	params.Set("prompt", "consent")
	params.Set("include_granted_scopes", "true")
	params.Set("code_challenge", req.CodeVerifier)
	params.Set("code_challenge_method", "S256")
	authURL.RawQuery = params.Encode()
	return authURL.String(), nil
}

func (p googleProvider) ExchangeToken(ctx context.Context, httpClient *http.Client, req TokenRequest) ([]byte, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", req.Code)
	form.Set("redirect_uri", req.RedirectURI)
	form.Set("client_id", req.ClientID)
	form.Set("client_secret", req.ClientSecret)
	form.Set("code_verifier", req.CodeVerifier)
	payload, err := PostForm(ctx, httpClient, p.tokenURL, form, "", "")
	if err != nil {
		return nil, err
	}
	if err = SniffJSONError(payload); err != nil {
		return nil, err
	}
	return payload, nil
}
