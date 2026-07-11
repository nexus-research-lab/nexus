package provider

import (
	"encoding/json"
	"strings"
)

func encodeModelCapabilities(input ModelCapabilities) string {
	payload, err := json.Marshal(input)
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func decodeModelCapabilities(raw string) ModelCapabilities {
	var result ModelCapabilities
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &result); err != nil {
		return ModelCapabilities{}
	}
	return result
}

func encodeProviderOptions(input map[string]any) string {
	if len(input) == 0 {
		return "{}"
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func decodeProviderOptions(raw string) map[string]any {
	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &result); err != nil || result == nil {
		return map[string]any{}
	}
	return result
}
