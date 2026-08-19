package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/runtimeadmission"
)

type fakeRuntimeTransitionCoordinator struct {
	mu          sync.Mutex
	enableCalls int
	enableErr   error
}

type gateRuntimeTransitionCoordinator struct {
	gate *runtimeadmission.Gate
}

func newGateRuntimeTransitionCoordinator() *gateRuntimeTransitionCoordinator {
	return &gateRuntimeTransitionCoordinator{gate: runtimeadmission.NewGate()}
}

func (c *gateRuntimeTransitionCoordinator) BeginRuntimeAdmission(
	ctx context.Context,
) (*runtimeadmission.Lease, error) {
	return c.gate.Admit(ctx)
}

func (c *gateRuntimeTransitionCoordinator) EnableAuthentication(
	ctx context.Context,
	commit func(context.Context) error,
) error {
	return c.gate.Transition(ctx, func(context.Context) error { return nil }, commit)
}

func (f *fakeRuntimeTransitionCoordinator) BeginRuntimeAdmission(
	ctx context.Context,
) (*runtimeadmission.Lease, error) {
	return runtimeadmission.NewDetachedLease(ctx), nil
}

func (f *fakeRuntimeTransitionCoordinator) EnableAuthentication(
	ctx context.Context,
	commit func(context.Context) error,
) error {
	f.mu.Lock()
	f.enableCalls++
	enableErr := f.enableErr
	f.mu.Unlock()
	if enableErr != nil {
		return enableErr
	}
	return commit(ctx)
}

func (f *fakeRuntimeTransitionCoordinator) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.enableCalls
}

func TestAgentRuntimeAdmissionAllowsInitializedAuth(t *testing.T) {
	cfg, db := newAuthTestDB(t)
	service := NewServiceWithDB(cfg, db)
	ctx := context.Background()

	lease, err := service.BeginAgentRuntimeAdmission(ctx)
	if err != nil {
		t.Fatalf("读取初始 runtime admission 失败: %v", err)
	}
	lease.Release()

	if _, err = service.InitOwner(ctx, InitOwnerInput{
		Username: "admin",
		Password: "password123",
	}); err != nil {
		t.Fatalf("初始化 owner 失败: %v", err)
	}
	lease, err = service.BeginAgentRuntimeAdmission(ctx)
	if err != nil {
		t.Fatalf("读取 owner 初始化后的 runtime admission 失败: %v", err)
	}
	lease.Release()
}

func TestInitOwnerRequiresRuntimeTransitionBeforeEnablingServerAuth(t *testing.T) {
	cfg, db := newAuthTestDB(t)
	service := NewServiceWithDB(cfg, db)
	revokeErr := errors.New("pre-auth runtime revoke failed")
	transition := &fakeRuntimeTransitionCoordinator{enableErr: revokeErr}
	service.SetRuntimeTransitionCoordinator(transition)

	if _, err := service.InitOwner(context.Background(), InitOwnerInput{
		Username: "admin",
		Password: "password123",
	}); !errors.Is(err, revokeErr) {
		t.Fatalf("InitOwner() error = %v, want revoke failure", err)
	}
	state, err := service.GetState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.UserCount != 0 || state.AuthRequired {
		t.Fatalf("runtime 撤销失败后不应启用认证: %+v", state)
	}
	if transition.callCount() != 1 {
		t.Fatalf("server runtime transition calls = %d, want 1", transition.callCount())
	}
}

func TestInitOwnerKeepsDesktopLocalModeOutsideRuntimeTransition(t *testing.T) {
	cfg, db := newAuthTestDB(t)
	cfg.AppMode = "desktop"
	service := NewServiceWithDB(cfg, db)
	transition := &fakeRuntimeTransitionCoordinator{
		enableErr: errors.New("desktop should not enter transition"),
	}
	service.SetRuntimeTransitionCoordinator(transition)

	if _, err := service.InitOwner(context.Background(), InitOwnerInput{
		Username: "admin",
		Password: "password123",
	}); err != nil {
		t.Fatalf("desktop InitOwner() error = %v", err)
	}
	if transition.callCount() != 0 {
		t.Fatalf("desktop runtime transition calls = %d, want 0", transition.callCount())
	}
	state, err := service.GetState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.AuthRequired || state.PasswordLoginEnabled {
		t.Fatalf("desktop 初始化后仍必须保持本地单用户投影: %+v", state)
	}
}

func TestConcurrentInitOwnerEntersRuntimeTransitionOnce(t *testing.T) {
	cfg, db := newAuthTestDB(t)
	service := NewServiceWithDB(cfg, db)
	transition := &fakeRuntimeTransitionCoordinator{}
	service.SetRuntimeTransitionCoordinator(transition)

	results := make(chan error, 2)
	for _, username := range []string{"first-owner", "second-owner"} {
		username := username
		go func() {
			_, err := service.InitOwner(context.Background(), InitOwnerInput{
				Username: username,
				Password: "password123",
			})
			results <- err
		}()
	}
	var successCount, alreadyInitializedCount int
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			successCount++
		case errors.Is(err, ErrOwnerAlreadyInitialized):
			alreadyInitializedCount++
		default:
			t.Fatalf("unexpected InitOwner() error = %v", err)
		}
	}
	if successCount != 1 || alreadyInitializedCount != 1 {
		t.Fatalf(
			"concurrent InitOwner results: success=%d already_initialized=%d",
			successCount,
			alreadyInitializedCount,
		)
	}
	if transition.callCount() != 1 {
		t.Fatalf("runtime transition calls = %d, want 1", transition.callCount())
	}
}

func TestInitOwnerTransitionMakesWaitingAdmissionObserveAuthenticatedState(t *testing.T) {
	cfg, db := newAuthTestDB(t)
	service := NewServiceWithDB(cfg, db)
	transition := newGateRuntimeTransitionCoordinator()
	service.SetRuntimeTransitionCoordinator(transition)

	preAuthLease, err := service.BeginAgentRuntimeAdmission(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	initDone := make(chan error, 1)
	go func() {
		_, initErr := service.InitOwner(context.Background(), InitOwnerInput{
			Username: "admin",
			Password: "password123",
		})
		initDone <- initErr
	}()
	select {
	case <-preAuthLease.Context().Done():
	case <-time.After(5 * time.Second):
		t.Fatal("owner 初始化未撤销 pre-auth admission")
	}

	type admissionResult struct {
		lease *runtimeadmission.Lease
		err   error
	}
	waiting := make(chan admissionResult, 1)
	go func() {
		lease, waitingErr := service.BeginAgentRuntimeAdmission(context.Background())
		waiting <- admissionResult{lease: lease, err: waitingErr}
	}()
	select {
	case result := <-waiting:
		if result.lease != nil {
			result.lease.Release()
		}
		t.Fatalf("认证提交前错误放行新 admission: %+v", result)
	default:
	}

	preAuthLease.Release()
	select {
	case err = <-initDone:
		if err != nil {
			t.Fatalf("InitOwner() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("owner 初始化未完成")
	}
	select {
	case result := <-waiting:
		if result.lease != nil {
			result.lease.Release()
		}
		if result.err != nil {
			t.Fatalf("认证提交后的 admission = %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("认证提交后未放行等待中的 admission")
	}
}
