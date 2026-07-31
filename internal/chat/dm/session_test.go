package dm

import (
	"testing"

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
