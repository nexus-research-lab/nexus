package browser

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

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

func TestCommandProgressUsesConfiguredLoggerWithoutArguments(t *testing.T) {
	var output bytes.Buffer
	service := NewService()
	service.SetLogger(slog.New(slog.NewJSONHandler(&output, nil)))
	id, detach := service.Attach("0.8.6", "Chrome", "browser", "generation", func(context.Context, any) error { return nil }, nil)
	defer detach()
	service.pending["test"] = make(chan commandResponse, 1)
	if !service.ObserveProgress(id, "test", map[string]any{"stage": "api_start", "method": "Input.dispatchMouseEvent", "elapsed_ms": 1, "params": "private-script"}) {
		t.Fatal("progress rejected")
	}
	if !strings.Contains(output.String(), "Input.dispatchMouseEvent") || strings.Contains(output.String(), "private-script") {
		t.Fatalf("unexpected diagnostics: %s", output.String())
	}
}
