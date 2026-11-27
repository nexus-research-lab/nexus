// manager_test.go 覆盖 runtime Manager 生命周期、后台任务、清理、删除栅栏、停机与 AutoDream 测试。
package runtime

import (
	"context"
	"errors"
	"io"
	"maps"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type fakeRuntimeClient struct {
	reconfigureCalls   int
	lastOptions        agentclient.Options
	sentContents       []string
	reconfigureErr     error
	disconnectCalls    int
	disconnectErr      error
	disconnectFn       func(context.Context) error
	interruptCalls     int
	interruptHook      func()
	stoppedTasks       []string
	taskMessages       []fakeTaskMessage
	stopTaskErr        error
	permissionModes    []sdkpermission.Mode
	permissionModeErr  error
	environmentUpdates []map[string]string
	environmentErr     error
	hookResponseAck    bool
	messages           <-chan sdkprotocol.ReceivedMessage
	receiveStarted     chan struct{}
	receiveStopped     chan struct{}
}

type fakeTaskMessage struct {
	TaskID  string
	Message string
	Summary string
}

func (c *fakeRuntimeClient) Connect(context.Context) error { return nil }

func (c *fakeRuntimeClient) Query(context.Context, string) error { return nil }

func (c *fakeRuntimeClient) ReceiveMessages(ctx context.Context) <-chan sdkprotocol.ReceivedMessage {
	if c.receiveStarted != nil {
		select {
		case c.receiveStarted <- struct{}{}:
		default:
		}
	}
	if c.messages == nil {
		closed := make(chan sdkprotocol.ReceivedMessage)
		close(closed)
		return closed
	}
	out := make(chan sdkprotocol.ReceivedMessage)
	go func() {
		defer close(out)
		defer func() {
			if c.receiveStopped != nil {
				select {
				case c.receiveStopped <- struct{}{}:
				default:
				}
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case message, ok := <-c.messages:
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case out <- message:
				}
			}
		}
	}()
	return out
}

func (c *fakeRuntimeClient) SendContent(_ context.Context, content any, _ *string, _ string) error {
	if text, ok := content.(string); ok {
		c.sentContents = append(c.sentContents, text)
	}
	return nil
}

func (c *fakeRuntimeClient) Interrupt(context.Context) error {
	c.interruptCalls++
	if c.interruptHook != nil {
		c.interruptHook()
	}
	return nil
}

func (c *fakeRuntimeClient) StopTask(_ context.Context, taskID string) error {
	c.stoppedTasks = append(c.stoppedTasks, taskID)
	return c.stopTaskErr
}

func (c *fakeRuntimeClient) SendTaskMessage(_ context.Context, taskID string, message string, summary string) error {
	c.taskMessages = append(c.taskMessages, fakeTaskMessage{TaskID: taskID, Message: message, Summary: summary})
	return nil
}

func (c *fakeRuntimeClient) RemoveMessages(context.Context, []string) error { return nil }

func (c *fakeRuntimeClient) SetPermissionMode(_ context.Context, mode sdkpermission.Mode) error {
	c.permissionModes = append(c.permissionModes, mode)
	return c.permissionModeErr
}

func (c *fakeRuntimeClient) Retire() {}

func (c *fakeRuntimeClient) Disconnect(ctx context.Context) error {
	c.disconnectCalls++
	if c.disconnectFn != nil {
		return c.disconnectFn(ctx)
	}
	return c.disconnectErr
}

func (c *fakeRuntimeClient) Reconfigure(_ context.Context, options agentclient.Options) error {
	c.reconfigureCalls++
	c.lastOptions = options
	if c.reconfigureErr != nil {
		return c.reconfigureErr
	}
	return nil
}

func (c *fakeRuntimeClient) UpdateEnvironment(_ context.Context, environment map[string]string) error {
	c.environmentUpdates = append(c.environmentUpdates, maps.Clone(environment))
	return c.environmentErr
}

func (c *fakeRuntimeClient) Supports(capability agentclient.Capability) bool {
	return c.hookResponseAck && capability == agentclient.CapabilityHookResponseAck
}

func (c *fakeRuntimeClient) SessionID() string { return "" }

func TestAgentClientRetireCancelsConfigurationFlights(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*agentClient, func(context.Context) error)
		call      func(*agentClient) error
	}{
		{
			name: "reconfigure",
			configure: func(client *agentClient, block func(context.Context) error) {
				client.reconfigureSession = func(ctx context.Context, _ *agentclient.Session, _ agentclient.Options) error {
					return block(ctx)
				}
			},
			call: func(client *agentClient) error {
				return client.Reconfigure(context.Background(), agentclient.Options{})
			},
		},
		{
			name: "permission mode",
			configure: func(client *agentClient, block func(context.Context) error) {
				client.setSessionPermissionMode = func(ctx context.Context, _ *agentclient.Session, _ sdkpermission.Mode) error {
					return block(ctx)
				}
			},
			call: func(client *agentClient) error {
				return client.SetPermissionMode(context.Background(), sdkpermission.ModeDefault)
			},
		},
		{
			name: "environment",
			configure: func(client *agentClient, block func(context.Context) error) {
				client.updateSessionEnvironment = func(ctx context.Context, _ *agentclient.Session, _ map[string]string) error {
					return block(ctx)
				}
			},
			call: func(client *agentClient) error {
				return client.UpdateEnvironment(context.Background(), map[string]string{"KEY": "value"})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			started := make(chan struct{})
			client := &agentClient{
				session:      &agentclient.Session{},
				closeSession: func(*agentclient.Session) error { return nil },
			}
			test.configure(client, func(ctx context.Context) error {
				close(started)
				<-ctx.Done()
				return ctx.Err()
			})

			results := make(chan error, 2)
			go func() { results <- test.call(client) }()
			<-started
			go func() { results <- test.call(client) }()
			client.Retire()

			for range 2 {
				select {
				case err := <-results:
					if !errors.Is(err, agentclient.ErrAborted) {
						t.Fatalf("配置调用 error = %v, want ErrAborted", err)
					}
				case <-time.After(time.Second):
					t.Fatal("Retire 未唤醒配置调用")
				}
			}
		})
	}
}

type fakeRuntimeFactory struct {
	client  *fakeRuntimeClient
	clients []*fakeRuntimeClient
	index   int
}

func (f *fakeRuntimeFactory) New(agentclient.Options) Client {
	if len(f.clients) > 0 {
		client := f.clients[f.index]
		f.index++
		return client
	}
	return f.client
}

type runtimeFactoryFunc func(agentclient.Options) Client

func (f runtimeFactoryFunc) New(options agentclient.Options) Client {
	return f(options)
}

func TestManagerUpdateEnvironmentAttemptsEveryMatchingRuntime(t *testing.T) {
	manager := NewManager()
	failed := &fakeRuntimeClient{environmentErr: errors.New("environment update failed")}
	succeeded := &fakeRuntimeClient{}
	manager.sessions["agent:agent-a:ws:dm:failed"] = &sessionState{
		Client: failed, RuntimeKind: agentclient.RuntimeNXS,
	}
	manager.sessions["agent:agent-a:ws:group:succeeded"] = &sessionState{
		Client: succeeded, RuntimeKind: agentclient.RuntimeNXS,
	}

	err := manager.UpdateEnvironmentForAgent(
		context.Background(),
		"agent-a",
		map[string]string{
			"NEXUS_DISABLE_AUTO_MEMORY_EXTRACTION": "1",
			"NEXUS_ENABLE_AUTO_MEMORY_EXTRACTION":  "0",
			"NEXUS_DISABLE_AUTO_DREAM":             "1",
			"NEXUS_ENABLE_AUTO_DREAM":              "0",
			"NEXUS_WEBSEARCH_CONFIG":               `{"enabled":false}`,
		},
	)
	if err == nil {
		t.Fatal("UpdateEnvironmentForAgent should return the failed runtime error")
	}
	if len(failed.environmentUpdates) != 1 || len(succeeded.environmentUpdates) != 1 {
		t.Fatalf(
			"all matching runtimes must be attempted: failed=%d succeeded=%d",
			len(failed.environmentUpdates),
			len(succeeded.environmentUpdates),
		)
	}
}

func TestProcessPolicyFingerprintAllowsProviderHotUpdateButRejectsIsolationChange(t *testing.T) {
	base := agentclient.Options{
		CWD: "/srv/nexus/users/owner/workspace",
		Env: map[string]string{
			"NEXUS_RUNTIME_USER_ID":        "owner",
			"NEXUS_RUNTIME_ISOLATION_MODE": "enforce",
			"OPENAI_API_KEY":               "old-secret",
		},
	}
	providerUpdate := base
	providerUpdate.Env = maps.Clone(base.Env)
	providerUpdate.Env["OPENAI_API_KEY"] = "new-secret"
	if managedRuntimeProcessPolicyFingerprint(base) !=
		managedRuntimeProcessPolicyFingerprint(providerUpdate) {
		t.Fatal("provider credential hot update unexpectedly changed process-policy fingerprint")
	}
	isolationUpdate := base
	isolationUpdate.Env = maps.Clone(base.Env)
	isolationUpdate.Env["NEXUS_RUNTIME_ISOLATION_MODE"] = "audit"
	if managedRuntimeProcessPolicyFingerprint(base) ==
		managedRuntimeProcessPolicyFingerprint(isolationUpdate) {
		t.Fatal("isolation change did not change process-policy fingerprint")
	}
}

func TestManagerSendContentToRunningRound(t *testing.T) {
	client := &fakeRuntimeClient{}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: client})
	sessionKey := "agent:nexus:ws:dm:test-queue"

	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{}); err != nil {
		t.Fatalf("创建 client 失败: %v", err)
	}
	_ = manager.StartRound(context.Background(), sessionKey, "round-queue", func() {})

	roundIDs, err := manager.SendContentToRunningRound(context.Background(), sessionKey, "补充信息")
	if err != nil {
		t.Fatalf("排队 streaming input 失败: %v", err)
	}
	if len(roundIDs) != 1 || roundIDs[0] != "round-queue" {
		t.Fatalf("返回运行中 round 不正确: %+v", roundIDs)
	}
	if len(client.sentContents) != 1 || client.sentContents[0] != "补充信息" {
		t.Fatalf("client 未收到排队输入: %+v", client.sentContents)
	}
}

func TestManagerAdoptGoalObjectiveRevision(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	sessionKey := "agent:nexus:ws:dm:test-goal-revision"
	var revisionA atomic.Int64
	var revisionB atomic.Int64
	revisionA.Store(2)
	manager.RegisterGoalObjectiveRevision(sessionKey, "round-b", &revisionB)
	manager.RegisterGoalObjectiveRevision(sessionKey, "round-a", &revisionA)

	roundIDs := manager.AdoptGoalObjectiveRevision(sessionKey, 5)
	if strings.Join(roundIDs, ",") != "round-a" {
		t.Fatalf("roundIDs = %#v, want only authorized round-a", roundIDs)
	}
	if revisionA.Load() != 5 || revisionB.Load() != 0 {
		t.Fatalf("revisions = (%d, %d), want authorized 5 and unbound 0", revisionA.Load(), revisionB.Load())
	}

	revisionB.Store(1)
	roundIDs = manager.AdoptGoalObjectiveRevision(sessionKey, 6)
	if strings.Join(roundIDs, ",") != "round-a,round-b" || revisionA.Load() != 6 || revisionB.Load() != 6 {
		t.Fatalf("authorized adoption roundIDs=%#v revisions=(%d, %d), want both 6", roundIDs, revisionA.Load(), revisionB.Load())
	}

	manager.RegisterGoalObjectiveRevision(sessionKey, "round-a", nil)
	manager.MarkRoundFinished(sessionKey, "round-b")
	if roundIDs = manager.AdoptGoalObjectiveRevision(sessionKey, 7); len(roundIDs) != 0 {
		t.Fatalf("after unregister/finish roundIDs=%#v, want empty", roundIDs)
	}
}

func TestManagerActivationReportsAndRollsBackOnlySuccessfulRounds(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	sessionKey := "agent:nexus:ws:room:test-goal-activation-rollback"
	activationErr := errors.New("scope already consumed")
	cleared := []string{}
	manager.RegisterGoalAccountingActivate(sessionKey, "round-a", func(context.Context, string) error {
		return activationErr
	})
	manager.RegisterGoalAccountingActivate(sessionKey, "round-b", func(context.Context, string) error {
		return nil
	})
	manager.RegisterGoalAccountingClear(sessionKey, "round-a", func() {
		cleared = append(cleared, "round-a")
	})
	manager.RegisterGoalAccountingClear(sessionKey, "round-b", func() {
		cleared = append(cleared, "round-b")
	})

	activated, err := manager.ActivateGoalAccounting(context.Background(), sessionKey, "goal-new")
	if !errors.Is(err, activationErr) {
		t.Fatalf("ActivateGoalAccounting() error = %v, want activation error", err)
	}
	if strings.Join(activated, ",") != "round-b" {
		t.Fatalf("activated = %#v, want only successful round-b", activated)
	}
	if rolledBack := manager.ClearGoalAccountingRounds(sessionKey, activated); strings.Join(rolledBack, ",") != "round-b" {
		t.Fatalf("rolled back = %#v, want only round-b", rolledBack)
	}
	if strings.Join(cleared, ",") != "round-b" {
		t.Fatalf("clear callbacks = %#v, failing round-a must retain its prior binding", cleared)
	}
}

func TestAgentClientDisconnectInvalidatesInFlightConnect(t *testing.T) {
	connectStarted := make(chan struct{})
	connectRelease := make(chan struct{})
	staleSessionClosed := make(chan struct{}, 1)
	client := &agentClient{
		newSession: func(context.Context, agentclient.Options) (*agentclient.Session, error) {
			close(connectStarted)
			<-connectRelease
			return &agentclient.Session{}, nil
		},
		closeSession: func(*agentclient.Session) error {
			staleSessionClosed <- struct{}{}
			return nil
		},
	}
	connectDone := make(chan error, 1)
	go func() { connectDone <- client.Connect(context.Background()) }()
	select {
	case <-connectStarted:
	case <-time.After(time.Second):
		t.Fatal("Connect 未进入 session 启动阶段")
	}

	disconnectCtx, cancelDisconnect := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancelDisconnect()
	if err := client.Disconnect(disconnectCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Disconnect 应等待并受 context 约束: %v", err)
	}
	close(connectRelease)
	select {
	case err := <-connectDone:
		if !errors.Is(err, agentclient.ErrAborted) {
			t.Fatalf("被 Disconnect 失效的 Connect 错误=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("失效的 Connect 未退出")
	}
	select {
	case <-staleSessionClosed:
	case <-time.After(time.Second):
		t.Fatal("失效 Connect 创建的 session 未关闭")
	}
	if client.IsConnected() {
		t.Fatal("Disconnect 后失效 Connect 不应重新安装 session")
	}
}

// TestAgentClientCleanupFailureBlocksReconnect 不将完成清理尝试误认为清理成功。
func TestAgentClientCleanupFailureBlocksReconnect(t *testing.T) {
	want := errors.New("descendants remain")
	cleanup := &agentClientSessionCleanup{done: closedSignal(), err: want}
	var opened atomic.Int32
	client := &agentClient{cleanup: cleanup, newSession: func(context.Context, agentclient.Options) (*agentclient.Session, error) {
		opened.Add(1)
		return nil, errors.New("unexpected startup")
	}}
	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := client.Connect(ctx)
		cancel()
		if !errors.Is(err, want) {
			t.Errorf("Connect %d lost cleanup failure: %v", i, err)
		}
	}
	if opened.Load() != 0 || client.cleanup != cleanup {
		t.Fatal("failed cleanup allowed new startup or discarded evidence")
	}
}

// TestCleanupFailureIsNotAnOrdinaryClosedTransport 防止批量回收把 joined 断管错误整体忽略。
func TestCleanupFailureIsNotAnOrdinaryClosedTransport(t *testing.T) {
	want := &agentclient.ProcessCleanupError{SessionID: 9000, Err: io.ErrClosedPipe}
	for _, err := range []error{want, errors.Join(agentclient.ErrNotConnected, want)} {
		if IsRuntimeTransportClosedError(err) {
			t.Fatalf("cleanup failure classified as ordinary transport closure: %v", err)
		}
	}
	if !IsRuntimeTransportClosedError(io.ErrClosedPipe) {
		t.Fatal("ordinary closed-pipe classification changed")
	}
}

// TestManagerBulkCleanupReportsAndRetainsFailure 批量关闭仍报告清理错误并保留原 Session 栅栏。
func TestManagerBulkCleanupReportsAndRetainsFailure(t *testing.T) {
	for _, entry := range []string{"owner", "idle", "agent_revocation"} {
		t.Run(entry, func(t *testing.T) {
			want := &agentclient.ProcessCleanupError{SessionID: 9000, Err: io.ErrClosedPipe}
			client := &fakeRuntimeClient{disconnectFn: func(context.Context) error {
				return errors.Join(agentclient.ErrNotConnected, want)
			}}
			manager := NewManager()
			state := &sessionState{Client: client, OwnerUserID: "cleanup-owner", AgentID: "cleanup-agent", LastUsedAt: time.Now().Add(-time.Hour)}
			manager.sessions["cleanup-failed"] = state
			closeAll := func() (int, error) {
				switch entry {
				case "owner":
					return manager.CloseOwnerSessions(t.Context(), state.OwnerUserID)
				case "idle":
					return manager.CloseIdleSessions(t.Context(), time.Minute)
				default:
					return manager.RevokeAgentSessions(t.Context(), state.OwnerUserID, state.AgentID)
				}
			}
			if _, err := closeAll(); !errors.Is(err, want) {
				t.Fatalf("bulk close lost cleanup error: %v", err)
			}
			if err := manager.CloseSession(t.Context(), "cleanup-failed"); !errors.Is(err, want) {
				t.Fatalf("exact session lost cleanup error: %v", err)
			}
			if entry != "idle" {
				if _, err := closeAll(); !errors.Is(err, want) {
					t.Fatalf("repeated bulk close lost cleanup error: %v", err)
				}
			}
			manager.mu.RLock()
			defer manager.mu.RUnlock()
			if manager.sessions["cleanup-failed"] != state || !state.Closing {
				t.Fatal("bulk cleanup failure released session fence")
			}
		})
	}
}

// TestAgentClientStaleStartupCleanupFailureStopsRetry 启动中配置换代后，旧产物清理失败不得重试。
func TestAgentClientStaleStartupCleanupFailureStopsRetry(t *testing.T) {
	want := errors.New("stale runtime cleanup failed")
	started, release := make(chan struct{}), make(chan struct{})
	var opened atomic.Int32
	client := &agentClient{options: agentclient.Options{Model: "old"}, newSession: func(context.Context, agentclient.Options) (*agentclient.Session, error) {
		if opened.Add(1) == 1 {
			close(started)
			<-release
			return &agentclient.Session{}, nil
		}
		return nil, errors.New("unexpected retry")
	}, closeSession: func(*agentclient.Session) error { return want }}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- client.Connect(ctx) }()
	<-started
	if err := client.Reconfigure(ctx, agentclient.Options{Model: "new"}); err != nil {
		t.Fatal(err)
	}
	close(release)
	select {
	case err := <-done:
		if !errors.Is(err, want) {
			t.Errorf("stale cleanup error lost: %v", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if opened.Load() != 1 || client.cleanup == nil {
		t.Fatal("stale process cleanup failure permitted retry")
	}
}

// TestManagerCleanupFailureRetainsSessionFence 同步失败和超时后失败都保留原状态与精确关闭结果。
func TestManagerCleanupFailureRetainsSessionFence(t *testing.T) {
	for _, delayed := range []bool{false, true} {
		t.Run(map[bool]string{false: "synchronous", true: "after_timeout"}[delayed], func(t *testing.T) {
			want := errors.New("runtime descendants unconfirmed")
			var calls atomic.Int32
			client := &fakeRuntimeClient{disconnectFn: func(context.Context) error {
				if delayed && calls.Add(1) == 1 {
					return context.DeadlineExceeded
				}
				return want
			}}
			manager := NewManager()
			state := &sessionState{Client: client}
			manager.sessions["cleanup-failed"] = state
			if err := manager.CloseSession(context.Background(), "cleanup-failed"); err == nil {
				t.Fatal("close failure not reported")
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := manager.CloseSession(ctx, "cleanup-failed"); !errors.Is(err, want) {
				t.Fatalf("repeat close lost failure: %v", err)
			}
			if _, err := manager.GetOrCreate(ctx, "cleanup-failed", agentclient.Options{}); !errors.Is(err, ErrRuntimeSessionClosing) {
				t.Fatalf("failed cleanup allowed replacement: %v", err)
			}
			manager.mu.RLock()
			defer manager.mu.RUnlock()
			if manager.sessions["cleanup-failed"] != state || !errors.Is(state.CloseError, want) {
				t.Fatal("cleanup failure evidence removed")
			}
		})
	}
}

func TestManagerShutdownRejectsNewWorkAndPreservesFailure(t *testing.T) {
	want := errors.New("runtime descendant cleanup failed")
	client := &fakeRuntimeClient{disconnectErr: want}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: client})
	if _, err := manager.GetOrCreate(t.Context(), "session", agentclient.Options{}); err != nil {
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
	manager := NewManagerWithFactory(runtimeFactoryFunc(func(agentclient.Options) Client {
		close(entered)
		<-release
		return client
	}))
	started := make(chan error, 1)
	go func() { _, err := manager.GetOrCreate(t.Context(), "session", agentclient.Options{}); started <- err }()
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

type dreamTestClient struct {
	fakeRuntimeClient
	once       sync.Once
	closed     chan struct{}
	closeErr   error
	connectErr error
	run        func(context.Context) (agentclient.AutoDreamResult, error)
}

func (c *dreamTestClient) Connect(context.Context) error { return c.connectErr }
func (c *dreamTestClient) TryAutoDream(ctx context.Context) (agentclient.AutoDreamResult, error) {
	return c.run(ctx)
}
func (c *dreamTestClient) Disconnect(context.Context) error {
	c.once.Do(func() { close(c.closed) })
	return c.closeErr
}

func dreamOptions() agentclient.Options {
	return agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS}, Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner"}}
}

func TestManagedAutoDreamCleanupFailureRetainsFence(t *testing.T) {
	key := "memory-maintenance:agent"
	failure := errors.New("descendant retirement unknown")
	c := &dreamTestClient{closed: make(chan struct{}), closeErr: failure, run: func(context.Context) (agentclient.AutoDreamResult, error) { return agentclient.AutoDreamResult{}, nil }}
	m := NewManagerWithFactory(runtimeFactoryFunc(func(agentclient.Options) Client { return c }))
	if _, err := m.TryAutoDream(t.Context(), key, dreamOptions()); !errors.Is(err, failure) {
		t.Fatalf("err=%v", err)
	}
	if _, err := m.TryAutoDream(t.Context(), key, dreamOptions()); err == nil {
		t.Fatal("cleanup failure allowed another maintenance process")
	}
}
