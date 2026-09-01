// INPUT: 可能包含 token、密码、认证 header、内部 prompt 的任意 JSON 可编码配置值。
// OUTPUT: 保留结构和 configured 状态、移除明文敏感值的安全投影。
// POS: 配置读取、版本计算与审计写入共用的唯一脱敏入口。
package configuration

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strings"
)

var exactSensitiveKeys = map[string]struct{}{
	"access_token": {}, "refresh_token": {}, "auth_token": {}, "api_key": {},
	"client_secret": {}, "password": {}, "passphrase": {}, "private_key": {},
	"secret": {}, "token": {}, "authorization": {}, "cookie": {},
	"credentials": {}, "credentials_encrypted": {},
	"session_id": {}, "sdk_session_id": {}, "resume_id": {},
	// 自定义 MCP / Provider options 允许自由形状，不能依赖子键名猜测秘密。
	"mcp_servers": {}, "provider_options": {},
	"base_system_prompt": {}, "main_agent_system_prompt": {},
}

func sanitizeValue(value any) any {
	payload, err := json.Marshal(value)
	if err != nil {
		return map[string]any{"redacted": true, "encoding_error": err.Error()}
	}
	var decoded any
	if err = json.Unmarshal(payload, &decoded); err != nil {
		return map[string]any{"redacted": true, "encoding_error": err.Error()}
	}
	return sanitizeNode(decoded, "")
}

func sanitizeNode(value any, key string) any {
	if isSensitiveKey(key) {
		return secretPresence(value)
	}
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for childKey, childValue := range typed {
			result[childKey] = sanitizeNode(childValue, childKey)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = sanitizeNode(child, key)
		}
		return result
	case string:
		if isURLKey(key) {
			return sanitizeURL(typed)
		}
		return typed
	default:
		return value
	}
}

func isURLKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "_", ""))
	return strings.Contains(normalized, "url") || strings.Contains(normalized, "uri")
}

func sanitizeURL(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" {
		return value
	}
	if parsed.User != nil {
		parsed.User = url.User("[redacted]")
	}
	query := parsed.Query()
	for key := range query {
		normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"))
		switch {
		case isSensitiveKey(normalized),
			normalized == "key",
			normalized == "code",
			normalized == "sig",
			normalized == "signature",
			strings.Contains(normalized, "credential"):
			query.Set(key, "[redacted]")
		}
	}
	parsed.RawQuery = query.Encode()
	if parsed.Fragment != "" {
		parsed.Fragment = "[redacted]"
	}
	return parsed.String()
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	if _, ok := exactSensitiveKeys[normalized]; ok {
		return true
	}
	compacted := strings.ReplaceAll(normalized, "_", "")
	for _, fragment := range []string{
		"accesstoken", "refreshtoken", "authtoken", "apikey", "clientsecret",
		"password", "passphrase", "privatekey", "systemprompt", "credentialsencrypted",
		"databaseurl",
	} {
		if strings.Contains(compacted, fragment) {
			return true
		}
	}
	for _, suffix := range []string{
		"sessiontoken", "bottoken", "credentialkey", "credentialskey",
		"credentialkeys", "credentialskeys",
		"clientsecret", "privatekey", "systemprompt", "databaseurl",
	} {
		if strings.HasSuffix(compacted, suffix) {
			return true
		}
	}
	if strings.Contains(compacted, "credential") && strings.HasSuffix(compacted, "keys") {
		return true
	}
	for _, suffix := range []string{"_token", "_secret", "_password", "_api_key", "_private_key"} {
		if strings.HasSuffix(normalized, suffix) {
			return true
		}
	}
	return strings.Contains(normalized, "system_prompt")
}

func secretPresence(value any) map[string]any {
	configured := false
	switch typed := value.(type) {
	case string:
		configured = strings.TrimSpace(typed) != ""
	case nil:
		configured = false
	default:
		configured = true
	}
	return map[string]any{"configured": configured, "redacted": true}
}

func revisionFor(value any) (string, error) {
	payload, err := json.Marshal(sanitizeValue(value))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func integrityRevisionFor(value any, key []byte) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(payload)
	return "hmac-sha256:" + hex.EncodeToString(mac.Sum(nil)), nil
}

func sanitizedJSON(value any) json.RawMessage {
	payload, err := json.Marshal(sanitizeValue(value))
	if err != nil {
		return json.RawMessage(`{"redacted":true}`)
	}
	return payload
}
