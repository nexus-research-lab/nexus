package providers

import (
	"context"
	"net/http"
	"net/url"
)

const (
	defaultLinkedInAuthURL  = "https://www.linkedin.com/oauth/v2/authorization"
	defaultLinkedInTokenURL = "https://www.linkedin.com/oauth/v2/accessToken"
)

type linkedInProvider struct {
	authURL  string
	tokenURL string
}

// NewLinkedInProvider 创建 LinkedIn Provider，使用 OIDC scope 和 PKCE。
func NewLinkedInProvider(authURL string, tokenURL string) Provider {
	return linkedInProvider{authURL: authURL, tokenURL: tokenURL}
}

func init() {
	Register(NewLinkedInProvider(defaultLinkedInAuthURL, defaultLinkedInTokenURL))
}

func (p linkedInProvider) ConnectorID() string {
	return "linkedin"
}

func (p linkedInProvider) RequiresPKCE() bool {
	return true
}

func (p linkedInProvider) RequiredExtraKeys() []string {
	return nil
}

func (p linkedInProvider) BuildAuthURL(_ context.Context, req AuthRequest) (string, error) {
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

func (p linkedInProvider) ExchangeToken(ctx context.Context, httpClient *http.Client, req TokenRequest) ([]byte, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", req.Code)
	form.Set("redirect_uri", req.RedirectURI)
	form.Set("client_id", req.ClientID)
	form.Set("client_secret", req.ClientSecret)
	form.Set("code_verifier", req.CodeVerifier)
	return PostForm(ctx, httpClient, p.tokenURL, form, "", "")
}
