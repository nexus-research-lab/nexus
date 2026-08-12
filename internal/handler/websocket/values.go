package websocket

import (
	"bytes"
	"encoding/json"
	"errors"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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

func goalCommandOptionsValue(value any) (protocol.GoalCommandOptions, error) {
	if value == nil {
		return protocol.GoalCommandOptions{}, nil
	}
	if typed, ok := value.(protocol.GoalCommandOptions); ok {
		return typed, nil
	}
	if _, ok := value.(map[string]any); !ok {
		return protocol.GoalCommandOptions{}, errors.New("goal_options must be an object")
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return protocol.GoalCommandOptions{}, err
	}
	var result protocol.GoalCommandOptions
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&result); err != nil {
		return protocol.GoalCommandOptions{}, err
	}
	return result, nil
}
