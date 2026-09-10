package auth

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

type invalidationFixture struct {
	events                []ControlIdentityInvalidation
	cursor                int64
	calls                 []string
	cancel                context.CancelFunc
	applyFailures         int
	commitFailures        int
	fetchFailures         int
	runtimeFailures       int
	allFailures           int
	failClosedCalls       int
	stopAfterFetchFailure bool
}

func (f *invalidationFixture) ControlIdentityInvalidationCursor(context.Context) (int64, error) {
	return f.cursor, nil
}
func (f *invalidationFixture) CommitControlIdentityInvalidationCursor(_ context.Context, id int64) error {
	f.calls = append(f.calls, fmt.Sprintf("commit:%d", id))
	if f.commitFailures > 0 {
		f.commitFailures--
		return errors.New("commit failed")
	}
	f.cursor = id
	return nil
}
func (f *invalidationFixture) ControlIdentityInvalidations(_ context.Context, after int64) ([]ControlIdentityInvalidation, error) {
	if f.fetchFailures > 0 {
		f.fetchFailures--
		return nil, errors.New("Control unavailable")
	}
	if f.stopAfterFetchFailure || after >= int64(len(f.events)) {
		f.cancel()
		return nil, nil
	}
	return f.events[after:], nil
}
func (f *invalidationFixture) ApplyControlIdentityInvalidation(_ context.Context, event ControlIdentityInvalidation) (string, error) {
	f.calls = append(f.calls, fmt.Sprintf("apply:%d", event.EventID))
	if f.applyFailures > 0 {
		f.applyFailures--
		return "owner-a", errors.New("apply failed")
	}
	return "owner-a", nil
}
func (f *invalidationFixture) FailClosedControlIdentities(context.Context) ([]string, error) {
	f.failClosedCalls++
	if f.allFailures > 0 {
		f.allFailures--
		return []string{"owner-a"}, errors.New("projection failed")
	}
	return []string{"owner-a"}, nil
}
func (f *invalidationFixture) CloseControlConnections() int {
	f.calls = append(f.calls, "connections:all")
	return 1
}
func (f *invalidationFixture) CloseControlSessionConnections(id string) int {
	f.calls = append(f.calls, "session:"+id)
	return 1
}
func (f *invalidationFixture) CloseOwnerConnections(id string) int {
	f.calls = append(f.calls, "owner:"+id)
	return 1
}
func (f *invalidationFixture) CloseOwnerSessions(_ context.Context, id string) (int, error) {
	f.calls = append(f.calls, "runtime:"+id)
	if f.runtimeFailures > 0 {
		f.runtimeFailures--
		return 0, errors.New("runtime close failed")
	}
	return 1, nil
}

func runInvalidationFixture(t *testing.T, f *invalidationFixture) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	f.cancel = cancel
	coordinator := NewControlIdentityInvalidationCoordinator(f, f, f, nil)
	coordinator.pollInterval = time.Nanosecond
	coordinator.grace = 0
	stop, err := coordinator.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	<-ctx.Done()
	stop()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatal("身份失效消费未及时退出")
	}
}

func TestControlInvalidationReasonsAndCursorOrder(t *testing.T) {
	f := &invalidationFixture{events: []ControlIdentityInvalidation{
		{EventID: 1, Reason: "session_revoked", SessionID: "browser-a"},
		{EventID: 2, Reason: "entitlement_changed"},
		{EventID: 3, Reason: "profile_changed"},
		{EventID: 4, Reason: "principal_changed"},
	}}
	runInvalidationFixture(t, f)
	want := []string{"apply:1", "session:browser-a", "commit:1", "apply:2", "commit:2", "apply:3", "owner:owner-a", "commit:3", "apply:4", "owner:owner-a", "runtime:owner-a", "commit:4"}
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("calls=%v, want %v", f.calls, want)
	}
}

func TestControlInvalidationRetriesBeforeAdvancingCursor(t *testing.T) {
	for _, stage := range []string{"apply", "commit", "runtime"} {
		t.Run(stage, func(t *testing.T) {
			f := &invalidationFixture{events: []ControlIdentityInvalidation{{EventID: 1, Reason: "principal_changed"}, {EventID: 2, Reason: "entitlement_changed"}}}
			switch stage {
			case "apply":
				f.applyFailures = 1
			case "commit":
				f.commitFailures = 1
			case "runtime":
				f.runtimeFailures = 1
			}
			runInvalidationFixture(t, f)
			applied := []string{}
			for _, call := range f.calls {
				if call == "apply:1" || call == "apply:2" {
					applied = append(applied, call)
				}
			}
			if !reflect.DeepEqual(applied, []string{"apply:1", "apply:1", "apply:2"}) || f.cursor != 2 {
				t.Fatalf("calls=%v cursor=%d", f.calls, f.cursor)
			}
		})
	}
}

func TestControlInvalidationSkipsPoisonEvent(t *testing.T) {
	f := &invalidationFixture{
		events: []ControlIdentityInvalidation{
			{EventID: 1, Reason: "principal_changed"},
			{EventID: 2, Reason: "profile_changed"},
		},
		applyFailures: 3,
	}
	runInvalidationFixture(t, f)

	var applied []string
	for _, call := range f.calls {
		if call == "apply:1" || call == "apply:2" {
			applied = append(applied, call)
		}
	}
	wantApplied := []string{"apply:1", "apply:1", "apply:1", "apply:2"}
	if !reflect.DeepEqual(applied, wantApplied) {
		t.Fatalf("apply calls=%v, want %v", applied, wantApplied)
	}
	if f.cursor != 2 {
		t.Fatalf("cursor=%d, want 2", f.cursor)
	}
	if f.failClosedCalls != 1 {
		t.Fatalf("fail-closed calls=%d, want 1", f.failClosedCalls)
	}
}

func TestControlInvalidationFailClosedRetriesFailedCleanup(t *testing.T) {
	for _, stage := range []string{"projection", "runtime"} {
		t.Run(stage, func(t *testing.T) {
			f := &invalidationFixture{fetchFailures: 3, stopAfterFetchFailure: true}
			if stage == "projection" {
				f.allFailures = 1
			} else {
				f.runtimeFailures = 1
			}
			runInvalidationFixture(t, f)
			if f.failClosedCalls != 2 {
				t.Fatalf("fail-closed calls=%d, want 2", f.failClosedCalls)
			}
			want := []string{"connections:all", "runtime:owner-a", "connections:all", "runtime:owner-a"}
			if !reflect.DeepEqual(f.calls, want) {
				t.Fatalf("calls=%v, want %v", f.calls, want)
			}
		})
	}
}

func TestControlInvalidationStopWaitsForConsumer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &invalidationFixture{cancel: cancel}
	coordinator := NewControlIdentityInvalidationCoordinator(f, f, f, nil)
	stop, err := coordinator.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	stop()
	// 停止后可安全读取消费状态，再次停止也不会阻塞。
	stop()
	if f.cursor != 0 {
		t.Fatalf("cursor=%d", f.cursor)
	}
}
