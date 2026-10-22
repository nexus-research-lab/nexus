package runtimehost

import (
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestContextAnnouncementsOnlyReportChangesUntilCompact(t *testing.T) {
	announcements := &ContextAnnouncements{}
	steps := []struct {
		content string
		want    bool
	}{
		{"state-a", true},
		{"state-a", false},
		{"state-b", true},
		{"state-b", false},
	}
	for index, step := range steps {
		if got := announcements.Changed("session", "connectors", step.content); got != step.want {
			t.Fatalf("step %d Changed(%q) = %v, want %v", index, step.content, got, step.want)
		}
	}
	if !announcements.Changed("other-session", "connectors", "state-b") {
		t.Fatal("sessions must track announcements independently")
	}

	announcements.ForgetAfterCompact("session", sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeAssistant})
	if announcements.Changed("session", "connectors", "state-b") {
		t.Fatal("non-boundary messages must keep announcements")
	}
	announcements.ForgetAfterCompact("session", sdkprotocol.ReceivedMessage{
		Type:   sdkprotocol.MessageTypeSystem,
		System: &sdkprotocol.SystemMessage{Subtype: "compact_boundary"},
	})
	if !announcements.Changed("session", "connectors", "state-b") {
		t.Fatal("compact boundary must force the next announcement")
	}
}
