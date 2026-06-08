package room

import (
	"strings"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *RealtimeService) recordPrivateRoundMarker(roundValue *activeRoomRound, slot *activeRoomSlot, dispatchPrompt string) error {
	if s.history == nil {
		return nil
	}
	options := roomRoundMarkerOptions(roundValue)
	if !options.HiddenFromUser && !options.Synthetic && strings.TrimSpace(options.Purpose) == "" && len(options.Metadata) == 0 {
		return s.history.AppendRoundMarker(
			slot.WorkspacePath,
			slot.RuntimeSessionKey,
			slot.AgentRoundID,
			strings.TrimSpace(dispatchPrompt),
			time.Now().UnixMilli(),
		)
	}
	return s.history.AppendRoundMarkerWithOptions(
		slot.WorkspacePath,
		slot.RuntimeSessionKey,
		slot.AgentRoundID,
		strings.TrimSpace(dispatchPrompt),
		time.Now().UnixMilli(),
		options,
	)
}

func roomRoundInputOptions(roundValue *activeRoomRound) sdkprotocol.OutboundMessageOptions {
	if roundValue == nil {
		return sdkprotocol.OutboundMessageOptions{}
	}
	options := roundValue.InputOptions
	if roundValue.Internal {
		options.HiddenFromUser = true
		options.Synthetic = true
		if strings.TrimSpace(options.Priority) == "" {
			options.Priority = "internal"
		}
	}
	return options
}

func roomRoundMarkerOptions(roundValue *activeRoomRound) workspacestore.RoundMarkerOptions {
	options := workspacestore.RoundMarkerOptions{}
	if roundValue == nil {
		return options
	}
	options.HiddenFromUser = roundValue.Internal || roundValue.InputOptions.HiddenFromUser
	options.Synthetic = roundValue.InputOptions.Synthetic
	options.Purpose = roundValue.InputOptions.Purpose
	options.Metadata = roundValue.InputOptions.Metadata
	if roundValue.Internal {
		options.Synthetic = true
	}
	return options
}

func (s *RealtimeService) persistPrivateOverlayMessage(slot *activeRoomSlot, message protocol.Message) error {
	if s.history == nil {
		return nil
	}
	privateMessage := normalizePrivateOverlayMessage(cloneMessageWithSessionKey(message, slot.RuntimeSessionKey))
	privateMessage["session_key"] = slot.RuntimeSessionKey
	if sessionID := firstNonEmpty(strings.TrimSpace(anyString(privateMessage["session_id"])), slot.getSDKSessionID()); sessionID != "" {
		privateMessage["session_id"] = sessionID
	}
	if strings.TrimSpace(anyString(privateMessage["message_id"])) == "" {
		privateMessage["message_id"] = "overlay_" + slot.AgentRoundID
	}
	privateMessage["metadata"] = mergePrivateOverlayMetadata(privateMessage["metadata"], map[string]any{
		"overlay_source":  "room_runtime",
		"room_session_id": slot.RoomSessionID,
	})
	return s.history.AppendOverlayMessage(slot.WorkspacePath, slot.RuntimeSessionKey, privateMessage)
}

func normalizePrivateOverlayMessage(message protocol.Message) protocol.Message {
	normalized := cloneMessageWithSessionKey(message, anyString(message["session_key"]))
	delete(normalized, "stream_status")
	delete(normalized, "is_complete")
	return normalized
}

func mergePrivateOverlayMetadata(current any, extra map[string]any) map[string]any {
	result := map[string]any{}
	if payload, ok := current.(map[string]any); ok {
		for key, value := range payload {
			result[key] = value
		}
	}
	for key, value := range extra {
		result[key] = value
	}
	return result
}
