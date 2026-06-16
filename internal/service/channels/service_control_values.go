package channels

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func validateChannelConfigInput(
	catalog ChannelCatalogItem,
	publicConfig map[string]string,
	secrets map[string]string,
	hasExistingCredentials bool,
) error {
	for _, field := range catalog.CredentialFields {
		if !field.Required {
			continue
		}
		if field.Secret {
			if strings.TrimSpace(secrets[field.Key]) == "" && !hasExistingCredentials {
				return fmt.Errorf("%s is required", field.Key)
			}
			continue
		}
		if strings.TrimSpace(publicConfig[field.Key]) == "" {
			return fmt.Errorf("%s is required", field.Key)
		}
	}
	return nil
}

func normalizeIMChannelType(channelType string) string {
	return protocol.NormalizeStoredChannelType(channelType)
}

func normalizeChannelConfigStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case ChannelConfigStatusConfigured,
		ChannelConfigStatusConnected,
		ChannelConfigStatusPending,
		ChannelConfigStatusError,
		ChannelConfigStatusDisabled:
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return ""
	}
}

func normalizeChannelOwnerUserID(ownerUserID string) string {
	if strings.TrimSpace(ownerUserID) == "" {
		return authctx.SystemUserID
	}
	return strings.TrimSpace(ownerUserID)
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
	switch strings.ToLower(strings.TrimSpace(value)) {
	case PairingStatusPending, PairingStatusActive, PairingStatusDisabled, PairingStatusRejected:
		return strings.ToLower(strings.TrimSpace(value))
	case "":
		return strings.TrimSpace(fallback)
	default:
		return ""
	}
}

func normalizePairingSource(value string, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case PairingSourceManual, PairingSourceIngress, PairingSourceWeChatQR:
		return strings.ToLower(strings.TrimSpace(value))
	case "":
		return strings.TrimSpace(fallback)
	default:
		return ""
	}
}

func nullStringValueOrNil(value sql.NullString) any {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil
	}
	return strings.TrimSpace(value.String)
}

func nullTimeValueOrNil(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time
}
