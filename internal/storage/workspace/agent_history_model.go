package workspace

import (
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	maxTranscriptCacheEntries     = 12
	maxTranscriptSanitizedLength  = 200
	transcriptScannerBufferBytes  = 16 * 1024 * 1024
	transcriptReadBufferBytes     = 64 * 1024
	transcriptSessionSearchTimout = 5 * time.Second
	overlayKindField              = "nexus_overlay_kind"
	overlayKindRoundMarker        = "round_marker"
	overlayKindRoomPublicCursor   = "room_public_cursor"
	overlayKindExternalDelivery   = "external_delivery_receipt"
)

type transcriptEntry struct {
	Index int
	Data  map[string]any
}

type transcriptRoundMarker struct {
	RoundID        string
	UserMessageID  string
	AgentRoundID   string
	Content        string
	Attachments    []protocol.ChatAttachment
	Timestamp      int64
	DeliveryPolicy string
	HiddenFromUser bool
	Synthetic      bool
	Purpose        string
	Metadata       map[string]string
}

type overlayHistoryState struct {
	MessageRows  []protocol.Message
	RoundMarkers []transcriptRoundMarker
}

// RoundMarkerOptions 描述 Nexus overlay round marker 的展示和调度语义。
type RoundMarkerOptions struct {
	UserMessageID  string
	AgentRoundID   string
	DeliveryPolicy string
	Attachments    []protocol.ChatAttachment
	HiddenFromUser bool
	Synthetic      bool
	Purpose        string
	Metadata       map[string]string
}

// RoomPublicCursor 记录某个 Room agent 已消费到的公区消息位置。
type RoomPublicCursor struct {
	RoomID              string
	ConversationID      string
	AgentID             string
	RoundID             string
	LastPublicMessageID string
	LastPublicTimestamp int64
	Timestamp           int64
}

// ExternalDeliveryReceipt 记录一条 assistant 消息投递到外部 IM 后的平台回执。
type ExternalDeliveryReceipt struct {
	RoundID                  string
	MessageID                string
	Channel                  string
	Target                   string
	ThreadID                 string
	PrimaryPlatformMessageID string
	PlatformMessageIDs       []string
	Timestamp                time.Time
}
