package server

import "testing"

func TestResolveGoalMCPSessionKeyUsesSharedRoomGoalForGroupRoom(t *testing.T) {
	got := resolveGoalMCPSessionKey(
		"agent:devin:ws:group:conversation-1",
		"room",
	)

	if got != "room:group:conversation-1" {
		t.Fatalf("resolveGoalMCPSessionKey() = %q, want shared room goal key", got)
	}
}

func TestResolveGoalMCPSessionKeyKeepsRoomSharedKey(t *testing.T) {
	got := resolveGoalMCPSessionKey("room:group:conversation-1", "room")

	if got != "room:group:conversation-1" {
		t.Fatalf("resolveGoalMCPSessionKey() = %q, want unchanged shared room key", got)
	}
}

func TestResolveGoalMCPSessionKeyKeepsRoomDMOnAgentGoal(t *testing.T) {
	got := resolveGoalMCPSessionKey(
		"agent:devin:ws:dm:conversation-1",
		"room",
	)

	if got != "agent:devin:ws:dm:conversation-1" {
		t.Fatalf("resolveGoalMCPSessionKey() = %q, want unchanged room dm key", got)
	}
}

func TestResolveGoalMCPSessionKeyKeepsNonRoomSession(t *testing.T) {
	got := resolveGoalMCPSessionKey(
		"agent:devin:ws:group:conversation-1",
		"automation",
	)

	if got != "agent:devin:ws:group:conversation-1" {
		t.Fatalf("resolveGoalMCPSessionKey() = %q, want unchanged non-room key", got)
	}
}
