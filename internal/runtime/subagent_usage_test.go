package runtime

import "testing"

func TestObserveSubagentUsageUsesSessionTaskHighWater(t *testing.T) {
	manager := NewManager()

	if got := manager.ObserveSubagentUsage("session-a", "task-1", 100); got != 100 {
		t.Fatalf("first delta = %d, want 100", got)
	}
	if got := manager.ObserveSubagentUsage("session-a", "task-1", 150); got != 50 {
		t.Fatalf("second delta = %d, want 50", got)
	}
	if got := manager.ObserveSubagentUsage("session-a", "task-1", 150); got != 0 {
		t.Fatalf("duplicate delta = %d, want 0", got)
	}
	if got := manager.ObserveSubagentUsage("session-a", "task-1", 120); got != 0 {
		t.Fatalf("out-of-order delta = %d, want 0", got)
	}
	if got := manager.ObserveSubagentUsage("session-a", "task-1", 180); got != 30 {
		t.Fatalf("later delta = %d, want 30", got)
	}
	manager.mu.Lock()
	delete(manager.sessions, "session-a")
	manager.mu.Unlock()
	if got := manager.ObserveSubagentUsage("session-a", "task-1", 200); got != 20 {
		t.Fatalf("delta after idle state removal = %d, want retained high-water delta 20", got)
	}
	if got := manager.ObserveSubagentUsage("session-b", "task-1", 180); got != 180 {
		t.Fatalf("other session delta = %d, want 180", got)
	}
}
