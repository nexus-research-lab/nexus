package connectors

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func credentialNeedsRefresh(credentials map[string]string) bool {
	expiresAtRaw := strings.TrimSpace(credentials["expires_at"])
	if expiresAtRaw == "" {
		return false
	}
	expiresAt, err := strconv.ParseFloat(expiresAtRaw, 64)
	if err != nil {
		return false
	}
	return time.Unix(int64(expiresAt), 0).Before(time.Now().Add(5 * time.Minute))
}

func normalizeOAuthPayload(payload []byte) string {
	normalized, err := credentialMapFromPayload(payload)
	if err != nil {
		return string(payload)
	}
	addCredentialExpiresAt(normalized, "expires_in", "expires_at")
	addCredentialExpiresAt(normalized, "refresh_expires_in", "refresh_expires_at")
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return string(payload)
	}
	return string(encoded)
}

func credentialMapFromPayload(payload []byte) (map[string]string, error) {
	if json.Valid(payload) {
		var raw map[string]any
		if err := json.Unmarshal(payload, &raw); err != nil {
			return nil, err
		}
		if data, ok := raw["data"].(map[string]any); ok {
			raw = data
		}
		normalized := map[string]string{}
		for key, value := range raw {
			if key == "" || value == nil {
				continue
			}
			normalized[key] = credentialScalarString(value)
		}
		return normalized, nil
	}
	values, err := url.ParseQuery(string(payload))
	if err != nil {
		return nil, err
	}
	normalized := map[string]string{}
	for key, value := range values {
		normalized[key] = strings.Join(value, ",")
	}
	return normalized, nil
}

func credentialScalarString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case json.Number:
		return typed.String()
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(encoded)
	}
}

func addCredentialExpiresAt(credentials map[string]string, durationKey string, targetKey string) {
	if strings.TrimSpace(credentials[targetKey]) != "" {
		return
	}
	expiresInRaw := strings.TrimSpace(credentials[durationKey])
	if expiresInRaw == "" {
		return
	}
	expiresIn, err := strconv.ParseFloat(expiresInRaw, 64)
	if err != nil || expiresIn <= 0 {
		return
	}
	credentials[targetKey] = strconv.FormatInt(time.Now().Add(time.Duration(expiresIn)*time.Second).Unix(), 10)
}

func mergeCredentialExtras(credentials string, extra map[string]string) string {
	if len(extra) == 0 || !json.Valid([]byte(credentials)) {
		return credentials
	}
	parsed := map[string]string{}
	if err := json.Unmarshal([]byte(credentials), &parsed); err != nil {
		return credentials
	}
	for key, value := range extra {
		if strings.TrimSpace(value) == "" {
			continue
		}
		parsed[key] = value
	}
	encoded, err := json.Marshal(parsed)
	if err != nil {
		return credentials
	}
	return string(encoded)
}

func normalizeExtras(extras map[string]string) map[string]string {
	normalized := map[string]string{}
	for key, value := range extras {
		normalized[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return normalized
}

func connectorFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
