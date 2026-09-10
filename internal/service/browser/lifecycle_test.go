package browser

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCommandDeadlineCancelsOriginalConnectionAndRejectsLateResult(t *testing.T) {
	service := NewService()
	messages := make(chan map[string]any, 4)
	clientID, detach := service.Attach("0.8.5", "Chrome", "browser", "generation", func(ctx context.Context, payload any) error {
		if err := ctx.Err(); err != nil {
			t.Errorf("sender received cancelled context: %v", err)
		}
		messages <- payload.(map[string]any)
		return nil
	}, nil)
	defer detach()
	done := make(chan error, 1)
	go func() {
		_, err := service.sendCommand(context.Background(), "scroll", map[string]any{"session": "s"}, 30*time.Millisecond)
		done <- err
	}()
	command := <-messages
	budget, ok := command["budget_ms"].(int64)
	if !ok || budget <= 0 || budget > 30 {
		t.Fatalf("budget = %v", command["budget_ms"])
	}
	if err := <-done; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
	cancel := <-messages
	if cancel["type"] != "browser.cancel" || cancel["id"] != command["id"] {
		t.Fatalf("cancel = %v", cancel)
	}
	if service.Resolve(clientID, command["id"].(string), map[string]any{"ok": true}, "") {
		t.Fatal("late result accepted")
	}
}

func TestOldConnectionCannotResolveOrChangeHealth(t *testing.T) {
	service := NewService()
	oldID, _ := service.Attach("0.8.5", "Chrome", "browser", "generation", func(context.Context, any) error { return nil }, nil)
	commands := make(chan map[string]any, 1)
	newID, detach := service.Attach("0.8.5", "Chrome", "browser", "generation", func(_ context.Context, p any) error { commands <- p.(map[string]any); return nil }, nil)
	defer detach()
	done := make(chan error, 1)
	go func() {
		_, err := service.sendCommand(context.Background(), "list_tabs", nil, time.Second)
		done <- err
	}()
	command := <-commands
	id := command["id"].(string)
	if service.Resolve(oldID, id, nil, "old") {
		t.Fatal("accepted stale response")
	}
	if service.ObserveProgress(oldID, id, map[string]any{"stage": "unknown"}) {
		t.Fatal("accepted stale progress")
	}
	if service.ObserveHealth(oldID, map[string]any{"execution_state": "ready"}) {
		t.Fatal("accepted stale heartbeat")
	}
	if !service.Resolve(newID, id, map[string]any{}, "") {
		t.Fatal("rejected current response")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestStatusDoesNotClaimRetainedConnectionIsHealthy(t *testing.T) {
	service := NewService()
	id, detach := service.Attach("0.8.5", "Chrome", "browser", "generation", func(context.Context, any) error { return nil }, nil)
	defer detach()
	if service.Status()["execution_state"] != "unverified" {
		t.Fatal("connection falsely marked healthy")
	}
	service.ObserveHealth(id, map[string]any{"execution_state": "recovery_required"})
	if service.Status()["execution_state"] != "recovery_required" {
		t.Fatal("missing recovery state")
	}
}

func TestResolvedOldConnectionCannotRepopulateSessionAfterReplacement(t *testing.T) {
	service := NewService()
	oldID, _ := service.Attach("0.8.5", "Chrome", "browser", "old", func(context.Context, any) error { return nil }, nil)
	_, detach := service.Attach("0.8.5", "Chrome", "browser", "new", func(context.Context, any) error { return nil }, nil)
	defer detach()
	service.updateSession(oldID, "s", "r", "navigate", nil, map[string]any{"tab_id": 42, "tab_ref": "old-ref"})
	if _, exists := service.sessions["s"]; exists {
		t.Fatal("old command repopulated replacement session")
	}
}
