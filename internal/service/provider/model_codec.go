// INPUT: Stored model facts and explicit user overrides.
// OUTPUT: Versioned automatic facts; historical inferred positives are not trusted.
// POS: Provider model JSON codec; user overrides never pass through legacy filtering.
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

// facts_version distinguishes observed/imported facts from older records which
// mixed remote metadata with model-name guesses. Re-discovery upgrades the record.
func encodeModelAutoCapabilities(input ModelCapabilities) string {
	payload, _ := json.Marshal(struct {
		ModelCapabilities
		FactsVersion int `json:"facts_version"`
	}{input, 1})
	return string(payload)
}

func decodeModelAutoCapabilities(raw string) ModelCapabilities {
	result := decodeModelCapabilities(raw)
	var metadata struct {
		FactsVersion int `json:"facts_version"`
	}
	_ = json.Unmarshal([]byte(raw), &metadata)
	if metadata.FactsVersion != 1 {
		if result.Vision != nil && *result.Vision {
			result.Vision = nil
		}
		if result.Reasoning != nil && *result.Reasoning {
			result.Reasoning = nil
		}
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
