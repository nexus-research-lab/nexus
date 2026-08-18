package dm

import (
	"testing"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRoomBackedSessionOptionsReplaceLocalOverlay(t *testing.T) {
	current := protocol.Session{
		SessionKey: "agent:agent-a:ws:dm:conversation-a",
		AgentID:    "agent-a",
		Options: protocol.WithSessionRuntimeSettings(
			map[string]any{
				protocol.OptionRuntimeProvider: "runtime-provider",
				protocol.OptionRuntimeModel:    "runtime-model",
			},
			protocol.SessionRuntimeSettings{
				Provider:       "old-provider",
				Model:          "old-model",
				PermissionMode: "default",
			},
		),
	}
	roomSession := current
	roomSession.Options = protocol.WithSessionRuntimeSettings(
		nil,
		protocol.SessionRuntimeSettings{
			Provider:       "new-provider",
			Model:          "new-model",
			PermissionMode: "plan",
		},
	)

	if SessionsEqual(current, roomSession) {
		t.Fatal("Session options 变化必须使 Room overlay 失效")
	}
	merged := MergeRoomBackedSession(current, roomSession)
	settings := protocol.SessionRuntimeSettingsFromOptions(merged.Options)
	if settings.Provider != "new-provider" ||
		settings.Model != "new-model" ||
		settings.PermissionMode != "plan" {
		t.Fatalf("Room Session 设置未覆盖本地 overlay: %+v", settings)
	}
	if merged.Options[protocol.OptionRuntimeProvider] != "runtime-provider" ||
		merged.Options[protocol.OptionRuntimeModel] != "runtime-model" {
		t.Fatalf("Room Session 合并不应丢失本地 runtime 指纹: %+v", merged.Options)
	}
}

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
	if StringPointerValue(merged.SessionID) != targetSessionID {
		t.Fatalf("SQL target SDK identity 未投影到 workspace: %+v", merged)
	}
	if _, exists := merged.Options[protocol.OptionRuntimeRetainedTranscriptSessionIDs]; exists {
		t.Fatalf("仅清理 transcript 所有权不应进入 workspace 读模型: %+v", merged.Options)
	}
}

func TestRoomBackedSessionKeepsMonotonicWorkspaceProgress(t *testing.T) {
	older := time.Date(2026, time.August, 12, 1, 0, 0, 0, time.UTC)
	newer := older.Add(5 * time.Minute)
	fileSessionID := "550e8400-e29b-41d4-a716-446655440000"
	current := protocol.Session{
		SessionKey:           "agent:agent-a:ws:dm:conversation-a",
		AgentID:              "agent-a",
		SessionID:            &fileSessionID,
		TranscriptSessionIDs: []string{fileSessionID},
		LastActivity:         newer,
		MessageCount:         17,
		Options:              map[string]any{},
	}
	roomSession := current
	roomSession.SessionID = nil
	roomSession.TranscriptSessionIDs = nil
	roomSession.LastActivity = older
	roomSession.MessageCount = 0

	merged := MergeRoomBackedSession(current, roomSession)
	if merged.MessageCount != 17 || !merged.LastActivity.Equal(newer) {
		t.Fatalf("Room SQL 不应降低 workspace 运行进度: %+v", merged)
	}
	if StringPointerValue(merged.SessionID) != fileSessionID ||
		len(merged.TranscriptSessionIDs) != 1 ||
		merged.TranscriptSessionIDs[0] != fileSessionID {
		t.Fatalf("Room SQL 不应丢失 workspace transcript lineage: %+v", merged)
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

func TestMessageMapperAddsDMContextAndStreamLifecycle(t *testing.T) {
	mapper := NewMessageMapper(
		"agent:nexus:ws:dm:test",
		"nexus",
		"round-1",
		"agent-round-1",
		"user-message-1",
	)
	events, messages, status, subtype, err := mapper.Map(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "sdk-session-1",
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"id":    "assistant-1",
				"model": "test-model",
			},
		}},
	})
	if err != nil {
		t.Fatalf("映射 DM 流事件失败: %v", err)
	}
	if len(messages) != 0 || status != "" || subtype != "" {
		t.Fatalf("流开始不应产生持久消息或终态: messages=%+v status=%q subtype=%q", messages, status, subtype)
	}
	if len(events) != 2 ||
		events[0].EventType != protocol.EventTypeStreamStart ||
		events[1].EventType != protocol.EventTypeStream {
		t.Fatalf("DM 应补充 stream_start: %+v", events)
	}
	for _, event := range events {
		if event.SessionKey != "agent:nexus:ws:dm:test" ||
			event.AgentID != "nexus" ||
			event.RoundID != "round-1" ||
			event.AgentRoundID != "agent-round-1" ||
			event.MessageID != "assistant-1" {
			t.Fatalf("DM 事件身份不完整: %+v", event)
		}
	}
}
