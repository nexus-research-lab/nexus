package workspace

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// AppendOverlayMessage 追加一条 Nexus overlay 消息。
func (s *AgentHistoryStore) AppendOverlayMessage(workspacePath string, sessionKey string, message protocol.Message) error {
	return s.files.appendJSONL(s.paths.SessionOverlayPath(workspacePath, sessionKey), message)
}

// AppendExternalDeliveryReceipt 追加一条外部 IM 投递回执 overlay 控制行。
func (s *AgentHistoryStore) AppendExternalDeliveryReceipt(
	workspacePath string,
	sessionKey string,
	receipt ExternalDeliveryReceipt,
) error {
	normalized := normalizeExternalDeliveryReceipt(receipt)
	if !normalized.hasAddress() || !normalized.hasDeliveryData() {
		return nil
	}

	timestamp := normalized.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	row := protocol.Message{
		overlayKindField:              overlayKindExternalDelivery,
		"message_id":                  externalDeliveryReceiptMessageID(normalized, timestamp),
		"role":                        overlayKindExternalDelivery,
		"round_id":                    normalized.RoundID,
		"assistant_message_id":        normalized.MessageID,
		"channel":                     normalized.Channel,
		"target":                      normalized.Target,
		"thread_id":                   normalized.ThreadID,
		"primary_platform_message_id": normalized.PrimaryPlatformMessageID,
		"platform_message_ids":        normalized.PlatformMessageIDs,
		"timestamp":                   timestamp.UnixMilli(),
	}
	return s.AppendOverlayMessage(workspacePath, sessionKey, row)
}

// AppendRoundMarker 记录一条 transcript round 对齐标记。
func (s *AgentHistoryStore) AppendRoundMarker(
	workspacePath string,
	sessionKey string,
	roundID string,
	content string,
	timestamp int64,
	deliveryPolicies ...string,
) error {
	var deliveryPolicy string
	if len(deliveryPolicies) > 0 {
		deliveryPolicy = strings.TrimSpace(deliveryPolicies[0])
	}
	return s.AppendRoundMarkerWithAttachments(workspacePath, sessionKey, roundID, content, timestamp, deliveryPolicy, nil)
}

// AppendRoundMarkerWithAttachments 记录一条带附件 metadata 的 transcript round 对齐标记。
func (s *AgentHistoryStore) AppendRoundMarkerWithAttachments(
	workspacePath string,
	sessionKey string,
	roundID string,
	content string,
	timestamp int64,
	deliveryPolicy string,
	attachments []protocol.ChatAttachment,
) error {
	return s.AppendRoundMarkerWithOptions(workspacePath, sessionKey, roundID, content, timestamp, RoundMarkerOptions{
		DeliveryPolicy: deliveryPolicy,
		Attachments:    attachments,
	})
}

// AppendRoundMarkerWithOptions 记录一条带展示语义的 transcript round 对齐标记。
func (s *AgentHistoryStore) AppendRoundMarkerWithOptions(
	workspacePath string,
	sessionKey string,
	roundID string,
	content string,
	timestamp int64,
	options RoundMarkerOptions,
) error {
	row := map[string]any{
		overlayKindField: overlayKindRoundMarker,
		"round_id":       strings.TrimSpace(roundID),
		"content":        strings.TrimSpace(content),
		"timestamp":      timestamp,
	}
	if userMessageID := strings.TrimSpace(options.UserMessageID); userMessageID != "" {
		row["user_message_id"] = userMessageID
	}
	if agentRoundID := strings.TrimSpace(options.AgentRoundID); agentRoundID != "" {
		row["agent_round_id"] = agentRoundID
	}
	if options.DeliveryPolicy != "" {
		row["delivery_policy"] = string(protocol.NormalizeChatDeliveryPolicy(options.DeliveryPolicy))
	}
	if normalizedAttachments := protocol.NormalizeChatAttachments(options.Attachments, ""); len(normalizedAttachments) > 0 {
		row["attachments"] = normalizedAttachments
	}
	if options.HiddenFromUser {
		row["hidden_from_user"] = true
	}
	if options.Synthetic {
		row["is_synthetic"] = true
	}
	if purpose := strings.TrimSpace(options.Purpose); purpose != "" {
		row["purpose"] = purpose
	}
	if len(options.Metadata) > 0 {
		metadata := make(map[string]string, len(options.Metadata))
		for key, value := range options.Metadata {
			if trimmedKey := strings.TrimSpace(key); trimmedKey != "" {
				metadata[trimmedKey] = strings.TrimSpace(value)
			}
		}
		if len(metadata) > 0 {
			row["metadata"] = metadata
		}
	}
	return s.files.appendJSONL(s.paths.SessionOverlayPath(workspacePath, sessionKey), row)
}

// AppendRoomPublicCursor 追加 Room 公区消费位置控制行。
func (s *AgentHistoryStore) AppendRoomPublicCursor(workspacePath string, sessionKey string, cursor RoomPublicCursor) error {
	now := time.Now().UnixMilli()
	if cursor.Timestamp == 0 {
		cursor.Timestamp = now
	}
	row := map[string]any{
		overlayKindField:         overlayKindRoomPublicCursor,
		"room_id":                strings.TrimSpace(cursor.RoomID),
		"conversation_id":        strings.TrimSpace(cursor.ConversationID),
		"agent_id":               strings.TrimSpace(cursor.AgentID),
		"round_id":               strings.TrimSpace(cursor.RoundID),
		"last_public_message_id": strings.TrimSpace(cursor.LastPublicMessageID),
		"last_public_timestamp":  cursor.LastPublicTimestamp,
		"timestamp":              cursor.Timestamp,
	}
	return s.files.appendJSONL(s.paths.SessionOverlayPath(workspacePath, sessionKey), row)
}

// ReadRoomPublicCursor 读取 Room agent 最新公区消费位置。
func (s *AgentHistoryStore) ReadRoomPublicCursor(
	workspacePath string,
	sessionKey string,
	conversationID string,
	agentID string,
) (RoomPublicCursor, bool, error) {
	rows, err := s.files.readJSONL(s.paths.SessionOverlayPath(workspacePath, sessionKey))
	if errors.Is(err, os.ErrNotExist) {
		return RoomPublicCursor{}, false, nil
	}
	if err != nil {
		return RoomPublicCursor{}, false, err
	}

	conversationID = strings.TrimSpace(conversationID)
	agentID = strings.TrimSpace(agentID)
	var latest RoomPublicCursor
	found := false
	for _, row := range rows {
		if stringFromAny(row[overlayKindField]) != overlayKindRoomPublicCursor {
			continue
		}
		if conversationID != "" && stringFromAny(row["conversation_id"]) != conversationID {
			continue
		}
		if agentID != "" && stringFromAny(row["agent_id"]) != agentID {
			continue
		}
		cursor := RoomPublicCursor{
			RoomID:              stringFromAny(row["room_id"]),
			ConversationID:      stringFromAny(row["conversation_id"]),
			AgentID:             stringFromAny(row["agent_id"]),
			RoundID:             stringFromAny(row["round_id"]),
			LastPublicMessageID: stringFromAny(row["last_public_message_id"]),
			LastPublicTimestamp: messageTimestamp(protocol.Message(row)),
			Timestamp:           messageTimestamp(protocol.Message(row)),
		}
		if value := int64FromAny(row["last_public_timestamp"]); value > 0 {
			cursor.LastPublicTimestamp = value
		}
		if !found || cursor.Timestamp >= latest.Timestamp {
			latest = cursor
			found = true
		}
	}
	return latest, found, nil
}

func (s *AgentHistoryStore) readOverlayRowsAndMarkers(
	workspacePath string,
	sessionKey string,
) ([]protocol.Message, []transcriptRoundMarker, error) {
	state, err := s.readOverlayHistoryState(workspacePath, sessionKey)
	if err != nil {
		return nil, nil, err
	}
	return state.MessageRows, state.RoundMarkers, nil
}

func (s *AgentHistoryStore) readOverlayHistoryState(
	workspacePath string,
	sessionKey string,
) (overlayHistoryState, error) {
	rows, err := s.files.readJSONL(s.paths.SessionOverlayPath(workspacePath, sessionKey))
	if errors.Is(err, os.ErrNotExist) {
		return overlayHistoryState{
			MessageRows:  []protocol.Message{},
			RoundMarkers: []transcriptRoundMarker{},
		}, nil
	}
	if err != nil {
		return overlayHistoryState{}, err
	}

	messageRows := make([]protocol.Message, 0, len(rows))
	roundMarkers := make([]transcriptRoundMarker, 0)
	for _, row := range rows {
		switch stringFromAny(row[overlayKindField]) {
		case overlayKindRoundMarker:
			roundMarkers = append(roundMarkers, transcriptRoundMarker{
				RoundID:        stringFromAny(row["round_id"]),
				UserMessageID:  stringFromAny(row["user_message_id"]),
				AgentRoundID:   stringFromAny(row["agent_round_id"]),
				Content:        stringFromAny(row["content"]),
				Attachments:    protocol.ChatAttachmentsFromAny(row["attachments"]),
				Timestamp:      messageTimestamp(protocol.Message(row)),
				DeliveryPolicy: stringFromAny(row["delivery_policy"]),
				HiddenFromUser: boolValueAny(row["hidden_from_user"]),
				Synthetic:      boolValueAny(row["is_synthetic"]),
				Purpose:        stringFromAny(row["purpose"]),
				Metadata:       stringMapFromAny(row["metadata"]),
			})
			continue
		}
		if isSessionOverlayControlRow(row) {
			continue
		}
		messageRows = append(messageRows, protocol.Message(row))
	}
	return overlayHistoryState{
		MessageRows:  messageRows,
		RoundMarkers: roundMarkers,
	}, nil
}

func isSessionOverlayControlRow(row map[string]any) bool {
	switch stringFromAny(row[overlayKindField]) {
	case "history_rewrite", overlayKindRoomPublicCursor, "room_context_checkpoint":
		return true
	default:
		return false
	}
}
