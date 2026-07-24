package workspace

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestUpsertSessionCreatesMissingWorkspace(t *testing.T) {
	storeRoot := t.TempDir()
	workspacePath := filepath.Join(storeRoot, "owner", "workspace", "agent")
	store := NewSessionFileStore(storeRoot)
	now := time.Now().UTC()

	created, err := store.UpsertSession(workspacePath, protocol.Session{
		SessionKey:   "agent:test:websocket:dm:user",
		AgentID:      "test",
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("UpsertSession() error = %v", err)
	}
	if created == nil || created.SessionKey != "agent:test:websocket:dm:user" {
		t.Fatalf("UpsertSession() created = %+v", created)
	}
}
