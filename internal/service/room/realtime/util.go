package realtime

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
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

// roomSlotTerminalStatus 同时兼容 mapper 终态 subtype 和旧 runtime 的 status 字段。
func roomSlotTerminalStatus(result exec.RoundExecutionResult) string {
	subtype := strings.TrimSpace(result.ResultSubtype)
	if subtype == "" {
		subtype = strings.TrimSpace(result.TerminalStatus)
	}
	return resultStatus(subtype)
}

func anyString(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return typed
}

func roomTargetResolution(targetAgentIDs []string) string {
	if len(targetAgentIDs) > 0 {
		return "mention"
	}
	return "none"
}

// normalizeRoomAgentIDs 清理并按输入顺序去重 Agent ID。
func normalizeRoomAgentIDs(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func cloneMessageWithSessionKey(message protocol.Message, sessionKey string) protocol.Message {
	result := make(protocol.Message, len(message))
	for key, value := range message {
		result[key] = value
	}
	result["session_key"] = sessionKey
	return result
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
