package message

import (
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestIsRuntimeTurnStartOnlyMatchesTopLevelInit(t *testing.T) {
	parent := "toolu_parent"
	cases := []struct {
		name    string
		message sdkprotocol.ReceivedMessage
		want    bool
	}{
		{"top-level init", sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeSystem, Subtype: "init"}, true},
		{"decoded init", sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeSystem, System: &sdkprotocol.SystemMessage{Subtype: "init"}}, true},
		{"child init", sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeSystem, Subtype: "init", ParentToolUseID: &parent}, false},
		{"task notification", sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeSystem, Subtype: "task_notification"}, false},
		{"assistant", sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeAssistant}, false},
	}
	for _, tc := range cases {
		if got := IsRuntimeTurnStart(tc.message); got != tc.want {
			t.Fatalf("%s: IsRuntimeTurnStart = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// 内部 round 的用户行默认隐藏；装饰后必须作为可见触发行。
func TestDecorateRuntimeWakeUserMessageUnhidesTriggerRow(t *testing.T) {
	row := DecorateRuntimeWakeUserMessage(protocol.Message{"hidden_from_user": true, "content": "x"}, RuntimeWakeTrigger{TaskID: "b1", Status: "completed", Summary: "done"})
	if _, hidden := row["hidden_from_user"]; hidden || row["content"] != "done" || row["purpose"] != RuntimeWakePurpose {
		t.Fatalf("row = %+v", row)
	}
}
