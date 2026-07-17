package operation

import (
	"context"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
)

func TestStagePresenceMatchesSharedAndMemberRuntimeSessionKeys(t *testing.T) {
	t.Parallel()

	service := NewService(config.Config{CacheFileDir: t.TempDir()})
	now := time.Date(2026, time.July, 17, 8, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	if _, err := service.TouchStagePresence(
		context.Background(),
		"room:group:conversation-1",
		"browser-a",
	); err != nil {
		t.Fatalf("TouchStagePresence() error = %v", err)
	}
	if !service.IsStageActive("agent:agent-1:ws:group:conversation-1") {
		t.Fatal("shared Room stage should match member runtime session")
	}
	if service.IsStageActive("agent:agent-1:ws:group:conversation-2") {
		t.Fatal("presence leaked to another conversation")
	}

	if _, err := service.TouchStagePresence(
		context.Background(),
		"agent:agent-1:ws:group:conversation-1",
		"browser-b",
	); err != nil {
		t.Fatalf("second TouchStagePresence() error = %v", err)
	}
	if _, err := service.CloseStagePresence(
		context.Background(),
		"room:group:conversation-1",
		"browser-a",
	); err != nil {
		t.Fatalf("CloseStagePresence() error = %v", err)
	}
	if !service.IsStageActive("agent:agent-1:ws:group:conversation-1") {
		t.Fatal("closing one browser client should preserve another active client")
	}
	if _, err := service.CloseStagePresence(
		context.Background(),
		"room:group:conversation-1",
		"browser-b",
	); err != nil {
		t.Fatalf("second CloseStagePresence() error = %v", err)
	}
	if service.IsStageActive("agent:agent-1:ws:group:conversation-1") {
		t.Fatal("stage should become inactive after all browser clients close")
	}
}

func TestStagePresenceExpiresAbandonedClient(t *testing.T) {
	t.Parallel()

	service := NewService(config.Config{CacheFileDir: t.TempDir()})
	now := time.Date(2026, time.July, 17, 8, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	service.presenceLifetime = 10 * time.Second

	if _, err := service.TouchStagePresence(
		context.Background(),
		"agent:agent-1:ws:dm:conversation-1",
		"browser-a",
	); err != nil {
		t.Fatalf("TouchStagePresence() error = %v", err)
	}
	now = now.Add(11 * time.Second)
	if service.IsStageActive("agent:agent-1:ws:dm:conversation-1") {
		t.Fatal("expired browser client remained active")
	}
}
