// INPUT: Preferences JSON 与规范化后的 WebSearch provider/api_key。
// OUTPUT: Preferences 解码结果、凭据存储模型与无副作用的变更比较。
// POS: Preferences 持久化格式层；不打开文件，也不绕过 confinedfs 事务边界。
package preferences

import (
	"encoding/json"
	"strconv"
	"strings"
)

// storedWebSearchCredential 是 WebSearch 凭据文件的唯一存储格式。
// provider 与 api_key 必须成对存在，避免不同 provider 复用同一份密钥。
type storedWebSearchCredential struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
}

// storedWebSearchCredentialBundle 在 Preferences 跨文件提交期间同时保留旧、新
// version 的凭据。Preferences.version 是发布指针，读取方只接受精确匹配项。
// 顶层 provider/api_key 仅用于读取升级前的单凭据格式。
type storedWebSearchCredentialBundle struct {
	Versions map[string]storedWebSearchCredential `json:"versions,omitempty"`
	Provider string                               `json:"provider,omitempty"`
	APIKey   string                               `json:"api_key,omitempty"`
}

func decodeWebSearchCredentialBundle(content []byte) storedWebSearchCredentialBundle {
	var bundle storedWebSearchCredentialBundle
	if err := json.Unmarshal(content, &bundle); err != nil {
		return storedWebSearchCredentialBundle{}
	}
	return normalizeWebSearchCredentialBundle(bundle)
}

func normalizeWebSearchCredentialBundle(
	bundle storedWebSearchCredentialBundle,
) storedWebSearchCredentialBundle {
	result := storedWebSearchCredentialBundle{
		Provider: strings.ToLower(strings.TrimSpace(bundle.Provider)),
		APIKey:   strings.TrimSpace(bundle.APIKey),
	}
	if result.Provider == "" || result.APIKey == "" {
		result.Provider = ""
		result.APIKey = ""
	}
	for version, credential := range bundle.Versions {
		parsed, err := strconv.ParseInt(strings.TrimSpace(version), 10, 64)
		if err != nil || parsed < 1 {
			continue
		}
		credential.Provider = strings.ToLower(strings.TrimSpace(credential.Provider))
		credential.APIKey = strings.TrimSpace(credential.APIKey)
		if credential.Provider == "" || credential.APIKey == "" {
			continue
		}
		if result.Versions == nil {
			result.Versions = make(map[string]storedWebSearchCredential)
		}
		result.Versions[strconv.FormatInt(parsed, 10)] = credential
	}
	return result
}

func (bundle storedWebSearchCredentialBundle) credentialForVersion(
	version int64,
) storedWebSearchCredential {
	normalized := normalizeWebSearchCredentialBundle(bundle)
	if credential, ok := normalized.Versions[strconv.FormatInt(version, 10)]; ok {
		return credential
	}
	// 升级前的凭据没有 version；首次成功写入会把它转换为版本化格式。
	if normalized.Provider != "" && normalized.APIKey != "" {
		return storedWebSearchCredential{
			Provider: normalized.Provider,
			APIKey:   normalized.APIKey,
		}
	}
	return storedWebSearchCredential{}
}

func credentialBundleForTransition(
	previous Preferences,
	next Preferences,
) storedWebSearchCredentialBundle {
	bundle := storedWebSearchCredentialBundle{}
	addCredentialVersion(&bundle, previous)
	addCredentialVersion(&bundle, next)
	return bundle
}

func credentialBundleForCurrent(current Preferences) storedWebSearchCredentialBundle {
	bundle := storedWebSearchCredentialBundle{}
	addCredentialVersion(&bundle, current)
	return bundle
}

func addCredentialVersion(
	bundle *storedWebSearchCredentialBundle,
	preferences Preferences,
) {
	provider := strings.ToLower(strings.TrimSpace(preferences.WebSearch.Provider))
	apiKey := strings.TrimSpace(preferences.WebSearchAPIKey())
	if preferences.Version < 1 || provider == "" || apiKey == "" {
		return
	}
	if bundle.Versions == nil {
		bundle.Versions = make(map[string]storedWebSearchCredential)
	}
	bundle.Versions[strconv.FormatInt(preferences.Version, 10)] = storedWebSearchCredential{
		Provider: provider,
		APIKey:   apiKey,
	}
}

func credentialBundleEmpty(bundle storedWebSearchCredentialBundle) bool {
	normalized := normalizeWebSearchCredentialBundle(bundle)
	return len(normalized.Versions) == 0 &&
		normalized.Provider == "" &&
		normalized.APIKey == ""
}

func decodePreferences(content []byte) (Preferences, error) {
	var item Preferences
	if err := json.Unmarshal(content, &item); err != nil {
		return Preferences{}, err
	}
	normalized := normalizePreferences(item)
	if normalized.UpdatedAt == "" {
		normalized.UpdatedAt = nowRFC3339()
	}
	return normalized, nil
}
