// INPUT: scheduled task 写入 automation_system_events 的主会话触发事件行。
// OUTPUT: Main Session 派发器领取与收口所需的最小事件快照。
// POS: 主会话事件词汇；只承载 scheduled_task.trigger，不再有 heartbeat wake outbox。
package types

import "time"

// SystemEvent 表示 Main Session 派发器消费的一条系统事件。
type SystemEvent struct {
	EventID    string
	EventType  string
	SourceType string
	SourceID   string
	Payload    string
	Status     string
	CreatedAt  time.Time
}
