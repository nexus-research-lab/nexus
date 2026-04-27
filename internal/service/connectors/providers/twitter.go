package providers

import (
	"context"
	"net/http"
	"net/url"
)

const (
	defaultTwitterAuthURL  = "https://twitter.com/i/oauth2/authorize"
	defaultTwitterTokenURL = "https://api.twitter.com/2/oauth2/token"
)

type twitterProvider struct {
	authURL  string
	tokenURL string
}

// NewTwitterProvider 创建 X/Twitter Provider，token 端点走 Basic Auth。
func NewTwitterProvider(authURL string, tokenURL string) Provider {
	return twitterProvider{authURL: authURL, tokenURL: tokenURL}
}

func init() {
	Register(NewTwitterProvider(defaultTwitterAuthURL, defaultTwitterTokenURL))
}

func (p twitterProvider) ConnectorID() string {
	return "x-twitter"
}

func (p twitterProvider) APIBaseURL() string {
	return "https://api.twitter.com"
}

func (p twitterProvider) RequiresPKCE() bool {
	return true
}

func (p twitterProvider) RequiredExtraKeys() []string {
	return nil
}

func (p twitterProvider) BuildAuthURL(_ context.Context, req AuthRequest) (string, error) {
	authURL, params, err := withCommonAuthParams(p.authURL, req)
	if err != nil {
		return "", err
	}
	params.Set("response_type", "code")
	params.Set("code_challenge", req.CodeVerifier)
	params.Set("code_challenge_method", "S256")
	authURL.RawQuery = params.Encode()
	return authURL.String(), nil
}

func (p twitterProvider) ExchangeToken(ctx context.Context, httpClient *http.Client, req TokenRequest) ([]byte, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", req.Code)
	form.Set("redirect_uri", req.RedirectURI)
	form.Set("client_id", req.ClientID)
	form.Set("code_verifier", req.CodeVerifier)
	return PostForm(ctx, httpClient, p.tokenURL, form, req.ClientID, req.ClientSecret)
}
