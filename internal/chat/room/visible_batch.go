package room

import (
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// PublicCursor 描述目标成员上次消费到的公区位置。
type PublicCursor struct {
	LastMessageID string
	LastTimestamp int64
}

// PublicInputBatchInput 描述公区消息批次选择输入。
type PublicInputBatchInput struct {
	PublicHistory []protocol.Message
	Cursor        PublicCursor
	AgentNameByID map[string]string
	TargetAgentID string
}

// PublicInputBatch 是一次要投递给目标成员的公区消息批次。
type PublicInputBatch struct {
	Messages      []protocol.Message
	LastMessageID string
	LastTimestamp int64
}

// BuildPublicInputBatch 根据目标成员 cursor 选择本次公区输入批次。
func BuildPublicInputBatch(input PublicInputBatchInput) PublicInputBatch {
	candidates := publicMessagesAfterCursor(input.PublicHistory, input.Cursor)
	if len(candidates) > roomMaxHistoryMessages {
		candidates = candidates[len(candidates)-roomMaxHistoryMessages:]
	}
	candidates = trimPublicBatchByChars(candidates, input.AgentNameByID)

	messages := make([]protocol.Message, 0, len(candidates))
	for _, message := range candidates {
		if !isVisiblePublicInputMessage(message, input.TargetAgentID) {
			continue
		}
		messages = append(messages, message)
	}

	batch := PublicInputBatch{Messages: messages}
	if len(candidates) > 0 {
		boundary := candidates[len(candidates)-1]
		batch.LastMessageID = normalizeAnyString(boundary["message_id"])
		batch.LastTimestamp = normalizeInt64(boundary["timestamp"])
	}
	return batch
}

func publicMessagesAfterCursor(history []protocol.Message, cursor PublicCursor) []protocol.Message {
	if len(history) == 0 {
		return nil
	}
	lastMessageID := strings.TrimSpace(cursor.LastMessageID)
	if lastMessageID != "" {
		for index, message := range history {
			if strings.TrimSpace(normalizeAnyString(message["message_id"])) == lastMessageID {
				return slices.Clone(history[index+1:])
			}
		}
	}
	if cursor.LastTimestamp > 0 {
		for index, message := range history {
			if normalizeInt64(message["timestamp"]) > cursor.LastTimestamp {
				return slices.Clone(history[index:])
			}
		}
		return nil
	}
	return slices.Clone(history)
}

func trimPublicBatchByChars(messages []protocol.Message, agentNameByID map[string]string) []protocol.Message {
	if len(messages) == 0 {
		return nil
	}
	totalChars := 0
	start := len(messages)
	for index := len(messages) - 1; index >= 0; index-- {
		line := formatHistoryLine(messages[index], agentNameByID)
		lineChars := len(line)
		nextChars := totalChars
		if lineChars > 0 {
			nextChars += lineChars
			if totalChars > 0 {
				nextChars++
			}
		}
		if nextChars > roomMaxHistoryChars && start < len(messages) {
			break
		}
		start = index
		totalChars = nextChars
		if nextChars > roomMaxHistoryChars {
			break
		}
	}
	return slices.Clone(messages[start:])
}

func isVisiblePublicInputMessage(message protocol.Message, targetAgentID string) bool {
	role := strings.TrimSpace(normalizeAnyString(message["role"]))
	switch role {
	case "user":
		return extractHistoryText(message) != ""
	case "assistant":
		if strings.TrimSpace(normalizeAnyString(message["agent_id"])) == strings.TrimSpace(targetAgentID) {
			return false
		}
		return formatHistoryLine(message, nil) != ""
	default:
		return false
	}
}
