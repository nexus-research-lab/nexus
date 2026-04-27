package room

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func reverseAgentNames(agentNameByID map[string]string) map[string]string {
	result := make(map[string]string, len(agentNameByID))
	for agentID, name := range agentNameByID {
		normalizedName := strings.TrimSpace(name)
		if normalizedName == "" {
			continue
		}
		result[normalizedName] = agentID
		result[strings.ToLower(normalizedName)] = agentID
	}
	return result
}

func mapTerminalSubtype(status string) string {
	switch status {
	case "finished":
		return "success"
	case "interrupted":
		return "interrupted"
	case "error":
		return "error"
	default:
		return ""
	}
}

func resultStatus(subtype any) string {
	switch strings.TrimSpace(anyString(subtype)) {
	case "interrupted":
		return "cancelled"
	case "error":
		return "error"
	default:
		return "finished"
	}
}

func anyString(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return typed
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func roomTargetResolution(targetAgentIDs []string) string {
	if len(targetAgentIDs) > 0 {
		return "mention"
	}
	return "none"
}

func cloneMessageWithSessionKey(message protocol.Message, sessionKey string) protocol.Message {
	result := make(protocol.Message, len(message))
	for key, value := range message {
		result[key] = value
	}
	result["session_key"] = sessionKey
	return result
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	normalized := strings.TrimSpace(value)
	return &normalized
}

func normalizeInt64(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	default:
		return 0
	}
}

func newRealtimeID() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("room_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}
