package server

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

type fakeOwnerRuntimeCloser struct {
	mu      sync.Mutex
	owners  []string
	started chan struct{}
	err     error
}

type transitionRuntimeClient struct {
	runtimectx.Client
	mu          sync.Mutex
	disconnects int
}

func (c *transitionRuntimeClient) Disconnect(context.Context) error {
	c.mu.Lock()
	c.disconnects++
	c.mu.Unlock()
	return nil
}

func (c *transitionRuntimeClient) Retire() {}

type transitionRuntimeFactory struct {
	client *transitionRuntimeClient
}

func (f transitionRuntimeFactory) New(agentclient.Options) runtimectx.Client {
	return f.client
}

func (f *fakeOwnerRuntimeCloser) CloseOwnerSessions(
	_ context.Context,
	ownerUserID string,
) (int, error) {
	f.mu.Lock()
	f.owners = append(f.owners, ownerUserID)
	f.mu.Unlock()
	if f.started != nil {
		close(f.started)
	}
	return 1, f.err
}

func TestRuntimeAuthTransitionDrainsAdmissionBeforeSystemOwnerRevocation(t *testing.T) {
	closer := &fakeOwnerRuntimeCloser{started: make(chan struct{})}
	transition := newRuntimeAuthTransition(closer)
	active, err := transition.BeginRuntimeAdmission(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	committed := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- transition.EnableAuthentication(context.Background(), func(context.Context) error {
			close(committed)
			return nil
		})
	}()

	select {
	case <-active.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("认证转场未取消 pre-auth runtime admission")
	}
	waitingAdmission := make(chan error, 1)
	go func() {
		lease, admissionErr := transition.BeginRuntimeAdmission(context.Background())
		if lease != nil {
			lease.Release()
		}
		waitingAdmission <- admissionErr
	}()
	select {
	case err = <-waitingAdmission:
		t.Fatalf("认证转场期间错误放行新 runtime admission: %v", err)
	default:
	}
	select {
	case <-closer.started:
		t.Fatal("pre-auth admission 未排空前不应撤销 runtime")
	default:
	}
	active.Release()

	select {
	case <-closer.started:
	case <-time.After(time.Second):
		t.Fatal("排空 admission 后未撤销 system owner runtime")
	}
	select {
	case <-committed:
	case <-time.After(time.Second):
		t.Fatal("撤销 runtime 后未提交认证")
	}
	if err = <-done; err != nil {
		t.Fatalf("EnableAuthentication() error = %v", err)
	}
	select {
	case err = <-waitingAdmission:
		if err != nil {
			t.Fatalf("认证提交后 admission error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("认证提交后未重新开放 runtime admission")
	}

	closer.mu.Lock()
	defer closer.mu.Unlock()
	if len(closer.owners) != 1 || closer.owners[0] != "__system__" {
		t.Fatalf("撤销 owner = %v, want [__system__]", closer.owners)
	}
}

func TestRuntimeAuthTransitionRevocationFailurePreventsCommit(t *testing.T) {
	revokeErr := errors.New("revoke failed")
	transition := newRuntimeAuthTransition(&fakeOwnerRuntimeCloser{err: revokeErr})
	committed := false
	err := transition.EnableAuthentication(context.Background(), func(context.Context) error {
		committed = true
		return nil
	})
	if !errors.Is(err, revokeErr) {
		t.Fatalf("EnableAuthentication() error = %v, want revoke error", err)
	}
	if committed {
		t.Fatal("runtime 撤销失败后不应启用认证")
	}
}

func TestRuntimeAuthTransitionCancelsActualManagerRoundBeforeCommit(t *testing.T) {
	client := &transitionRuntimeClient{}
	manager := runtimectx.NewManagerWithFactory(transitionRuntimeFactory{client: client})
	const (
		sessionKey = "agent:nexus:ws:dm:pre-auth"
		roundID    = "round-pre-auth"
	)
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{
		Env: map[string]string{"NEXUS_RUNTIME_USER_ID": authctx.SystemUserID},
	}); err != nil {
		t.Fatal(err)
	}
	roundCanceled := make(chan struct{})
	if err := manager.StartRound(t.Context(), sessionKey, roundID, func() {
		close(roundCanceled)
		manager.MarkRoundFinished(sessionKey, roundID)
	}); err != nil {
		t.Fatalf("failed to start pre-auth runtime round: %v", err)
	}

	committed := false
	transition := newRuntimeAuthTransition(manager)
	if err := transition.EnableAuthentication(context.Background(), func(context.Context) error {
		select {
		case <-roundCanceled:
		default:
			t.Fatal("认证提交发生在 pre-auth round 取消之前")
		}
		committed = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !committed {
		t.Fatal("runtime 撤销成功后未提交认证")
	}
	if manager.HasSession(sessionKey) {
		t.Fatal("认证提交后仍保留 pre-auth runtime session")
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.disconnects != 1 {
		t.Fatalf("pre-auth runtime disconnects = %d, want 1", client.disconnects)
	}
}
