package jsoncodec

import "encoding/json"

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
