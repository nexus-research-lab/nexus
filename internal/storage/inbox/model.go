// INPUT: 常驻对话收到的一条输入（用户消息、定时任务到点、agent 自设跟进、Echo、WorkGraph 宿主恢复或 IM 回执）。
// OUTPUT: inbox 事件、可还原本轮输入的 payload、状态词汇与暂停闸门。
// POS: inbox 的数据模型；payload 只在宿主数据库中，Agent 工作区不可写，因此本身即可信来源。
package inbox

import (
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// 状态只能单向推进；delivering 是"已在安全点交给 runtime、等待 applied"的中间态。
// withdrawn 是用户撤回，cancelled 是到点前失效（Echo 被新输入取消、agent 取消跟进）。
const (
	StatusPending    = "pending"
	StatusDelivering = "delivering"
	StatusConsumed   = "consumed"
	StatusWithdrawn  = "withdrawn"
	StatusCancelled  = "cancelled"
)

// 事件类型：用户输入，或宿主代为写入的定时任务到点、agent 自设跟进、Echo、WorkGraph 恢复与 IM 回执。
const (
	KindUserMessage   = "user_message"
	KindScheduledTask = "scheduled_task"
	KindFollowUp      = "follow_up"
	KindEcho          = "echo"
	KindHostRecovery  = "host_recovery"
	KindIMReply       = "im_reply"
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
	// AvailableAt 为零即立即可取；未来时间的事件到点前对起轮与安全点不可见。
	AvailableAt time.Time
	CreatedAt   time.Time
}

// Payload 保存还原这条输入所需的全部字段；轮次身份在被取走时才分配。
// follow_up 的 Content 是 agent 写给未来自己的备注；echo 不带内容，到点由 gate 决定说什么。
type Payload struct {
	Content         string                    `json:"content"`
	Attachments     []protocol.ChatAttachment `json:"attachments,omitempty"`
	ClientMessageID string                    `json:"client_message_id,omitempty"`
	Principal       Principal                 `json:"principal"`
	// Task 只在 scheduled_task 事件上出现，标明是哪个任务的哪次运行。
	Task *TaskSource `json:"task,omitempty"`
	// Recovery 只在 host_recovery 事件上出现。
	Recovery *RecoverySource `json:"recovery,omitempty"`
	// IMReply 只在 im_reply 事件上出现。
	IMReply *IMReplySource `json:"im_reply,omitempty"`
}

// RecoverySource 是 WorkGraph 交给协调者的宿主恢复输入；绑定只由宿主签发，模型无法构造。
type RecoverySource struct {
	ExecutionID string                            `json:"execution_id"`
	Target      *protocol.ExecutionRecoveryTarget `json:"target,omitempty"`
	Work        *protocol.ExecutionWorkBinding    `json:"work,omitempty"`
	Review      *protocol.ExecutionReviewBinding  `json:"review,omitempty"`
	// PauseRevision 是续跑计划读到的闸门代数；入队时已不是当前代数即拒绝。
	PauseRevision *int64 `json:"pause_revision,omitempty"`
	// AdmittedRevision 非空表示入队时获准越过暂停；之后再有停止或恢复即失效。
	AdmittedRevision *int64 `json:"admitted_revision,omitempty"`
}

// IMReplySource 是外部 IM 对 agent 消息的回执身份；状态机在 imdelivery 仓储里。
type IMReplySource struct {
	ReplyID string `json:"reply_id"`
}

// Gate 是一个 inbox 的暂停闸门：只挡宿主恢复输入，用户输入与定时事件不受影响。
// Revision 在每次停止或恢复时递增，续跑计划据此判断自己是否已被取代。
type Gate struct {
	Paused   bool
	Revision int64
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
