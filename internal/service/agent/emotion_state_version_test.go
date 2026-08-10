package agent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestRuntimeEmotionVersionCASSerializesConcurrentWriters(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	initial := LoadRuntimeEmotionView(workspace, "dm:test", now)
	if initial.Version != 1 {
		t.Fatalf("initial emotion version = %d, want 1", initial.Version)
	}

	first, err := SetRuntimeEmotionBaseAtVersion(
		workspace,
		RuntimeEmotionBaseUpdate{
			Mood: "curious", Energy: 7, Valence: 8,
			Description: "ready to explore", Timestamp: now,
		},
		initial.Version,
	)
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != 2 || first.Base.Mood != "curious" {
		t.Fatalf("first CAS result = %+v", first)
	}
	if _, err = SetRuntimeEmotionBaseAtVersion(
		workspace,
		RuntimeEmotionBaseUpdate{
			Mood: "stale", Energy: 1, Valence: 1,
			Description: "must not win", Timestamp: now,
		},
		initial.Version,
	); !errors.Is(err, ErrRuntimeEmotionVersionConflict) {
		t.Fatalf("stale emotion write error = %v", err)
	}

	var wait sync.WaitGroup
	results := make(chan error, 2)
	for _, mood := range []string{"steady", "bright"} {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, writeErr := SetRuntimeEmotionContextAtVersion(
				workspace,
				RuntimeEmotionContextUpdate{
					ContextID: "dm:test", Mood: mood,
					Valence: 7, Trigger: "concurrent update", Timestamp: now,
				},
				first.Version,
			)
			results <- writeErr
		}()
	}
	wait.Wait()
	close(results)
	successes := 0
	conflicts := 0
	for resultErr := range results {
		switch {
		case resultErr == nil:
			successes++
		case errors.Is(resultErr, ErrRuntimeEmotionVersionConflict):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent emotion error: %v", resultErr)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("emotion concurrent results: successes=%d conflicts=%d", successes, conflicts)
	}
	current := LoadRuntimeEmotionView(workspace, "dm:test", now)
	if current.Version != 3 || current.Context == nil {
		t.Fatalf("final emotion state = %+v", current)
	}
	safe := SafeRuntimeEmotionView(current)
	payload, err := json.Marshal(safe)
	if err != nil {
		t.Fatal(err)
	}
	if safe.StatePath != "" || string(payload) == "" ||
		json.Valid(payload) && containsJSONField(payload, "state_path") {
		t.Fatalf("safe emotion view leaked state path: %s", payload)
	}
}

func containsJSONField(payload []byte, field string) bool {
	var values map[string]any
	if json.Unmarshal(payload, &values) != nil {
		return false
	}
	_, ok := values[field]
	return ok
}
