// INPUT: Session command、分页与派生 detail 的服务层数据。
// OUTPUT: Session service 的窄请求/响应与通知接口。
// POS: 不进入 wire 真相源的 Session 业务 DTO。
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
	DeferIndex           bool
}

// MessageDetail 是消息页中大型 Tool result / 图片引用的完整内容。
type MessageDetail struct {
	Ref       string
	Kind      string
	MediaType string
	ByteSize  int64
	Payload   []byte
}
