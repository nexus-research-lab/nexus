package typingloop

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type typingCallRecorder struct {
	mu    sync.Mutex
	calls []bool
}

func (r *typingCallRecorder) signal(_ context.Context, boolValue bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, boolValue)
	return nil
}

func (r *typingCallRecorder) snapshot() []bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]bool(nil), r.calls...)
}

func (r *typingCallRecorder) signalErrorOnActive(_ context.Context, boolValue bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, boolValue)
	if boolValue {
		return errors.New("typing unavailable")
	}
	return nil
}

func TestStartSkipsTypingForQuickStop(t *testing.T) {
	t.Parallel()

	recorder := &typingCallRecorder{}
	stop := Start(context.Background(), recorder.signal, LoopOptions{
		StartDelay:        50 * time.Millisecond,
		KeepaliveInterval: 10 * time.Millisecond,
		CallTimeout:       100 * time.Millisecond,
		StopWait:          100 * time.Millisecond,
	})
	stop()
	time.Sleep(80 * time.Millisecond)

	if calls := recorder.snapshot(); len(calls) != 0 {
		t.Fatalf("快速结束不应发送 typing 状态: %+v", calls)
	}
}

func TestStartKeepsTypingAliveAndStops(t *testing.T) {
	t.Parallel()

	recorder := &typingCallRecorder{}
	stop := Start(context.Background(), recorder.signal, LoopOptions{
		StartDelay:        5 * time.Millisecond,
		KeepaliveInterval: 10 * time.Millisecond,
		CallTimeout:       100 * time.Millisecond,
		StopWait:          100 * time.Millisecond,
	})

	var calls []bool
	deadline := time.After(200 * time.Millisecond)
	for {
		calls = recorder.snapshot()
		if len(calls) >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("未发送 typing keepalive: %+v", calls)
		case <-time.After(5 * time.Millisecond):
		}
	}

	stop()
	calls = recorder.snapshot()
	if len(calls) < 3 {
		t.Fatalf("typing stop 调用不足: %+v", calls)
	}
	if !calls[0] || !calls[1] || calls[len(calls)-1] {
		t.Fatalf("typing 状态顺序不正确: %+v", calls)
	}
}

func TestStartStopsAfterConsecutiveFailures(t *testing.T) {
	t.Parallel()

	recorder := &typingCallRecorder{}
	stop := Start(context.Background(), recorder.signalErrorOnActive, LoopOptions{
		StartDelay:        time.Millisecond,
		KeepaliveInterval: 5 * time.Millisecond,
		CallTimeout:       100 * time.Millisecond,
		StopWait:          100 * time.Millisecond,
		MaxFailures:       2,
	})
	defer stop()

	deadline := time.After(200 * time.Millisecond)
	for {
		calls := recorder.snapshot()
		if len(calls) >= 3 && calls[0] && calls[1] && !calls[2] {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("typing 连续失败后未停止: %+v", calls)
		case <-time.After(5 * time.Millisecond):
		}
	}

	time.Sleep(30 * time.Millisecond)
	calls := recorder.snapshot()
	if len(calls) != 3 {
		t.Fatalf("typing 熔断后不应继续 keepalive: %+v", calls)
	}
}

func TestStartStopsAfterMaxDuration(t *testing.T) {
	t.Parallel()

	recorder := &typingCallRecorder{}
	stop := Start(context.Background(), recorder.signal, LoopOptions{
		StartDelay:        time.Millisecond,
		KeepaliveInterval: 5 * time.Millisecond,
		CallTimeout:       100 * time.Millisecond,
		StopWait:          100 * time.Millisecond,
		MaxDuration:       20 * time.Millisecond,
	})

	deadline := time.After(200 * time.Millisecond)
	for {
		calls := recorder.snapshot()
		if len(calls) >= 2 && !calls[len(calls)-1] {
			beforeStop := len(calls)
			stop()
			afterStop := recorder.snapshot()
			if len(afterStop) != beforeStop {
				t.Fatalf("TTL stop 后显式 stop 不应重复发送: before=%+v after=%+v", calls, afterStop)
			}
			return
		}
		select {
		case <-deadline:
			t.Fatalf("typing 未按最大持续时间停止: %+v", calls)
		case <-time.After(5 * time.Millisecond):
		}
	}
}
