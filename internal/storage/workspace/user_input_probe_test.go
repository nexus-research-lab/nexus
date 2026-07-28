package workspace

import (
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRoomHistoryHasCanonicalUserInputIgnoresHiddenAndSyntheticUsers(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	store := NewRoomHistoryStore(filepath.Join(stateRoot, "users"))
	ownerUserID := "user-input-probe-owner"
	conversationID := "conversation-user-input-probe"

	for _, message := range []protocol.Message{
		{
			"message_id":       "hidden-user",
			"role":             "user",
			"content":          "internal continuation",
			"hidden_from_user": true,
		},
		{
			"message_id":   "synthetic-user",
			"role":         "user",
			"content":      "synthetic continuation",
			"is_synthetic": true,
		},
		{
			"message_id": "assistant-only",
			"role":       "assistant",
			"content":    "background output",
		},
	} {
		if err := store.AppendInlineMessage(ownerUserID, conversationID, message); err != nil {
			t.Fatalf("写入非用户输入 fixture: %v", err)
		}
	}

	hasUserInput, err := store.HasCanonicalUserInput(ownerUserID, conversationID)
	if err != nil {
		t.Fatalf("探测非用户输入 fixture: %v", err)
	}
	if hasUserInput {
		t.Fatal("隐藏、synthetic 或 assistant 消息不得作为真实用户输入")
	}

	if err = store.AppendInlineMessage(ownerUserID, conversationID, protocol.Message{
		"message_id": "visible-user",
		"role":       "user",
		"content":    "real user input",
	}); err != nil {
		t.Fatalf("写入真实用户输入 fixture: %v", err)
	}
	hasUserInput, err = store.HasCanonicalUserInput(ownerUserID, conversationID)
	if err != nil {
		t.Fatalf("探测真实用户输入 fixture: %v", err)
	}
	if !hasUserInput {
		t.Fatal("可见的非 synthetic user 消息必须作为真实用户输入")
	}
}
