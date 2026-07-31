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
