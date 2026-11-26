// INPUT: 常驻对话收到的一条输入（用户消息或定时任务到点）。
// OUTPUT: inbox 事件、可还原本轮输入的 payload 与状态词汇。
// POS: inbox 的数据模型；payload 只在宿主数据库中，Agent 工作区不可写，因此本身即可信来源。
package inbox

import (
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// 状态只能单向推进；delivering 是"已在安全点交给 runtime、等待 applied"的中间态。
const (
	StatusPending    = "pending"
	StatusDelivering = "delivering"
	StatusConsumed   = "consumed"
	StatusWithdrawn  = "withdrawn"
)

// 事件类型：用户输入，或宿主代为写入的定时任务到点。
const (
	KindUserMessage   = "user_message"
	KindScheduledTask = "scheduled_task"
)

// Event 是一条 inbox 事件。InboxKey 即常驻对话的 session key。
type Event struct {
	EventID     string
	OwnerUserID string
	InboxKey    string
	Seq         int64
	Kind        string
	DedupeKey   string
	Payload     Payload
	Status      string
	RoundID     string
	CreatedAt   time.Time
}

// Payload 保存还原这条输入所需的全部字段；轮次身份在被取走时才分配。
type Payload struct {
	Content         string                    `json:"content"`
	Attachments     []protocol.ChatAttachment `json:"attachments,omitempty"`
	ClientMessageID string                    `json:"client_message_id,omitempty"`
	Principal       Principal                 `json:"principal"`
	// Task 只在 scheduled_task 事件上出现，标明是哪个任务的哪次运行。
	Task *TaskSource `json:"task,omitempty"`
}

// TaskSource 是定时任务事件的来源身份。
type TaskSource struct {
	JobID string `json:"job_id"`
	RunID string `json:"run_id"`
	Name  string `json:"name"`
}

// Principal 是提交这条输入的真人身份；SessionID 是宿主认证记录 ID，不是凭据。
type Principal struct {
	UserID     string `json:"user_id"`
	AuthMethod string `json:"auth_method"`
	SessionID  string `json:"session_id"`
}
