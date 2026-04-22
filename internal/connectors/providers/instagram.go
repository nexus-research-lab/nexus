package providers

import (
	"context"
	"net/http"
	"net/url"
)

const (
	defaultInstagramAuthURL  = "https://www.instagram.com/oauth/authorize"
	defaultInstagramTokenURL = "https://api.instagram.com/oauth/access_token"
)

type instagramProvider struct {
	authURL  string
	tokenURL string
}

// NewInstagramProvider 创建 Instagram Provider，使用新版 Instagram OAuth 地址。
func NewInstagramProvider(authURL string, tokenURL string) Provider {
	return instagramProvider{authURL: authURL, tokenURL: tokenURL}
}

func init() {
	Register(NewInstagramProvider(defaultInstagramAuthURL, defaultInstagramTokenURL))
}

func (p instagramProvider) ConnectorID() string {
	return "instagram"
}

func (p instagramProvider) RequiresPKCE() bool {
	return false
}

func (p instagramProvider) RequiredExtraKeys() []string {
	return nil
}

func (p instagramProvider) BuildAuthURL(_ context.Context, req AuthRequest) (string, error) {
	authURL, params, err := withCommonAuthParams(p.authURL, req)
	if err != nil {
		return "", err
	}
	params.Set("response_type", "code")
	authURL.RawQuery = params.Encode()
	return authURL.String(), nil
}

func (p instagramProvider) ExchangeToken(ctx context.Context, httpClient *http.Client, req TokenRequest) ([]byte, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", req.Code)
	form.Set("redirect_uri", req.RedirectURI)
	form.Set("client_id", req.ClientID)
	form.Set("client_secret", req.ClientSecret)
	return PostForm(ctx, httpClient, p.tokenURL, form, "", "")
}
