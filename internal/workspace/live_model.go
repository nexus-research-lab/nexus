package workspace

const (
	// LiveEventFileWriteStart 表示文件开始写入。
	LiveEventFileWriteStart = "file_write_start"
	// LiveEventFileWriteDelta 表示文件写入中的增量快照。
	LiveEventFileWriteDelta = "file_write_delta"
	// LiveEventFileWriteEnd 表示文件写入完成。
	LiveEventFileWriteEnd = "file_write_end"
	// LiveEventFileDeleted 表示文件已删除。
	LiveEventFileDeleted = "file_deleted"

	// LiveSourceAgent 表示来自 Agent 运行时的文件变更。
	LiveSourceAgent = "agent"
	// LiveSourceAPI 表示来自 REST/CLI 的文件变更。
	LiveSourceAPI = "api"
	// LiveSourceSystem 表示系统级文件变更。
	LiveSourceSystem = "system"
	// LiveSourceUnknown 表示未知来源的文件变更。
	LiveSourceUnknown = "unknown"
)

// DiffStats 表示一次文件写入的摘要。
type DiffStats struct {
	Additions    int `json:"additions"`
	Deletions    int `json:"deletions"`
	ChangedLines int `json:"changed_lines"`
}

// LiveEvent 表示发送给前端的 workspace 实时事件。
type LiveEvent struct {
	Type            string     `json:"type"`
	AgentID         string     `json:"agent_id"`
	Path            string     `json:"path"`
	Version         int        `json:"version"`
	Source          string     `json:"source"`
	SessionKey      *string    `json:"session_key,omitempty"`
	ToolUseID       *string    `json:"tool_use_id,omitempty"`
	ContentSnapshot *string    `json:"content_snapshot,omitempty"`
	AppendedText    *string    `json:"appended_text,omitempty"`
	DiffStats       *DiffStats `json:"diff_stats,omitempty"`
	Timestamp       string     `json:"timestamp"`
}

// LiveListener 表示 workspace 实时事件回调。
type LiveListener func(LiveEvent)
