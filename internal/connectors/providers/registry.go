package providers

import (
	"errors"
	"os"
)

var registry = map[string]Provider{}

// Register 在 init 时注册 Provider。
func Register(p Provider) {
	registry[p.ConnectorID()] = p
}

// Get 按 connector_id 查询 Provider。
func Get(connectorID string) (Provider, error) {
	switch connectorID {
	case "github":
		tokenURL := os.Getenv("NEXUS_CONNECTOR_GITHUB_TOKEN_URL")
		deviceCodeURL := os.Getenv("NEXUS_CONNECTOR_GITHUB_DEVICE_CODE_URL")
		if tokenURL != "" || deviceCodeURL != "" {
			return NewGitHubProviderWithDeviceURL(
				defaultGitHubAuthURL,
				firstNonEmpty(tokenURL, defaultGitHubTokenURL),
				firstNonEmpty(deviceCodeURL, defaultGitHubDeviceCodeURL),
			), nil
		}
	case "gmail":
		if tokenURL := os.Getenv("NEXUS_CONNECTOR_GOOGLE_TOKEN_URL"); tokenURL != "" {
			return NewGoogleProvider(defaultGoogleAuthURL, tokenURL), nil
		}
	case "linkedin":
		if tokenURL := os.Getenv("NEXUS_CONNECTOR_LINKEDIN_TOKEN_URL"); tokenURL != "" {
			return NewLinkedInProvider(defaultLinkedInAuthURL, tokenURL), nil
		}
	case "x-twitter":
		if tokenURL := os.Getenv("NEXUS_CONNECTOR_TWITTER_TOKEN_URL"); tokenURL != "" {
			return NewTwitterProvider(defaultTwitterAuthURL, tokenURL), nil
		}
	case "instagram":
		if tokenURL := os.Getenv("NEXUS_CONNECTOR_INSTAGRAM_TOKEN_URL"); tokenURL != "" {
			return NewInstagramProvider(defaultInstagramAuthURL, tokenURL), nil
		}
	case "feishu-docx":
		authURL := os.Getenv("NEXUS_CONNECTOR_FEISHU_DOCX_AUTH_URL")
		tokenURL := os.Getenv("NEXUS_CONNECTOR_FEISHU_DOCX_TOKEN_URL")
		apiURL := os.Getenv("NEXUS_CONNECTOR_FEISHU_DOCX_API_BASE_URL")
		if authURL != "" || tokenURL != "" || apiURL != "" {
			return NewFeishuDocxProvider(
				firstNonEmpty(authURL, defaultFeishuDocxAuthURL),
				firstNonEmpty(tokenURL, defaultFeishuDocxTokenURL),
				firstNonEmpty(apiURL, defaultFeishuDocxAPIURL),
			), nil
		}
	}
	p, ok := registry[connectorID]
	if !ok {
		return nil, errors.New("connector provider not registered: " + connectorID)
	}
	return p, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
