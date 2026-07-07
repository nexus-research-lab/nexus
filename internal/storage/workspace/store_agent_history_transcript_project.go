package workspace

import (
	"strings"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func projectTranscriptChain(
	workspacePath string,
	sessionKey string,
	agentID string,
	chain []transcriptEntry,
	roundMarkers []transcriptRoundMarker,
) []protocol.Message {
	projected := make([]protocol.Message, 0, len(chain))
	currentRoundID := ""
	var processor *message.Processor
	var lastTimestamp int64
	alignedMarkers := alignTranscriptRoundMarkers(chain, roundMarkers)
	markerIndex := 0

	for _, entry := range chain {
		if shouldSkipTranscriptEntry(entry.Data) {
			continue
		}

		entryTimestamp := transcriptEntryTimestamp(entry.Data, entry.Index, lastTimestamp)
		lastTimestamp = entryTimestamp

		if guidanceRows := buildTranscriptGuidanceMessages(
			sessionKey,
			agentID,
			currentRoundID,
			entry.Data,
			entryTimestamp,
		); len(guidanceRows) > 0 {
			projected = append(projected, guidanceRows...)
			continue
		}

		decoded, err := sdkprotocol.DecodeMessage(entry.Data)
		if err != nil {
			continue
		}

		switch decoded.Type {
		case sdkprotocol.MessageTypeUser:
			if isTranscriptToolResult(decoded) {
				if processor == nil {
					currentRoundID = firstNonEmpty(stringFromAny(entry.Data["parentUuid"]), strings.TrimSpace(decoded.UUID))
					processor = newTranscriptProcessor(workspacePath, sessionKey, agentID, currentRoundID, decoded.SessionID)
				}
				output := processor.Process(decoded)
				projected = append(projected, stampTranscriptDurableMessages(output.DurableMessages, entryTimestamp)...)
				continue
			}
			// 中文注释：runtime transcript 会夹杂一类“空 user turn”，
			// 它们不是前端真实输入，不能消费 Nexus 的 round marker。
			// 否则后续真实 assistant 会挂错 round，result 也就无法并回同一轮。
			if !shouldMaterializeTranscriptUserTurn(entry.Data) {
				continue
			}
			marker := consumeTranscriptRoundMarker(alignedMarkers, &markerIndex)
			currentRoundID = firstNonEmpty(marker.RoundID, buildTranscriptRoundID(decoded.UUID))
			processor = newTranscriptProcessor(workspacePath, sessionKey, agentID, currentRoundID, decoded.SessionID)
			if marker.HiddenFromUser || isTranscriptGoalContextOnlyUserTurn(entry.Data) {
				continue
			}
			userMessage := buildTranscriptUserMessage(
				sessionKey,
				agentID,
				currentRoundID,
				marker.UserMessageID,
				decoded.SessionID,
				entry.Data,
				marker.Content,
				marker.Attachments,
				marker.DeliveryPolicy,
				entryTimestamp,
			)
			if userMessage == nil {
				continue
			}
			projected = append(projected, *userMessage)
		case sdkprotocol.MessageTypeAssistant,
			sdkprotocol.MessageTypeSystem,
			sdkprotocol.MessageTypeTaskProgress:
			if processor == nil {
				currentRoundID = buildTranscriptRoundID(decoded.UUID)
				processor = newTranscriptProcessor(workspacePath, sessionKey, agentID, currentRoundID, decoded.SessionID)
			}
			output := processor.Process(decoded)
			projected = append(projected, stampTranscriptDurableMessages(output.DurableMessages, entryTimestamp)...)
		case sdkprotocol.MessageTypeResult:
			// result 统一以 Nexus overlay 为真相源。
			// transcript 即使带了 result，也不再直接投影进历史，
			// 避免 assistant/usage 与 runtime result 语义重新混在一起。
			continue
		default:
			continue
		}
	}

	return projected
}

func newTranscriptProcessor(
	workspacePath string,
	sessionKey string,
	agentID string,
	roundID string,
	sessionID string,
) *message.Processor {
	return message.NewProcessor(message.MessageContext{
		SessionKey:    sessionKey,
		AgentID:       agentID,
		WorkspacePath: strings.TrimSpace(workspacePath),
		RoundID:       roundID,
		ParentID:      roundID,
	}, strings.TrimSpace(sessionID))
}

func buildTranscriptUserMessage(
	sessionKey string,
	agentID string,
	roundID string,
	userMessageID string,
	sessionID string,
	entry map[string]any,
	contentOverride string,
	attachments []protocol.ChatAttachment,
	deliveryPolicy string,
	timestamp int64,
) *protocol.Message {
	content := firstNonEmpty(contentOverride, transcriptUserContent(entry))
	if content == "" {
		return nil
	}
	// 与 overlay marker 物化保持同一派生规则，保证按 message_id 去重仍然生效。
	if userMessageID = strings.TrimSpace(userMessageID); userMessageID == "" {
		userMessageID = "msg_user_" + roundID
	}
	payload := protocol.Message{
		"message_id":  userMessageID,
		"session_key": sessionKey,
		"agent_id":    agentID,
		"round_id":    roundID,
		"role":        "user",
		"content":     content,
		"timestamp":   timestamp,
	}
	if strings.TrimSpace(sessionID) != "" {
		payload["session_id"] = strings.TrimSpace(sessionID)
	}
	if strings.TrimSpace(deliveryPolicy) != "" {
		payload["delivery_policy"] = string(protocol.NormalizeChatDeliveryPolicy(deliveryPolicy))
	}
	if normalizedAttachments := protocol.NormalizeChatAttachments(attachments, agentID); len(normalizedAttachments) > 0 {
		payload["attachments"] = normalizedAttachments
	}
	return &payload
}

func transcriptUserContent(entry map[string]any) string {
	messageValue, _ := entry["message"].(map[string]any)
	contentValue := messageValue["content"]
	if text := sanitizeTranscriptUserContent(stringFromAny(contentValue)); text != "" {
		return text
	}
	items, _ := contentValue.([]any)
	parts := make([]string, 0, len(items))
	for _, item := range items {
		payload, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if text := sanitizeTranscriptUserContent(stringFromAny(payload["text"])); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func sanitizeTranscriptUserContent(content string) string {
	trimmed := strings.TrimSpace(content)
	if message.IsInternalTranscriptInterruptPrompt(trimmed) {
		return ""
	}
	return trimmed
}

func stampTranscriptDurableMessages(
	rows []protocol.Message,
	timestamp int64,
) []protocol.Message {
	if len(rows) == 0 {
		return nil
	}
	result := make([]protocol.Message, 0, len(rows))
	for _, row := range rows {
		stamped := protocol.Clone(row)
		stamped["timestamp"] = timestamp
		result = append(result, stamped)
	}
	return result
}

func isTranscriptToolResult(message sdkprotocol.ReceivedMessage) bool {
	if message.User == nil {
		return false
	}
	if message.User.ToolUseResult != nil {
		return true
	}
	for _, block := range message.User.Message.Content {
		blockType := strings.TrimSpace(string(block.Type()))
		if blockType == "tool_result" || blockType == "server_tool_result" {
			return true
		}
	}
	return false
}

func buildTranscriptRoundID(uuid string) string {
	trimmed := strings.TrimSpace(uuid)
	if trimmed == "" {
		return "transcript_round"
	}
	return trimmed
}

func transcriptEntryTimestamp(entry map[string]any, index int, lastTimestamp int64) int64 {
	value := stringFromAny(entry["timestamp"])
	if value != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return parsed.UnixMilli()
		}
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			return parsed.UnixMilli()
		}
	}
	if lastTimestamp > 0 {
		return lastTimestamp + 1
	}
	return int64(index + 1)
}
