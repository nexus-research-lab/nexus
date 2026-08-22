package duework

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestLoopWithoutAuditCoalescesWakeAndStopsWithoutPolling(t *testing.T) {
	loop := New(Options{})
	if loop.auditInterval != 0 {
		t.Fatalf("audit interval = %s, want disabled", loop.auditInterval)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	started := make(chan struct{}, 4)
	releaseStartup := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- loop.Run(ctx, func(context.Context, time.Time) (Result, error) {
			call := calls.Add(1)
			started <- struct{}{}
			if call == 1 {
				<-releaseStartup
			}
			return Result{}, nil
		})
	}()

	waitSignal(t, started)
	for index := 0; index < 100; index++ {
		loop.Notify()
	}
	close(releaseStartup)
	waitSignal(t, started)
	time.Sleep(20 * time.Millisecond)
	if got := calls.Load(); got != 2 {
		t.Fatalf("reconcile calls = %d, want exactly startup plus one coalesced wake", got)
	}
	cancel()
	if err := waitError(t, done); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

func TestLoopUsesExactDeadlineBeforeAudit(t *testing.T) {
	loop := New(Options{AuditInterval: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	done := make(chan error, 1)
	fired := make(chan time.Time, 2)
	go func() {
		done <- loop.Run(ctx, func(_ context.Context, now time.Time) (Result, error) {
			call := calls.Add(1)
			fired <- now
			if call == 1 {
				due := time.Now().Add(40 * time.Millisecond)
				return Result{NextDueAt: &due}, nil
			}
			return Result{}, nil
		})
	}()

	first := waitTime(t, fired)
	second := waitTime(t, fired)
	if elapsed := second.Sub(first); elapsed < 20*time.Millisecond || elapsed > 500*time.Millisecond {
		t.Fatalf("deadline elapsed = %s, want an exact short timer", elapsed)
	}
	cancel()
	if err := waitError(t, done); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

func TestLoopRetriesErrorsAndRejectsConcurrentRun(t *testing.T) {
	errorSeen := make(chan error, 1)
	loop := New(Options{
		AuditInterval: time.Hour,
		ErrorRetry:    20 * time.Millisecond,
		OnError: func(err error) {
			errorSeen <- err
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	done := make(chan error, 1)
	go func() {
		done <- loop.Run(ctx, func(context.Context, time.Time) (Result, error) {
			if calls.Add(1) == 1 {
				return Result{}, errors.New("temporary")
			}
			cancel()
			return Result{}, nil
		})
	}()

	if err := waitError(t, errorSeen); err == nil || err.Error() != "temporary" {
		t.Fatalf("observed error = %v", err)
	}
	if err := loop.Run(context.Background(), func(context.Context, time.Time) (Result, error) {
		return Result{}, nil
	}); err == nil {
		t.Fatal("concurrent Run should fail")
	}
	if err := waitError(t, done); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("reconcile calls = %d, want retry after error", calls.Load())
	}
}

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

func waitSignal(t *testing.T, channel <-chan struct{}) {
	t.Helper()
	select {
	case <-channel:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for signal")
	}
}

func waitTime(t *testing.T, channel <-chan time.Time) time.Time {
	t.Helper()
	select {
	case value := <-channel:
		return value
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for time")
		return time.Time{}
	}
}

func waitError(t *testing.T, channel <-chan error) error {
	t.Helper()
	select {
	case err := <-channel:
		return err
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for result")
		return nil
	}
}
