// INPUT: model-produced configuration templates and human-only secret values.
// OUTPUT: validated opaque slots, non-secret validation payloads, or one-time materialized JSON.
// POS: shared low-level boundary used before transcripts, permission events, plans, and writes.
package secretinput

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

const (
	placeholderKey   = "$secret"
	validationSecret = "__nexus_human_secret__"
	maxSlots         = 32
	maxDepth         = 24
	maxSecretBytes   = 64 << 10
	maxTotalBytes    = 256 << 10
)

var slotIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,63}$`)

// Slot is safe model/UI metadata. ID and Path identify a field, never its value.
type Slot struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

// PrepareJSON rejects direct secret material and replaces valid placeholders
// with non-empty sentinels so existing domain validators can inspect shape.
func PrepareJSON(input json.RawMessage) (json.RawMessage, []Slot, error) {
	if len(input) == 0 {
		input = json.RawMessage(`{}`)
	}
	var value any
	if err := json.Unmarshal(input, &value); err != nil {
		return nil, nil, fmt.Errorf("解析配置 secret template: %w", err)
	}
	slots := make([]Slot, 0)
	seen := make(map[string]string)
	prepared, err := prepareNode(value, "", "", false, 0, &slots, seen)
	if err != nil {
		return nil, nil, err
	}
	payload, err := json.Marshal(prepared)
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].Path < slots[j].Path })
	return payload, slots, nil
}

// MaterializeJSON resolves every declared slot exactly once from a human-only
// value map. Extra, missing, or empty values fail closed.
func MaterializeJSON(input json.RawMessage, values map[string]string) (json.RawMessage, error) {
	_, slots, err := PrepareJSON(input)
	if err != nil {
		return nil, err
	}
	if len(slots) == 0 {
		if len(values) != 0 {
			return nil, errors.New("配置计划未声明 secret slot")
		}
		return append(json.RawMessage(nil), input...), nil
	}
	if len(values) != len(slots) {
		return nil, errors.New("人类输入的 secret slot 数量与配置计划不匹配")
	}
	total := 0
	for _, slot := range slots {
		value, ok := values[slot.ID]
		if !ok || strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("secret slot %s 缺少人类输入", slot.ID)
		}
		if len(value) > maxSecretBytes {
			return nil, fmt.Errorf("secret slot %s 超过大小限制", slot.ID)
		}
		total += len(value)
	}
	if total > maxTotalBytes {
		return nil, errors.New("配置 secret 总大小超过限制")
	}
	for key := range values {
		found := false
		for _, slot := range slots {
			if key == slot.ID {
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("人类输入包含未声明的 secret slot")
		}
	}
	var value any
	if err = json.Unmarshal(input, &value); err != nil {
		return nil, err
	}
	materialized, err := materializeNode(value, values, 0)
	if err != nil {
		return nil, err
	}
	return json.Marshal(materialized)
}

// SlotsFromToolInput extracts safe slot metadata from plan/apply arguments.
func SlotsFromToolInput(toolName string, input map[string]any) []Slot {
	if !isConfigurationChangeTool(toolName) || input == nil {
		return nil
	}
	payload, err := json.Marshal(input["input"])
	if err != nil {
		return nil
	}
	_, slots, err := PrepareJSON(payload)
	if err != nil {
		return nil
	}
	return slots
}

// RedactConfigurationToolInput prevents a malformed/direct secret from
// entering permission events or durable assistant messages. Valid placeholders
// remain visible so the native UI can render the exact human-only fields.
func RedactConfigurationToolInput(toolName string, input map[string]any) map[string]any {
	cloned := cloneMap(input)
	if !isConfigurationChangeTool(toolName) {
		return cloned
	}
	cloned["input"] = redactNode(cloned["input"], "", false, 0)
	return cloned
}

func isConfigurationChangeTool(toolName string) bool {
	toolName = strings.TrimSpace(toolName)
	for _, leaf := range []string{
		"plan_nexus_configuration_change",
		"apply_nexus_configuration_change",
	} {
		if toolName == leaf {
			return true
		}
		for _, separator := range []string{"__", ".", "/"} {
			if strings.HasSuffix(toolName, separator+leaf) {
				return true
			}
		}
	}
	return false
}

func prepareNode(
	value any,
	key string,
	path string,
	insideSecretContainer bool,
	depth int,
	slots *[]Slot,
	seen map[string]string,
) (any, error) {
	if depth > maxDepth {
		return nil, errors.New("配置 secret template 嵌套过深")
	}
	if slotID, ok := placeholderID(value); ok {
		if !insideSecretContainer && !IsSensitiveKey(key) {
			return nil, fmt.Errorf("非敏感字段 %s 不能使用 $secret placeholder", displayPath(path))
		}
		if !slotIDPattern.MatchString(slotID) {
			return nil, fmt.Errorf("secret slot %s 必须为 8-64 位安全标识", displayPath(path))
		}
		if previous, exists := seen[slotID]; exists && previous != path {
			return nil, fmt.Errorf("secret slot %q 被多个字段复用", slotID)
		}
		if len(*slots) >= maxSlots {
			return nil, errors.New("一次配置变更最多包含 32 个 secret slot")
		}
		seen[slotID] = path
		*slots = append(*slots, Slot{ID: slotID, Path: displayPath(path)})
		return validationSecret, nil
	}
	switch typed := value.(type) {
	case map[string]any:
		if IsSensitiveKey(key) && !isSecretValueContainerKey(key) && !insideSecretContainer {
			return nil, fmt.Errorf(
				"敏感字段 %s 必须使用 {\"$secret\":\"opaque_slot_id\"}，不得把明文交给智能体",
				displayPath(path),
			)
		}
		childrenSensitive := insideSecretContainer || isSecretValueContainerKey(key)
		result := make(map[string]any, len(typed))
		for childKey, child := range typed {
			childPath := joinPath(path, childKey)
			prepared, err := prepareNode(
				child,
				childKey,
				childPath,
				childrenSensitive,
				depth+1,
				slots,
				seen,
			)
			if err != nil {
				return nil, err
			}
			result[childKey] = prepared
		}
		return result, nil
	case []any:
		if IsSensitiveKey(key) && !insideSecretContainer {
			return nil, fmt.Errorf(
				"敏感字段 %s 必须使用 {\"$secret\":\"opaque_slot_id\"}，不得把明文交给智能体",
				displayPath(path),
			)
		}
		childrenSensitive := insideSecretContainer || isSecretValueContainerKey(key)
		result := make([]any, len(typed))
		for index, child := range typed {
			prepared, err := prepareNode(
				child,
				key,
				fmt.Sprintf("%s[%d]", path, index),
				childrenSensitive,
				depth+1,
				slots,
				seen,
			)
			if err != nil {
				return nil, err
			}
			result[index] = prepared
		}
		return result, nil
	default:
		if insideSecretContainer || isSecretValueContainerKey(key) || IsSensitiveKey(key) {
			return nil, fmt.Errorf(
				"敏感字段 %s 必须使用 {\"$secret\":\"opaque_slot_id\"}，不得把明文交给智能体",
				displayPath(path),
			)
		}
		if text, ok := value.(string); ok && URLContainsSecret(text) {
			return nil, fmt.Errorf("URL 字段 %s 含凭据或敏感 query；请改用独立 secret slot", displayPath(path))
		}
		return value, nil
	}
}

func materializeNode(value any, values map[string]string, depth int) (any, error) {
	if depth > maxDepth {
		return nil, errors.New("配置 secret template 嵌套过深")
	}
	if slotID, ok := placeholderID(value); ok {
		secret, exists := values[slotID]
		if !exists {
			return nil, fmt.Errorf("secret slot %q 缺失", slotID)
		}
		return secret, nil
	}
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			next, err := materializeNode(child, values, depth+1)
			if err != nil {
				return nil, err
			}
			result[key] = next
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			next, err := materializeNode(child, values, depth+1)
			if err != nil {
				return nil, err
			}
			result[index] = next
		}
		return result, nil
	default:
		return value, nil
	}
}

func placeholderID(value any) (string, bool) {
	object, ok := value.(map[string]any)
	if !ok || len(object) != 1 {
		return "", false
	}
	raw, exists := object[placeholderKey]
	if !exists {
		return "", false
	}
	slotID, ok := raw.(string)
	return strings.TrimSpace(slotID), ok
}

func redactNode(value any, key string, insideSecretContainer bool, depth int) any {
	if depth > maxDepth {
		return map[string]any{"redacted": true}
	}
	if slotID, ok := placeholderID(value); ok && slotIDPattern.MatchString(slotID) {
		return map[string]any{placeholderKey: slotID}
	}
	switch typed := value.(type) {
	case map[string]any:
		if IsSensitiveKey(key) && !isSecretValueContainerKey(key) && !insideSecretContainer {
			return redactedSecretPresence(value)
		}
		childrenSensitive := insideSecretContainer || isSecretValueContainerKey(key)
		result := make(map[string]any, len(typed))
		for childKey, child := range typed {
			result[childKey] = redactNode(child, childKey, childrenSensitive, depth+1)
		}
		return result
	case []any:
		if IsSensitiveKey(key) && !insideSecretContainer {
			return redactedSecretPresence(value)
		}
		childrenSensitive := insideSecretContainer || isSecretValueContainerKey(key)
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = redactNode(child, key, childrenSensitive, depth+1)
		}
		return result
	default:
		if insideSecretContainer || isSecretValueContainerKey(key) || IsSensitiveKey(key) {
			return map[string]any{"configured": secretConfigured(value), "redacted": true}
		}
		return value
	}
}

// IsSensitiveKey identifies a field whose own value is secret. Extensible
// configuration objects such as mcp_servers and provider_options are not
// secret as a whole: their structural values must retain their JSON types,
// while explicitly sensitive leaves are replaced with opaque slots.
func IsSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	exact := map[string]struct{}{
		"access_token": {}, "refresh_token": {}, "auth_token": {}, "api_key": {},
		"client_secret": {}, "password": {}, "passphrase": {}, "private_key": {},
		"secret": {}, "token": {}, "authorization": {}, "cookie": {},
		"credentials_encrypted": {},
		"base_system_prompt":    {}, "main_agent_system_prompt": {},
		"web_search_api_key": {},
	}
	if _, ok := exact[normalized]; ok {
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
		"clientsecret", "privatekey", "systemprompt", "databaseurl",
	} {
		if strings.HasSuffix(compacted, suffix) {
			return true
		}
	}
	for _, suffix := range []string{"_token", "_secret", "_password", "_api_key", "_private_key"} {
		if strings.HasSuffix(normalized, suffix) {
			return true
		}
	}
	return strings.Contains(normalized, "system_prompt")
}

// isSecretValueContainerKey identifies maps whose values are all secret even
// when their user-defined keys (for example X-Custom-Header or PATH) are not.
// The container remains an object; only its leaves cross the OOB secret path.
func isSecretValueContainerKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	switch normalized {
	case "credentials", "headers", "env":
		return true
	default:
		return false
	}
}

// URLContainsSecret rejects embedded credentials and common sensitive query
// parameters because they cannot be safely split out of a model-visible string.
func URLContainsSecret(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" {
		return false
	}
	if parsed.User != nil {
		return true
	}
	for key, values := range parsed.Query() {
		normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"))
		if IsSensitiveKey(normalized) ||
			normalized == "key" ||
			normalized == "code" ||
			normalized == "sig" ||
			normalized == "signature" ||
			strings.Contains(normalized, "credential") {
			for _, item := range values {
				if strings.TrimSpace(item) != "" {
					return true
				}
			}
		}
	}
	return parsed.Fragment != ""
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return map[string]any{"redacted": true}
	}
	var result map[string]any
	if json.Unmarshal(payload, &result) != nil {
		return map[string]any{"redacted": true}
	}
	return result
}

func joinPath(parent string, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

func displayPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return "(input)"
	}
	return path
}

func secretConfigured(value any) bool {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) != ""
	case nil:
		return false
	default:
		return true
	}
}

func redactedSecretPresence(value any) map[string]any {
	return map[string]any{"configured": secretConfigured(value), "redacted": true}
}
