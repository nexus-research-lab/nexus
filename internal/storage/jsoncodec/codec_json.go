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
