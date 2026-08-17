package server

import (
	"testing"
	"time"
)

func TestSubagentReconciliationDelayUsesOnlyTheNearestDeadline(t *testing.T) {
	now := time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC)
	if delay, armed := subagentReconciliationDelay(now, nil); armed || delay != 0 {
		t.Fatalf("empty deadline delay=%s armed=%t", delay, armed)
	}
	deadline := now.Add(17 * time.Second)
	if delay, armed := subagentReconciliationDelay(now, &deadline); !armed || delay != 17*time.Second {
		t.Fatalf("future deadline delay=%s armed=%t", delay, armed)
	}
	expired := now.Add(-time.Second)
	if delay, armed := subagentReconciliationDelay(now, &expired); !armed || delay != subagentReconcileRetryMinDelay {
		t.Fatalf("expired deadline delay=%s armed=%t", delay, armed)
	}
}

func TestSubagentReconciliationRetryBackoffIsBounded(t *testing.T) {
	if delay := subagentReconciliationRetryBackoff(0); delay != time.Second {
		t.Fatalf("first retry delay=%s", delay)
	}
	if delay := subagentReconciliationRetryBackoff(4 * time.Second); delay != 8*time.Second {
		t.Fatalf("next retry delay=%s", delay)
	}
	if delay := subagentReconciliationRetryBackoff(16 * time.Second); delay != 30*time.Second {
		t.Fatalf("bounded retry delay=%s", delay)
	}
}
