package duework

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

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
