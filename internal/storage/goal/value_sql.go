package goal

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func nullString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}

func nullInt64Pointer(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func marshalMap(value map[string]any) string {
	if value == nil {
		value = map[string]any{}
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func marshalUsage(value protocol.GoalUsage) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(payload)
}
