package dm

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRoomBackedSessionSQLClearsMaterializedForkDependency(t *testing.T) {
	targetSessionID := "target-sdk-session"
	current := protocol.Session{
		SessionKey: "agent:agent-a:ws:dm:conversation-fork",
		AgentID:    "agent-a",
		Options: map[string]any{
			protocol.OptionRuntimeForkSourceSessionID: "source-sdk-session",
			protocol.OptionRuntimeForkMessageID:       "source-boundary",
		},
	}
	roomSession := current
	roomSession.SessionID = &targetSessionID
	roomSession.Options = map[string]any{
		protocol.OptionRuntimeRetainedTranscriptSessionIDs: []string{"source-sdk-session"},
	}

	merged := MergeRoomBackedSession(current, roomSession)
	if _, exists := merged.Options[protocol.OptionRuntimeForkSourceSessionID]; exists {
		t.Fatalf("SQL 已物化 fork 后不应恢复 source 依赖: %+v", merged.Options)
	}
	if _, exists := merged.Options[protocol.OptionRuntimeForkMessageID]; exists {
		t.Fatalf("SQL 已物化 fork 后不应恢复 message 边界: %+v", merged.Options)
	}
	if textutil.PointerValue(merged.SessionID) != targetSessionID {
		t.Fatalf("SQL target SDK identity 未投影到 workspace: %+v", merged)
	}
	if _, exists := merged.Options[protocol.OptionRuntimeRetainedTranscriptSessionIDs]; exists {
		t.Fatalf("仅清理 transcript 所有权不应进入 workspace 读模型: %+v", merged.Options)
	}
}

func TestRoomBackedSessionKeepsLocalContextUsage(t *testing.T) {
	current := protocol.Session{
		SessionKey: "agent:agent-a:ws:group:conversation-a",
		AgentID:    "agent-a",
		ContextUsage: &protocol.ContextUsageData{
			TotalTokens: 37_500,
			MaxTokens:   131_100,
			Percentage:  28.6,
			Model:       "glm-4.5-air",
		},
		Options: map[string]any{},
	}
	roomSession := current
	roomSession.ContextUsage = nil

	merged := MergeRoomBackedSession(current, roomSession)
	if merged.ContextUsage == nil || *merged.ContextUsage != *current.ContextUsage {
		t.Fatalf("Room Session 合并丢失 context usage: %+v", merged.ContextUsage)
	}
	if SessionsEqual(current, roomSession) {
		t.Fatal("context usage 变化必须触发本地 overlay 刷新")
	}
}
