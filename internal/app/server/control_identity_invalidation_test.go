package server

import (
	"context"
	"sync"
	"testing"
	"time"

	shared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/runtime"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
)

type fakeControlInvalidationSource struct {
	mu      sync.Mutex
	applied []int64
	done    chan struct{}
}

func (f *fakeControlInvalidationSource) ControlIdentityInvalidationCursor(context.Context) (int64, error) {
	return 0, nil
}

func (f *fakeControlInvalidationSource) CommitControlIdentityInvalidationCursor(
	_ context.Context,
	_ int64,
) error {
	return nil
}

func (f *fakeControlInvalidationSource) ControlIdentityInvalidations(
	_ context.Context,
	after int64,
) ([]authsvc.ControlIdentityInvalidation, error) {
	if after != 0 {
		return nil, nil
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
	first := len(f.applied) == 1
	f.mu.Unlock()
	if first {
		close(f.done)
	}
	return "owner-a", nil
}

func (f *fakeControlInvalidationSource) FailClosedControlIdentities(context.Context) ([]string, error) {
	return nil, nil
}

func TestControlIdentityInvalidationCoordinatorAppliesOrderedEvent(t *testing.T) {
	source := &fakeControlInvalidationSource{done: make(chan struct{})}
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
