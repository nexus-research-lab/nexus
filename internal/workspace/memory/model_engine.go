package memory

import (
	"strings"
	"time"
)

// ScopeKind 描述记忆隔离边界。
type ScopeKind string

const (
	ScopeKindUser             ScopeKind = "user"
	ScopeKindAgent            ScopeKind = "agent"
	ScopeKindDMSession        ScopeKind = "dm_session"
	ScopeKindRoomShared       ScopeKind = "room_shared"
	ScopeKindRoomAgentSession ScopeKind = "room_agent_session"
)

// MemoryScope 描述一次召回或提交可见的记忆范围。
type MemoryScope struct {
	Kind           ScopeKind `json:"kind"`
	UserID         string    `json:"user_id,omitempty"`
	AgentID        string    `json:"agent_id,omitempty"`
	SessionKey     string    `json:"session_key,omitempty"`
	SessionID      string    `json:"session_id,omitempty"`
	RoomID         string    `json:"room_id,omitempty"`
	ConversationID string    `json:"conversation_id,omitempty"`
}

// Key 返回可持久化的 scope 唯一键。
func (s MemoryScope) Key() string {
	switch s.Kind {
	case ScopeKindRoomShared:
		return buildScopeKey(s.Kind, s.RoomID, s.ConversationID)
	case ScopeKindRoomAgentSession:
		return buildScopeKey(s.Kind, s.RoomID, s.ConversationID, s.AgentID)
	case ScopeKindDMSession:
		return buildScopeKey(s.Kind, s.AgentID, s.SessionKey)
	case ScopeKindAgent:
		return buildScopeKey(s.Kind, s.AgentID)
	case ScopeKindUser:
		return buildScopeKey(s.Kind, s.UserID)
	default:
		return s.inferredKey()
	}
}

func (s MemoryScope) inferredKey() string {
	if strings.TrimSpace(s.RoomID) != "" && strings.TrimSpace(s.ConversationID) != "" && strings.TrimSpace(s.AgentID) != "" {
		return buildScopeKey(ScopeKindRoomAgentSession, s.RoomID, s.ConversationID, s.AgentID)
	}
	if strings.TrimSpace(s.AgentID) != "" && strings.TrimSpace(s.SessionKey) != "" {
		return buildScopeKey(ScopeKindDMSession, s.AgentID, s.SessionKey)
	}
	if strings.TrimSpace(s.AgentID) != "" {
		return buildScopeKey(ScopeKindAgent, s.AgentID)
	}
	if strings.TrimSpace(s.UserID) != "" {
		return buildScopeKey(ScopeKindUser, s.UserID)
	}
	if strings.TrimSpace(s.RoomID) != "" && strings.TrimSpace(s.ConversationID) != "" {
		return buildScopeKey(ScopeKindRoomShared, s.RoomID, s.ConversationID)
	}
	return ""
}

func buildScopeKey(kind ScopeKind, requiredParts ...string) string {
	parts := make([]string, 0, len(requiredParts)+1)
	parts = append(parts, string(kind))
	for _, part := range requiredParts {
		value := strings.TrimSpace(part)
		if value == "" {
			return ""
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, ":")
}

// MemoryOptions 控制 v1 记忆行为。
type MemoryOptions struct {
	Enabled        bool
	AutoRecall     bool
	AutoExtract    bool
	MaxResults     int
	ScoreThreshold float64
	RecallTimeout  time.Duration
}

// DefaultOptions 返回默认记忆配置。
func DefaultOptions() MemoryOptions {
	return MemoryOptions{
		Enabled:        true,
		AutoRecall:     true,
		AutoExtract:    true,
		MaxResults:     5,
		ScoreThreshold: 0.08,
		RecallTimeout:  2 * time.Second,
	}
}

// Normalize 填充非布尔配置的默认值。
func (o MemoryOptions) Normalize() MemoryOptions {
	if o.MaxResults <= 0 {
		o.MaxResults = DefaultOptions().MaxResults
	}
	if o.ScoreThreshold <= 0 {
		o.ScoreThreshold = DefaultOptions().ScoreThreshold
	}
	if o.RecallTimeout <= 0 {
		o.RecallTimeout = DefaultOptions().RecallTimeout
	}
	return o
}

// RecallRequest 描述一次动态召回输入。
type RecallRequest struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results,omitempty"`
}

// MemoryInjection 是 prompt 注入结果。
type MemoryInjection struct {
	StableSystemContext string       `json:"stable_system_context,omitempty"`
	DynamicUserContext  string       `json:"dynamic_user_context,omitempty"`
	Items               []MemoryItem `json:"items"`
}

// CommittedTurn 描述一次成功对话的记忆候选。
type CommittedTurn struct {
	UserText       string    `json:"user_text"`
	AssistantText  string    `json:"assistant_text"`
	SessionKey     string    `json:"session_key,omitempty"`
	SessionID      string    `json:"session_id,omitempty"`
	RoundID        string    `json:"round_id,omitempty"`
	AgentID        string    `json:"agent_id,omitempty"`
	RoomID         string    `json:"room_id,omitempty"`
	ConversationID string    `json:"conversation_id,omitempty"`
	Timestamp      time.Time `json:"timestamp,omitempty"`
}

// CaptureResult 描述自动记忆提交结果。
type CaptureResult struct {
	Processed bool         `json:"processed"`
	Skipped   bool         `json:"skipped"`
	Reason    string       `json:"reason,omitempty"`
	Items     []MemoryItem `json:"items,omitempty"`
}

// MemoryItem 是前端、HTTP、CLI 共享的记忆条目模型。
type MemoryItem struct {
	EntryID     string    `json:"entry_id"`
	Path        string    `json:"path"`
	Kind        string    `json:"kind"`
	Category    string    `json:"category,omitempty"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority,omitempty"`
	Source      string    `json:"source,omitempty"`
	Scope       string    `json:"scope,omitempty"`
	SessionKey  string    `json:"session_key,omitempty"`
	RoundID     string    `json:"round_id,omitempty"`
	AccessCount int       `json:"access_count"`
	Score       float64   `json:"score,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	Fields      []Field   `json:"fields,omitempty"`
}

// MemoryListOptions 描述列表过滤条件。
type MemoryListOptions struct {
	Limit    int      `json:"limit,omitempty"`
	Statuses []string `json:"statuses,omitempty"`
	Scope    string   `json:"scope,omitempty"`
}

// MemoryWriteInput 描述手动新增或更新条目的输入。
type MemoryWriteInput struct {
	Kind     string  `json:"kind,omitempty"`
	Category string  `json:"category,omitempty"`
	Title    string  `json:"title,omitempty"`
	Content  string  `json:"content,omitempty"`
	Status   string  `json:"status,omitempty"`
	Priority string  `json:"priority,omitempty"`
	Source   string  `json:"source,omitempty"`
	Scope    string  `json:"scope,omitempty"`
	Fields   []Field `json:"fields,omitempty"`
}

// MemoryStats 描述当前 workspace 记忆概况。
type MemoryStats struct {
	Total        int            `json:"total"`
	ByStatus     map[string]int `json:"by_status"`
	ByKind       map[string]int `json:"by_kind"`
	ByScope      map[string]int `json:"by_scope"`
	Candidate    int            `json:"candidate"`
	Accessed     int            `json:"accessed"`
	Checkpointed int            `json:"checkpointed"`
}

// MemoryCleanupResult 描述记忆清理结果。
type MemoryCleanupResult struct {
	RemovedSessionFiles int      `json:"removed_session_files"`
	RemovedCheckpoints  int      `json:"removed_checkpoints"`
	RemovedEmptyDiaries int      `json:"removed_empty_diaries"`
	RemovedFiles        []string `json:"removed_files,omitempty"`
}

// MemoryIndex 是 v2 接入 FTS、向量或图谱索引的边界。
type MemoryIndex interface {
	Search(query string, candidates []MemoryItem, limit int) []MemoryItem
}
