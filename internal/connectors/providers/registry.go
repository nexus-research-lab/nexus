package providers

import (
	"cmp"
	"errors"
	"os"
)

var registry = map[string]Provider{}

type providerOverrideFactory func() Provider

var providerOverrideFactories = map[string]providerOverrideFactory{
	"github":      githubProviderOverride,
	"gmail":       gmailProviderOverride,
	"linkedin":    linkedinProviderOverride,
	"x-twitter":   twitterProviderOverride,
	"instagram":   instagramProviderOverride,
	"feishu-docx": feishuDocxProviderOverride,
}

// Register 在 init 时注册 Provider。
func Register(p Provider) {
	registry[p.ConnectorID()] = p
}

// Get 按 connector_id 查询 Provider。
func Get(connectorID string) (Provider, error) {
	if factory := providerOverrideFactories[connectorID]; factory != nil {
		if provider := factory(); provider != nil {
			return provider, nil
		}
	}
	p, ok := registry[connectorID]
	if !ok {
		return nil, errors.New("connector provider not registered: " + connectorID)
	}
	return p, nil
}

func githubProviderOverride() Provider {
	tokenURL := os.Getenv("NEXUS_CONNECTOR_GITHUB_TOKEN_URL")
	deviceCodeURL := os.Getenv("NEXUS_CONNECTOR_GITHUB_DEVICE_CODE_URL")
	if tokenURL == "" && deviceCodeURL == "" {
		return nil
	}
	return NewGitHubProviderWithDeviceURL(
		defaultGitHubAuthURL,
		cmp.Or(tokenURL, defaultGitHubTokenURL),
		cmp.Or(deviceCodeURL, defaultGitHubDeviceCodeURL),
	)
}

func gmailProviderOverride() Provider {
	if tokenURL := os.Getenv("NEXUS_CONNECTOR_GOOGLE_TOKEN_URL"); tokenURL != "" {
		return NewGoogleProvider(defaultGoogleAuthURL, tokenURL)
	}
	return nil
}

func linkedinProviderOverride() Provider {
	if tokenURL := os.Getenv("NEXUS_CONNECTOR_LINKEDIN_TOKEN_URL"); tokenURL != "" {
		return NewLinkedInProvider(defaultLinkedInAuthURL, tokenURL)
	}
	return nil
}

func twitterProviderOverride() Provider {
	if tokenURL := os.Getenv("NEXUS_CONNECTOR_TWITTER_TOKEN_URL"); tokenURL != "" {
		return NewTwitterProvider(defaultTwitterAuthURL, tokenURL)
	}
	return nil
}

func instagramProviderOverride() Provider {
	if tokenURL := os.Getenv("NEXUS_CONNECTOR_INSTAGRAM_TOKEN_URL"); tokenURL != "" {
		return NewInstagramProvider(defaultInstagramAuthURL, tokenURL)
	}
	return nil
}

func feishuDocxProviderOverride() Provider {
	authURL := os.Getenv("NEXUS_CONNECTOR_FEISHU_DOCX_AUTH_URL")
	tokenURL := os.Getenv("NEXUS_CONNECTOR_FEISHU_DOCX_TOKEN_URL")
	apiURL := os.Getenv("NEXUS_CONNECTOR_FEISHU_DOCX_API_BASE_URL")
	if authURL == "" && tokenURL == "" && apiURL == "" {
		return nil
	}
	return NewFeishuDocxProvider(
		cmp.Or(authURL, defaultFeishuDocxAuthURL),
		cmp.Or(tokenURL, defaultFeishuDocxTokenURL),
		cmp.Or(apiURL, defaultFeishuDocxAPIURL),
	)
}
