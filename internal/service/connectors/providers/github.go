package providers

import (
	"context"
	"net/http"
	"net/url"
)

const (
	defaultGitHubAuthURL  = "https://github.com/login/oauth/authorize"
	defaultGitHubTokenURL = "https://github.com/login/oauth/access_token"
)

type githubProvider struct {
	authURL  string
	tokenURL string
}

// NewGitHubProvider 创建 GitHub Provider，GitHub 会在 200 JSON 中返回 error 字段。
func NewGitHubProvider(authURL string, tokenURL string) Provider {
	return githubProvider{authURL: authURL, tokenURL: tokenURL}
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
