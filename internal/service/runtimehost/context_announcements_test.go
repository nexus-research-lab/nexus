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
		if got := announcements.Changed("session", "sdk-1", "connectors", step.content); got != step.want {
			t.Fatalf("step %d Changed(%q) = %v, want %v", index, step.content, got, step.want)
		}
	}
	if !announcements.Changed("other-session", "sdk-1", "connectors", "state-b") {
		t.Fatal("sessions must track announcements independently")
	}

	announcements.ForgetAfterCompact("session", sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeAssistant})
	if announcements.Changed("session", "sdk-1", "connectors", "state-b") {
		t.Fatal("non-boundary messages must keep announcements")
	}
	announcements.ForgetAfterCompact("session", sdkprotocol.ReceivedMessage{
		Type:   sdkprotocol.MessageTypeSystem,
		System: &sdkprotocol.SystemMessage{Subtype: "compact_boundary"},
	})
	if !announcements.Changed("session", "sdk-1", "connectors", "state-b") {
		t.Fatal("compact boundary must force the next announcement")
	}
}

func TestContextAnnouncementsFollowSDKSessionGeneration(t *testing.T) {
	announcements := &ContextAnnouncements{}
	// 首轮还没有 SDK session id，补上 id 后仍是同一会话，不重复告知。
	if !announcements.Changed("session", "", "connectors", "state-a") {
		t.Fatal("first announcement must be reported")
	}
	if announcements.Changed("session", "sdk-1", "connectors", "state-a") {
		t.Fatal("learning the SDK session id must not repeat the announcement")
	}
	// 换成新的 SDK 会话后，模型看不到旧告知。
	if !announcements.Changed("session", "sdk-2", "connectors", "state-a") {
		t.Fatal("a new SDK session must receive the announcement again")
	}
}
