// INPUT: 宿主退出、并发启动/回执落盘与后台任务。
// OUTPUT: 关闭准入、终态持久化顺序、可重复等待及失败保留的回归证据。
// POS: Manager 整体生命周期测试。
package runtime

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestManagerShutdownRejectsNewWorkAndPreservesFailure(t *testing.T) {
	want := errors.New("runtime descendant cleanup failed")
	client := &fakeRuntimeClient{disconnectErr: want}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: client})
	if _, err := manager.GetOrCreate(t.Context(), "session", bridge.Options{}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := manager.Close(t.Context()); !errors.Is(err, want) {
			t.Fatalf("Close=%v, want retained cleanup error", err)
		}
	}
	if client.disconnectCalls != 1 {
		t.Fatalf("shutdown repeated side effects: %d disconnects", client.disconnectCalls)
	}
	for _, owner := range []string{"", "owner"} {
		if _, err := manager.BeginClientStartup(t.Context(), "new", owner); !errors.Is(err, ErrRuntimeManagerClosed) {
			t.Fatalf("startup owner=%q err=%v", owner, err)
		}
	}
	if manager.StartBackgroundTask("new", func(context.Context) { t.Error("late background task ran") }) {
		t.Fatal("closed manager accepted background task")
	}
	if err := manager.StartRound(t.Context(), "new", "round", nil); !errors.Is(err, ErrRuntimeManagerClosed) {
		t.Fatalf("closed manager accepted round: %v", err)
	}
}

func TestManagerShutdownWaitsForLateFactory(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	client := &fakeRuntimeClient{}
	manager := NewManagerWithFactory(runtimeFactoryFunc(func(bridge.Options) Client {
		close(entered)
		<-release
		return client
	}))
	started := make(chan error, 1)
	go func() { _, err := manager.GetOrCreate(t.Context(), "session", bridge.Options{}); started <- err }()
	<-entered
	short, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if err := manager.Close(short); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close returned before factory disposed: %v", err)
	}
	close(release)
	if err := <-started; !errors.Is(err, ErrRuntimeManagerClosed) {
		t.Fatalf("late factory installed client: %v", err)
	}
	if err := manager.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if client.disconnectCalls != 1 || manager.SessionClient("session") != nil {
		t.Fatalf("late client survived: disconnects=%d", client.disconnectCalls)
	}
}

type shutdownReceiptStore struct {
	entered, release chan struct{}
	phase            protocol.SandboxPolicyReceiptPhase
	saved            atomic.Bool
}

func (s *shutdownReceiptStore) Latest(context.Context, string, string) (protocol.SandboxPolicyReceiptSnapshot, bool, error) {
	return protocol.SandboxPolicyReceiptSnapshot{}, false, nil
}
func (s *shutdownReceiptStore) Save(_ context.Context, snapshot protocol.SandboxPolicyReceiptSnapshot) error {
	close(s.entered)
	<-s.release
	s.phase = snapshot.Phase
	s.saved.Store(true)
	return nil
}
func (s *shutdownReceiptStore) UpdatePhase(_ context.Context, _, _ string, _ uint64, phase protocol.SandboxPolicyReceiptPhase, _ string) error {
	if !s.saved.Load() {
		return errors.New("terminal update raced initial receipt insertion")
	}
	s.phase = phase
	return nil
}

func TestManagerShutdownDrainsReceiptInsertionBeforeRetirement(t *testing.T) {
	store := &shutdownReceiptStore{entered: make(chan struct{}), release: make(chan struct{})}
	client := &receiptRuntimeClient{receipt: SandboxEffectivePolicyReceipt{
		Version: 1, SessionID: "runtime", RuntimeKind: bridge.RuntimeNXS,
		PolicyDigest: "sha256:shutdown", Phase: protocol.SandboxPolicyReceiptConfirmed,
		CapabilityEvidence: "bridge_negotiated", IsolationEvidence: "not_attested", ConfirmedAt: time.Now().UTC(),
	}}
	manager := NewManagerWithFactory(runtimeFactoryFunc(func(bridge.Options) Client { return client }))
	manager.SetSandboxPolicyReceiptStore(store)
	startup, err := manager.BeginClientStartup(t.Context(), "session", "owner")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := startup.GetOrCreateWithFactory(t.Context(), bridge.Options{Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner"}}, nil); err != nil {
		t.Fatal(err)
	}
	connected := make(chan error, 1)
	go func() {
		err := startup.Connect(t.Context())
		startup.Close()
		connected <- err
	}()
	<-store.entered
	short, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if err := manager.Close(short); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close did not wait for pending receipt: %v", err)
	}
	close(store.release)
	if err := <-connected; err != nil {
		t.Fatal(err)
	}
	if err := manager.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if store.phase != protocol.SandboxPolicyReceiptRetired || client.disconnectCalls != 1 {
		t.Fatalf("phase=%s disconnects=%d", store.phase, client.disconnectCalls)
	}
}
