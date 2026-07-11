package session

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// CreateRequest 表示创建会话请求。
type CreateRequest struct {
	SessionKey string `json:"session_key"`
	AgentID    string `json:"agent_id,omitempty"`
	Title      string `json:"title,omitempty"`
}

// UpdateRequest 表示更新会话请求。
type UpdateRequest struct {
	Title *string `json:"title,omitempty"`
}

// DirectoryNotifier 接收会话目录变更通知。
type DirectoryNotifier interface {
	NotifyDirectoryChanged(context.Context, string, protocol.Session)
}

// DirectoryNotifierFunc 适配函数式会话目录通知器。
type DirectoryNotifierFunc func(context.Context, string, protocol.Session)

// NotifyDirectoryChanged 实现 DirectoryNotifier。
func (fn DirectoryNotifierFunc) NotifyDirectoryChanged(ctx context.Context, reason string, session protocol.Session) {
	if fn != nil {
		fn(ctx, strings.TrimSpace(reason), session)
	}
}

// MessagePageRequest 表示消息分页读取请求。
type MessagePageRequest struct {
	Limit                int
	BeforeRoundID        string
	BeforeRoundTimestamp int64
	AroundRoundID        string
	AroundLimit          int
}

// TurnPageRequest 表示 turn 投影分页读取请求。
type TurnPageRequest struct {
	Limit         int
	BeforeRoundID string
	AroundRoundID string
	Sort          string // asc | desc
	View          string // summary | full
}
