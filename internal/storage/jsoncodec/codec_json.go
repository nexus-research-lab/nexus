package jsoncodec

import "encoding/json"

// MarshalStringSlice 编码字符串数组 JSON。
func MarshalStringSlice(values []string) string {
	if values == nil {
		values = []string{}
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}
	return string(payload)
}

// ParseStringSlice 解析字符串数组 JSON。
func ParseStringSlice(raw string) []string {
	if raw == "" {
		return nil
	}
	var result []string
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil
	}
	return result
}

// ParseMap 解析 map JSON。
func ParseMap(raw string) map[string]any {
	if raw == "" {
		return nil
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil
	}
	return result
}

// MarshalMap 编码 map JSON，并把不可持久化的值交给调用方处理。
func MarshalMap(values map[string]any) (string, error) {
	if values == nil {
		values = map[string]any{}
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}
