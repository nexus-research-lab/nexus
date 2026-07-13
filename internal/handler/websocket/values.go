package websocket

import handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"

func firstStringValue(values ...any) string {
	for _, value := range values {
		if text := handlershared.StringValue(value); text != "" {
			return text
		}
	}
	return ""
}

func stringSliceValue(value any) []string {
	rawItems, ok := value.([]any)
	if !ok {
		if typed, ok := value.([]string); ok {
			return typed
		}
		return nil
	}
	result := make([]string, 0, len(rawItems))
	for _, item := range rawItems {
		text := handlershared.StringValue(item)
		if text != "" {
			result = append(result, text)
		}
	}
	return result
}
