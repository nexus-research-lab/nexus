package server

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	shared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/runtime"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
)

type fakeControlInvalidationSource struct {
	mu            sync.Mutex
	events        []authsvc.ControlIdentityInvalidation
	applied       []int64
	committed     []int64
	applyFailures map[int64]int
	failClosed    int
	done          chan struct{}
	signaled      bool
	signalAfter   int
}

func (f *fakeControlInvalidationSource) ControlIdentityInvalidationCursor(context.Context) (int64, error) {
	return 0, nil
}

func (f *fakeControlInvalidationSource) CommitControlIdentityInvalidationCursor(
	_ context.Context,
	cursor int64,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.committed = append(f.committed, cursor)
	return nil
}

func (f *fakeControlInvalidationSource) ControlIdentityInvalidations(
	_ context.Context,
	after int64,
) ([]authsvc.ControlIdentityInvalidation, error) {
	if after != 0 {
		return nil, nil
	}
	if len(f.events) > 0 {
		return f.events, nil
	}
	return []authsvc.ControlIdentityInvalidation{{
		EventID: 1, DeploymentID: "dep-a", UserID: "user-a", Reason: "principal_changed",
	}}, nil
}

func (f *fakeControlInvalidationSource) ApplyControlIdentityInvalidation(
	_ context.Context,
	event authsvc.ControlIdentityInvalidation,
) (string, error) {
	f.mu.Lock()
	f.applied = append(f.applied, event.EventID)
	remainingFailures := f.applyFailures[event.EventID]
	if remainingFailures > 0 {
		f.applyFailures[event.EventID] = remainingFailures - 1
	}
	shouldSignal := f.done != nil && !f.signaled && len(f.applied) == f.signalAfter
	if shouldSignal {
		f.signaled = true
		close(f.done)
	}
	f.mu.Unlock()
	if remainingFailures > 0 {
		return "owner-a", errors.New("persistent apply failure")
	}
	return "owner-a", nil
}

func (f *fakeControlInvalidationSource) FailClosedControlIdentities(context.Context) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failClosed++
	return nil, nil
}

func TestControlIdentityInvalidationCoordinatorAppliesOrderedEvent(t *testing.T) {
	source := &fakeControlInvalidationSource{done: make(chan struct{}), signalAfter: 1}
	server := &Server{
		api:      shared.NewAPI(logx.NewDiscardLogger()),
		services: &AppServices{Runtime: runtime.NewManager()},
	}
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() {
		server.runControlIdentityInvalidations(ctx, source, 0)
		close(finished)
	}()

	select {
	case <-source.done:
	case <-time.After(time.Second):
		t.Fatal("Control identity invalidation 未被应用")
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("Control identity invalidation coordinator 未停止")
	}

	source.mu.Lock()
	defer source.mu.Unlock()
	if len(source.applied) != 1 || source.applied[0] != 1 {
		t.Fatalf("applied events = %v, want [1]", source.applied)
	}
}

func TestControlIdentityInvalidationCoordinatorSkipsPoisonEvent(t *testing.T) {
	source := &fakeControlInvalidationSource{
		events: []authsvc.ControlIdentityInvalidation{
			{EventID: 1, DeploymentID: "dep-a", UserID: "user-a", Reason: "principal_changed"},
			{EventID: 2, DeploymentID: "dep-a", UserID: "user-a", Reason: "profile_changed"},
		},
		applyFailures: map[int64]int{1: controlInvalidationApplyAttempts},
		done:          make(chan struct{}),
		signalAfter:   4,
	}
	server := &Server{
		api:      shared.NewAPI(logx.NewDiscardLogger()),
		services: &AppServices{Runtime: runtime.NewManager()},
	}
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() {
		server.runControlIdentityInvalidations(ctx, source, 0)
		close(finished)
	}()

	select {
	case <-source.done:
	case <-time.After(5 * time.Second):
		t.Fatal("poison event blocked the following invalidation")
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("Control identity invalidation coordinator 未停止")
	}

	source.mu.Lock()
	defer source.mu.Unlock()
	wantApplied := []int64{1, 1, 1, 2}
	if len(source.applied) != len(wantApplied) {
		t.Fatalf("applied events = %v, want %v", source.applied, wantApplied)
	}
	for index, eventID := range wantApplied {
		if source.applied[index] != eventID {
			t.Fatalf("applied events = %v, want %v", source.applied, wantApplied)
		}
	}
	if len(source.committed) != 2 || source.committed[0] != 1 || source.committed[1] != 2 {
		t.Fatalf("committed cursors = %v, want [1 2]", source.committed)
	}
	if source.failClosed != 1 {
		t.Fatalf("fail-closed calls = %d, want 1", source.failClosed)
	}
}
