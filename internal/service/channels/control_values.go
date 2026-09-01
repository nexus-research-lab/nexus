package channels

import (
	"cmp"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
)

func validateChannelConfigInput(
	catalog ChannelCatalogItem,
	publicConfig map[string]string,
	secrets map[string]string,
	hasExistingCredentials bool,
) error {
	for _, field := range catalog.CredentialFields {
		if !field.Secret {
			continue
		}
		if _, present := publicConfig[field.Key]; present {
			return invalidChannelControl(fmt.Errorf(
				"%s is secret and must be supplied through the credentials channel",
				field.Key,
			))
		}
	}
	if publicKey, secretKey, ok := channelManualCredentialPair(catalog.ChannelType); ok {
		hasPublic := strings.TrimSpace(publicConfig[publicKey]) != ""
		hasSecret := strings.TrimSpace(secrets[secretKey]) != "" || hasExistingCredentials
		if hasPublic && !hasSecret {
			return invalidChannelControl(fmt.Errorf("%s is required", secretKey))
		}
		if !hasPublic && len(secrets) > 0 {
			return invalidChannelControl(fmt.Errorf("%s is required", publicKey))
		}
	}
	for _, field := range catalog.CredentialFields {
		if !field.Required {
			continue
		}
		if field.Secret {
			if strings.TrimSpace(secrets[field.Key]) == "" && !hasExistingCredentials {
				return invalidChannelControl(fmt.Errorf("%s is required", field.Key))
			}
			continue
		}
		if strings.TrimSpace(publicConfig[field.Key]) == "" {
			return invalidChannelControl(fmt.Errorf("%s is required", field.Key))
		}
	}
	return nil
}

func channelManualCredentialPair(channelType string) (string, string, bool) {
	switch normalizeIMChannelType(channelType) {
	case ChannelTypeFeishu:
		return "app_id", "app_secret", true
	case ChannelTypeDingTalk:
		return "client_id", "client_secret", true
	case ChannelTypeWeChat:
		return "bot_id", "secret", true
	default:
		return "", "", false
	}
}

func normalizeIMChannelType(channelType string) string {
	return protocol.NormalizeStoredChannelType(channelType)
}

func normalizeChannelConfigStatus(status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch normalized {
	case ChannelConfigStatusConfigured,
		ChannelConfigStatusConnected,
		ChannelConfigStatusPending,
		ChannelConfigStatusError,
		ChannelConfigStatusDisabled:
		return normalized
	default:
		return ""
	}
}

func normalizeChannelOwnerUserID(ownerUserID string) string {
	return cmp.Or(strings.TrimSpace(ownerUserID), authctx.SystemUserID)
}

func nullableString(value string) any {
	return channelcontract.NullableString(value)
}

func nullStringValue(value sql.NullString) string {
	return channelcontract.NullStringValue(value)
}

func firstNonEmpty(values ...string) string {
	return channelcontract.FirstNonEmpty(values...)
}

func normalizeStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		result[normalizedKey] = strings.TrimSpace(value)
	}
	return result
}

func publicChannelConfigForView(channelType string, values map[string]string) map[string]string {
	result := normalizeStringMap(values)
	if catalog, ok := channelCatalogByType(channelType); ok {
		for _, field := range catalog.CredentialFields {
			if field.Secret {
				delete(result, field.Key)
			}
		}
	}
	if normalizeIMChannelType(channelType) == ChannelTypeWeixinPersonal {
		delete(result, "account_id")
		delete(result, "user_id")
	}
	return result
}

func encodeStringMap(values map[string]string) (string, error) {
	if values == nil {
		values = map[string]string{}
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func decodeStringMap(raw string) (map[string]string, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]string{}, nil
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, err
	}
	return normalizeStringMap(result), nil
}

func normalizePairingStatus(value string, fallback string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case PairingStatusPending, PairingStatusActive, PairingStatusDisabled, PairingStatusRejected:
		return normalized
	case "":
		return strings.TrimSpace(fallback)
	default:
		return ""
	}
}

func normalizePairingSource(value string, fallback string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case PairingSourceManual, PairingSourceIngress, PairingSourceWeChatQR:
		return normalized
	case "":
		return strings.TrimSpace(fallback)
	default:
		return ""
	}
}

func nullStringValueOrNil(value sql.NullString) any {
	trimmed := strings.TrimSpace(value.String)
	if !value.Valid || trimmed == "" {
		return nil
	}
	return trimmed
}

func nullTimeValueOrNil(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time
}
