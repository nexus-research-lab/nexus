package room

import "github.com/nexus-research-lab/nexus/internal/protocol"

// VisibleContextInput 描述一次 Room 成员被唤醒时可见的公区、私域和触发上下文。
type VisibleContextInput struct {
	PublicMessages      []protocol.Message
	RoomMessages        []protocol.RoomDirectedMessageRecord
	LatestTrigger       Trigger
	AgentNameByID       map[string]string
	TargetAgentID       string
	ContextWindowTokens int
	ColdStart           bool
	PublicAnchor        PublicAnchorMetadata
}

// Trigger 描述 Room round 里唤醒单个成员的直接原因。
type Trigger struct {
	TriggerType   string
	Content       string
	MessageID     string
	SourceAgentID string
	TargetAgentID string
	ReplyRoute    protocol.RoomReplyRoute
}

// BuildVisibleContext 构建 Room 成员本轮动态输入。
func BuildVisibleContext(input VisibleContextInput) string {
	return BuildVisibleContextPlan(input).Text
}

// BuildGuidedPublicInputContext 构造运行中 round 的公区增量引导文本。
func BuildGuidedPublicInputContext(input VisibleContextInput) string {
	return BuildGuidedPublicInputContextPlan(input).Text
}
