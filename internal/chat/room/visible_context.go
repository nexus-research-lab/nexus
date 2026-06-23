package room

import (
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	roomMaxHistoryMessages = 80
	roomMaxHistoryChars    = 12_000
)

// VisibleContextInput 描述一次 Room 成员被唤醒时可见的公共上下文。
type VisibleContextInput struct {
	PublicMessages []protocol.Message
	RoomMessages   []protocol.RoomDirectedMessageRecord
	LatestTrigger  Trigger
	AgentNameByID  map[string]string
	TargetAgentID  string
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
	lines := buildHistoryLines(contextPublicMessages(input.PublicMessages, input.LatestTrigger), input.AgentNameByID)
	if len(lines) == 0 {
		lines = []string{"(No new public messages this turn.)"}
	}

	contextValue := fmt.Sprintf(
		"<public_feed>\n%s\n</public_feed>\n\n"+
			"<latest_trigger>\n%s\n</latest_trigger>",
		strings.Join(lines, "\n"),
		formatRoomTrigger(input.LatestTrigger, input.AgentNameByID),
	)
	if privateContext := buildRoomDirectedMessageContext(input.RoomMessages, input.AgentNameByID, input.TargetAgentID); privateContext != "" {
		contextValue += "\n\n" + privateContext
	}
	return contextValue
}

// BuildGuidedPublicInputContext 构造运行中 round 的公区增量引导文本。
func BuildGuidedPublicInputContext(input VisibleContextInput) string {
	lines := buildHistoryLines(contextPublicMessages(input.PublicMessages, input.LatestTrigger), input.AgentNameByID)
	if len(lines) == 0 {
		triggerType := strings.TrimSpace(input.LatestTrigger.TriggerType)
		triggerContent := strings.TrimSpace(input.LatestTrigger.Content)
		if triggerType == "" && triggerContent == "" {
			return ""
		}
		lines = []string{"(No new public messages this turn.)"}
	}
	return fmt.Sprintf(
		"New public Room messages arrived while you were running. Treat them as public facts already in the Room. If they affect your current work, incorporate them and continue.\n\n"+
			"<public_feed>\n%s\n</public_feed>\n\n"+
			"<latest_trigger>\n%s\n</latest_trigger>",
		strings.Join(lines, "\n"),
		formatRoomTrigger(input.LatestTrigger, input.AgentNameByID),
	)
}
