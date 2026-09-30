// INPUT: 可阻塞控制请求、独立 scratch 与 Manager 停机/撤销。
// OUTPUT: 一次性维护进入统一启动、取消、资源交接及失败栅栏。
// POS: 生命周期故障测试；不证明真实模型的记忆整理结果。
package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

type dreamTestClient struct {
	fakeRuntimeClient
	once       sync.Once
	closed     chan struct{}
	lease      *SandboxResourceLease
	closeErr   error
	connectErr error
	run        func(context.Context) (bridge.AutoDreamResult, error)
}

func (c *dreamTestClient) BindSandboxLease(lease *SandboxResourceLease) error {
	c.lease = lease
	return nil
}
func (c *dreamTestClient) Connect(context.Context) error { return c.connectErr }
func (c *dreamTestClient) TryAutoDream(ctx context.Context) (bridge.AutoDreamResult, error) {
	return c.run(ctx)
}
func (c *dreamTestClient) Disconnect(context.Context) error {
	c.once.Do(func() {
		if c.lease != nil {
			if c.closeErr != nil {
				_ = c.lease.MarkCleanupUncertain(c.closeErr)
			} else {
				c.closeErr = c.lease.Release()
			}
		}
		close(c.closed)
	})
	return c.closeErr
}

func dreamOptions() bridge.Options {
	return bridge.Options{Runtime: bridge.RuntimeOptions{Kind: bridge.RuntimeNXS}, Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner"}}
}

func TestManagedAutoDreamRetiresAndTransfersScratch(t *testing.T) {
	key := "memory-maintenance:agent"
	lease, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: key, Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	c := &dreamTestClient{closed: make(chan struct{}), run: func(context.Context) (bridge.AutoDreamResult, error) {
		return bridge.AutoDreamResult{Status: bridge.AutoDreamStatusSkipped, Reason: "not_due"}, nil
	}}
	m := NewManagerWithFactory(runtimeFactoryFunc(func(bridge.Options) Client { return c }))
	opts := dreamOptions()
	opts.Sandbox = &bridge.SandboxSettings{Resources: lease.Resources()}
	result, consumed, err := m.TryAutoDream(t.Context(), key, opts, lease)
	if err != nil || !consumed || result.Reason != "not_due" {
		t.Fatalf("result=%+v consumed=%v err=%v", result, consumed, err)
	}
	if lease.active() || m.SessionClient(key) != nil {
		t.Fatal("maintenance client or scratch retained after successful close")
	}
	if err := m.WaitBackgroundTasks(t.Context(), key); err != nil {
		t.Fatal(err)
	}
}

func TestManagedAutoDreamCancellationForcesUnresponsiveControl(t *testing.T) {
	for _, cause := range []string{"caller", "shutdown", "agent_revocation"} {
		t.Run(cause, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			started := make(chan struct{})
			c := &dreamTestClient{closed: make(chan struct{})}
			c.run = func(context.Context) (bridge.AutoDreamResult, error) {
				close(started)
				<-c.closed // Deliberately ignore RPC context; only transport closure releases it.
				return bridge.AutoDreamResult{}, context.Canceled
			}
			m := NewManagerWithFactory(runtimeFactoryFunc(func(bridge.Options) Client { return c }))
			result := make(chan error, 1)
			go func() {
				_, _, err := m.TryAutoDream(ctx, "memory-maintenance:agent", dreamOptions(), nil)
				result <- err
			}()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("control did not start")
			}
			closeCtx, closeCancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer closeCancel()
			switch cause {
			case "caller":
				cancel()
			case "shutdown":
				if err := m.Close(closeCtx); err != nil {
					t.Fatal(err)
				}
			case "agent_revocation":
				if _, err := m.RevokeAgentSessions(closeCtx, "owner", "agent"); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-result:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("result=%v", err)
				}
			case <-closeCtx.Done():
				t.Fatal("cancellation did not retire control")
			}
		})
	}
}

func TestManagedAutoDreamCleanupFailureRetainsFence(t *testing.T) {
	key := "memory-maintenance:agent"
	lease, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: key, Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	failure := errors.New("descendant retirement unknown")
	c := &dreamTestClient{closed: make(chan struct{}), closeErr: failure, run: func(context.Context) (bridge.AutoDreamResult, error) { return bridge.AutoDreamResult{}, nil }}
	m := NewManagerWithFactory(runtimeFactoryFunc(func(bridge.Options) Client { return c }))
	_, consumed, err := m.TryAutoDream(t.Context(), key, dreamOptions(), lease)
	if !consumed || !errors.Is(err, failure) || lease.Marker().CleanupState != cleanupStateUnknown {
		t.Fatalf("consumed=%v err=%v marker=%+v", consumed, err, lease.Marker())
	}
	if _, _, err := m.TryAutoDream(t.Context(), key, dreamOptions(), nil); err == nil {
		t.Fatal("cleanup failure allowed another maintenance process")
	}
}

func TestManagedAutoDreamConnectFailureClosesTransferredLease(t *testing.T) {
	lease, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "memory-maintenance:agent", Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	failure := errors.New("initialize failed")
	c := &dreamTestClient{closed: make(chan struct{}), connectErr: failure}
	m := NewManagerWithFactory(runtimeFactoryFunc(func(bridge.Options) Client { return c }))
	_, consumed, err := m.TryAutoDream(t.Context(), "memory-maintenance:agent", dreamOptions(), lease)
	if !consumed || !errors.Is(err, failure) || lease.active() {
		t.Fatalf("consumed=%v err=%v active=%v", consumed, err, lease.active())
	}
}
