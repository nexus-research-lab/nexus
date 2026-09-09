// INPUT: Nexus Relay `/api/relay/v1` JSON envelope 与分页字段。
// OUTPUT: Nexus 不依赖 Relay 仓库即可消费的稳定 M1 wire DTO。
// POS: Relay HTTP client 的跨仓协议真相源。
package relay

import "time"

const (
	// ContentVersionV1 是首版 Markdown block 正文版本。
	ContentVersionV1 = 1
	// BlockTypeMarkdown 是 Relay M1 唯一正文 block。
	BlockTypeMarkdown = "markdown"
	// AuthorTypeUser 是 Relay M1 唯一消息作者类型。
	AuthorTypeUser = "user"
)

// ContentBlock 是一段版本化消息正文。
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MessageContent 是 Relay 保存的共享正文。
type MessageContent struct {
	Version int            `json:"version"`
	Blocks  []ContentBlock `json:"blocks"`
}

// Team 是当前 Deployment 的默认协作空间。
type Team struct {
	ID           string `json:"id"`
	DeploymentID string `json:"deployment_id"`
	Name         string `json:"name"`
}

// Room 是 Relay M1 的共享房间。
type Room struct {
	ID     string `json:"id"`
	TeamID string `json:"team_id"`
	Name   string `json:"name"`
}

// Conversation 是 Relay 消息排序与同步边界。
type Conversation struct {
	ID                    string `json:"id"`
	RoomID                string `json:"room_id"`
	Type                  string `json:"type"`
	HighWaterMessageSeq   int64  `json:"high_water_message_seq"`
	SyncStreamID          string `json:"sync_stream_id"`
	StreamEpoch           string `json:"stream_epoch"`
	HighWaterSyncEventSeq int64  `json:"high_water_sync_event_seq"`
}

// Bootstrap 是当前 Deployment 的默认 Team、Room 和 Conversation。
type Bootstrap struct {
	Team         Team         `json:"team"`
	Room         Room         `json:"room"`
	Conversation Conversation `json:"conversation"`
}

// Message 是 Conversation 中只追加一次的真人消息。
type Message struct {
	ID                string         `json:"id"`
	ConversationID    string         `json:"conversation_id"`
	MessageSeq        int64          `json:"message_seq"`
	AuthorType        string         `json:"author_type"`
	AuthorUserID      string         `json:"author_user_id"`
	AuthorUsername    string         `json:"author_username"`
	AuthorDisplayName string         `json:"author_display_name"`
	ClientMessageID   string         `json:"client_message_id"`
	Content           MessageContent `json:"content"`
	CreatedAt         time.Time      `json:"created_at"`
}

// CreateMessageInput 是 Relay M1 消息写入意图。
type CreateMessageInput struct {
	Content MessageContent `json:"content"`
}

// MessageCommit 是 Message 与同步水位的原子提交结果。
type MessageCommit struct {
	Message      Message `json:"message"`
	StreamID     string  `json:"stream_id"`
	StreamEpoch  string  `json:"stream_epoch"`
	EventSeq     int64   `json:"event_seq"`
	HighWaterSeq int64   `json:"high_water_seq"`
	Replayed     bool    `json:"replayed"`
}

// Snapshot 是固定消息上界与同步水位的一页一致性快照。
type Snapshot struct {
	ConversationID    string    `json:"conversation_id"`
	StreamID          string    `json:"stream_id"`
	StreamEpoch       string    `json:"stream_epoch"`
	SnapshotSeq       int64     `json:"snapshot_seq"`
	ThroughMessageSeq int64     `json:"through_message_seq"`
	AfterMessageSeq   int64     `json:"after_message_seq"`
	NextMessageSeq    int64     `json:"next_message_seq"`
	HasMore           bool      `json:"has_more"`
	Messages          []Message `json:"messages"`
}

// SyncEvent 是可由 current-state Message 重建的同步索引事件。
type SyncEvent struct {
	EventSeq int64   `json:"event_seq"`
	Type     string  `json:"type"`
	Message  Message `json:"message"`
}

// Difference 是 Conversation stream 的连续增量。
type Difference struct {
	StreamID       string      `json:"stream_id"`
	StreamEpoch    string      `json:"stream_epoch"`
	AfterSeq       int64       `json:"after_seq"`
	NextSeq        int64       `json:"next_seq"`
	HighWaterSeq   int64       `json:"high_water_seq"`
	MinRetainedSeq int64       `json:"min_retained_seq"`
	HasMore        bool        `json:"has_more"`
	Events         []SyncEvent `json:"events"`
}

// StreamUpdated 是 Relay WSS 发出的已提交水位提示；消息恢复仍走 Difference。
type StreamUpdated struct {
	Type         string `json:"type"`
	StreamID     string `json:"stream_id"`
	StreamEpoch  string `json:"stream_epoch"`
	HighWaterSeq int64  `json:"high_water_seq"`
}

// SnapshotOptions 描述一页固定上界的快照请求。
type SnapshotOptions struct {
	AfterMessageSeq   int64
	Limit             int
	ThroughMessageSeq *int64
	SnapshotSeq       *int64
	StreamEpoch       string
}

// DifferenceOptions 描述一页增量同步请求。
type DifferenceOptions struct {
	AfterSeq    int64
	Limit       int
	StreamEpoch string
}
