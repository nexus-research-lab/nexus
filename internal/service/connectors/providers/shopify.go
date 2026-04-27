package providers

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

var shopifyShopPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]*$`)

type shopifyProvider struct{}

// NewShopifyProvider 创建 Shopify Provider，Shopify 需要 shop 模板替换。
func NewShopifyProvider() Provider {
	return shopifyProvider{}
}

func init() {
	Register(NewShopifyProvider())
}

func (p shopifyProvider) ConnectorID() string {
	return "shopify"
}

func (p shopifyProvider) APIBaseURL() string {
	return "https://{shop}.myshopify.com/admin/api/2024-07"
}

func (p shopifyProvider) RequiresPKCE() bool {
	return false
}

func (p shopifyProvider) RequiredExtraKeys() []string {
	return []string{"shop"}
}

func (p shopifyProvider) BuildAuthURL(_ context.Context, req AuthRequest) (string, error) {
	shop, err := validateShop(req.Extra)
	if err != nil {
		return "", err
	}
	authURL, err := url.Parse("https://" + shop + ".myshopify.com/admin/oauth/authorize")
	if err != nil {
		return "", err
	}
	params := authURL.Query()
	params.Set("client_id", req.ClientID)
	params.Set("scope", strings.Join(req.Scopes, ","))
	params.Set("redirect_uri", req.RedirectURI)
	params.Set("state", req.State)
	authURL.RawQuery = params.Encode()
	return authURL.String(), nil
}

func (p shopifyProvider) ExchangeToken(ctx context.Context, httpClient *http.Client, req TokenRequest) ([]byte, error) {
	shop, err := validateShop(req.Extra)
	if err != nil {
		return nil, err
	}
	form := url.Values{}
	form.Set("client_id", req.ClientID)
	form.Set("client_secret", req.ClientSecret)
	form.Set("code", req.Code)
	return PostForm(ctx, httpClient, "https://"+shop+".myshopify.com/admin/oauth/access_token", form, "", "")
}

func validateShop(extra map[string]string) (string, error) {
	shop := strings.TrimSpace(extra["shop"])
	if parsed, err := url.Parse(shop); err == nil && parsed.Host != "" {
		shop = parsed.Host
	}
	shop = strings.TrimSuffix(shop, "/admin")
	shop = strings.TrimSuffix(strings.ToLower(shop), ".myshopify.com")
	if shop == "" {
		return "", errors.New("shop 参数缺失")
	}
	if !shopifyShopPattern.MatchString(shop) {
		return "", errors.New("shop 参数格式不正确")
	}
	return shop, nil
}
