package protocol

import "encoding/json"

// ParseJSONStringSlice 解析字符串数组 JSON。
func ParseJSONStringSlice(raw string) []string {
	if raw == "" {
		return nil
	}
	var result []string
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil
	}
	return result
}

// ParseJSONMap 解析 map JSON。
func ParseJSONMap(raw string) map[string]any {
	if raw == "" {
		return nil
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil
	}
	return result
}
