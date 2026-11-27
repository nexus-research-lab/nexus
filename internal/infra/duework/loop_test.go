package duework

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestNextBackoffIsExponentialAndBounded(t *testing.T) {
	backoff := time.Second
	maximum := 5 * time.Second
	want := []time.Duration{2 * time.Second, 4 * time.Second, 5 * time.Second, 5 * time.Second}
	for index, expected := range want {
		backoff = nextBackoff(backoff, maximum)
		if backoff != expected {
			t.Fatalf("step %d backoff = %s, want %s", index, backoff, expected)
		}
	}
}

type delayedError time.Duration

func (e delayedError) Error() string             { return "slow down" }
func (e delayedError) RetryDelay() time.Duration { return time.Duration(e) }

func TestLoopHonorsRetryDelayOverNotify(t *testing.T) {
	loop := New(Options{ErrorRetry: time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	var failedAt time.Time
	done := make(chan time.Duration, 1)
	go func() {
		_ = loop.Run(ctx, func(context.Context, time.Time) (Result, error) {
			if calls.Add(1) == 1 {
				failedAt = time.Now()
				return Result{}, delayedError(80 * time.Millisecond)
			}
			done <- time.Since(failedAt)
			cancel()
			return Result{}, nil
		})
	}()
	time.Sleep(10 * time.Millisecond)
	loop.Notify()
	select {
	case elapsed := <-done:
		if elapsed < 80*time.Millisecond {
			t.Fatalf("Notify bypassed Retry-After: %s", elapsed)
		}
	case <-time.After(time.Second):
		t.Fatal("loop never retried")
	}
}
