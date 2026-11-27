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

type observedDoneContext struct {
	context.Context
	observed chan struct{}
	once     sync.Once
}

func newObservedDoneContext(parent context.Context) *observedDoneContext {
	return &observedDoneContext{Context: parent, observed: make(chan struct{})}
}

func (c *observedDoneContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.observed) })
	return c.Context.Done()
}

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

type fakeOwnerProcessReaper struct {
	mu      sync.Mutex
	owners  []string
	err     error
	started chan string
	release <-chan struct{}
}

func (r *fakeOwnerProcessReaper) ReapOwnerProcesses(ctx context.Context, ownerUserID string) error {
	r.mu.Lock()
	r.owners = append(r.owners, ownerUserID)
	r.mu.Unlock()
	if r.started != nil {
		r.started <- ownerUserID
	}
	if r.release != nil {
		select {
		case <-r.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return r.err
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

type ownershipFenceClient struct {
	fakeRuntimeClient
	mu                 sync.Mutex
	retired            bool
	retireCalls        int
	connectCalls       int
	connectStarted     chan struct{}
	connectRelease     <-chan struct{}
	disconnectCalls    int
	reconfigureCalls   int
	reconfigureStarted chan struct{}
	reconfigureRelease <-chan struct{}
	disconnectStarted  chan struct{}
	disconnectRelease  <-chan struct{}
}

func (c *ownershipFenceClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.retired {
		c.mu.Unlock()
		return agentclient.ErrAborted
	}
	c.connectCalls++
	started := c.connectStarted
	release := c.connectRelease
	c.mu.Unlock()
	notifyRuntimeFence(started)
	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.retired {
		return agentclient.ErrAborted
	}
	return nil
}

func (c *ownershipFenceClient) Retire() {
	c.mu.Lock()
	c.retired = true
	c.retireCalls++
	c.mu.Unlock()
}

func (c *ownershipFenceClient) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	c.disconnectCalls++
	started := c.disconnectStarted
	release := c.disconnectRelease
	c.mu.Unlock()
	notifyRuntimeFence(started)
	if release == nil {
		return nil
	}
	select {
	case <-release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *ownershipFenceClient) Reconfigure(ctx context.Context, _ agentclient.Options) error {
	c.mu.Lock()
	c.reconfigureCalls++
	started := c.reconfigureStarted
	release := c.reconfigureRelease
	c.mu.Unlock()
	notifyRuntimeFence(started)
	if release == nil {
		return nil
	}
	select {
	case <-release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func notifyRuntimeFence(signal chan struct{}) {
	if signal == nil {
		return
	}
	select {
	case signal <- struct{}{}:
	default:
	}
}

type runtimeClientSequenceFactory struct {
	mu      sync.Mutex
	clients []Client
	index   int
}

func (f *runtimeClientSequenceFactory) New(agentclient.Options) Client {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.index >= len(f.clients) {
		return nil
	}
	client := f.clients[f.index]
	f.index++
	return client
}

type runtimeFactoryFunc func(agentclient.Options) Client

func (f runtimeFactoryFunc) New(options agentclient.Options) Client {
	return f(options)
}

func TestManagerUpdateEnvironmentForAgentRejectsManagedIdentity(t *testing.T) {
	manager := NewManager()
	client := &fakeRuntimeClient{}
	manager.sessions["agent:agent-a:conversation:1"] = &sessionState{
		Client:      client,
		RuntimeKind: agentclient.RuntimeNXS,
	}

	err := manager.UpdateEnvironmentForAgent(context.Background(), "agent-a", map[string]string{
		"NEXUS_RUNTIME_USER_ID": "owner-b",
	})
	if err == nil {
		t.Fatal("运行期环境更新应拒绝宿主管理的 owner 身份")
	}
	if len(client.environmentUpdates) != 0 {
		t.Fatalf("非法运行期环境不应下发给 runtime: %+v", client.environmentUpdates)
	}
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

func TestManagerStartRoundCancellationWhileWaitingForIdleDrain(t *testing.T) {
	messages := make(chan sdkprotocol.ReceivedMessage, 1)
	client := &fakeRuntimeClient{messages: messages}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: client})
	sessionKey := "agent:nexus:ws:dm:idle-handoff-cancel"
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{}); err != nil {
		t.Fatalf("创建 runtime client 失败: %v", err)
	}

	handlerStarted := make(chan struct{})
	handlerCanceled := make(chan struct{})
	releaseHandler := make(chan struct{})
	manager.StartIdleMessageDrain(sessionKey, func(ctx context.Context, _ sdkprotocol.ReceivedMessage) bool {
		close(handlerStarted)
		<-ctx.Done()
		close(handlerCanceled)
		<-releaseHandler
		return true
	})
	messages <- sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeTaskNotification}
	<-handlerStarted

	startCtx, cancelStart := context.WithCancel(context.Background())
	roundCanceled := make(chan struct{}, 1)
	startResult := make(chan error, 1)
	go func() {
		startResult <- manager.StartRound(startCtx, sessionKey, "round-1", func() {
			roundCanceled <- struct{}{}
		})
	}()
	<-handlerCanceled
	cancelStart()
	if err := <-startResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("StartRound() error = %v, want context canceled", err)
	}
	select {
	case <-roundCanceled:
	default:
		t.Fatal("等待 idle drain 期间取消时未释放 round context")
	}
	if running := manager.GetRunningRoundIDs(sessionKey); len(running) != 0 {
		t.Fatalf("取消的 round 被错误登记: %v", running)
	}

	close(releaseHandler)
	if err := manager.CloseSession(context.Background(), sessionKey); err != nil {
		t.Fatalf("CloseSession() error = %v", err)
	}
}

func TestManagerExternalCloseInvalidatesRetryAfterStartupCleanup(t *testing.T) {
	disconnectStarted := make(chan struct{}, 1)
	disconnectRelease := make(chan struct{})
	stale := &ownershipFenceClient{
		disconnectStarted: disconnectStarted,
		disconnectRelease: disconnectRelease,
	}
	fresh := &ownershipFenceClient{}
	factory := &runtimeClientSequenceFactory{clients: []Client{stale, fresh}}
	manager := NewManagerWithFactory(factory)
	sessionKey := "agent:nexus:ws:dm:close-during-startup-cleanup"
	startup, err := manager.BeginClientStartup(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatalf("开始启动事务失败: %v", err)
	}
	if _, err = startup.GetOrCreateWithFactory(context.Background(), agentclient.Options{}, nil); err != nil {
		startup.Close()
		t.Fatalf("创建旧 client 失败: %v", err)
	}
	if err = startup.Connect(context.Background()); err != nil {
		startup.Close()
		t.Fatalf("连接旧 client 失败: %v", err)
	}

	cleanupResult := make(chan error, 1)
	go func() {
		_, closeErr := startup.CloseCurrent(context.Background())
		cleanupResult <- closeErr
	}()
	select {
	case <-disconnectStarted:
	case <-time.After(time.Second):
		t.Fatal("启动失败清理未进入 Disconnect()")
	}
	externalCloseResult := make(chan error, 1)
	go func() {
		externalCloseResult <- manager.CloseSession(context.Background(), sessionKey)
	}()
	waitRuntimeStartupGateCloseBlocks(t, manager, sessionKey, 1)
	close(disconnectRelease)
	if err = <-cleanupResult; err != nil {
		t.Fatalf("启动失败清理 error = %v", err)
	}
	if err = <-externalCloseResult; err != nil {
		t.Fatalf("并发 CloseSession() error = %v", err)
	}
	if _, err = startup.GetOrCreateWithFactory(context.Background(), agentclient.Options{}, nil); !errors.Is(err, agentclient.ErrAborted) {
		t.Fatalf("并发关闭后的 retry error = %v，期望 ErrAborted", err)
	}
	startup.Close()
	factory.mu.Lock()
	factoryCalls := factory.index
	factory.mu.Unlock()
	if factoryCalls != 1 {
		t.Fatalf("并发关闭后 retry 仍创建了新 client: factory calls = %d", factoryCalls)
	}
	if current := manager.SessionClient(sessionKey); current != nil {
		t.Fatalf("并发关闭后 session 仍持有 client: %#v", current)
	}
}

func waitRuntimeStartupGateCloseBlocks(t *testing.T, manager *Manager, sessionKey string, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		manager.mu.RLock()
		gate := manager.startupGates[sessionKey]
		blocked := gate != nil && gate.closeBlocks == want && gate.closeEpoch > 0
		manager.mu.RUnlock()
		if blocked {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("CloseSession() startup gate closeBlocks 未达到 %d", want)
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

func TestManagerOwnerReaperWaitsForInflightConnect(t *testing.T) {
	connectStarted := make(chan struct{}, 1)
	releaseConnect := make(chan struct{})
	client := &ownershipFenceClient{
		connectStarted: connectStarted,
		connectRelease: releaseConnect,
	}
	reaperStarted := make(chan string, 1)
	reaper := &fakeOwnerProcessReaper{started: reaperStarted}
	manager := NewManagerWithFactory(&runtimeClientSequenceFactory{clients: []Client{client}})
	manager.SetOwnerProcessReaper(reaper)
	startup, err := manager.BeginClientStartup(context.Background(), "session-a", "owner-a")
	if err != nil {
		t.Fatalf("BeginClientStartup() error = %v", err)
	}
	options := agentclient.Options{Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner-a"}}
	if _, err = startup.GetOrCreateWithFactory(context.Background(), options, nil); err != nil {
		startup.Close()
		t.Fatalf("GetOrCreateWithFactory() error = %v", err)
	}
	connectResult := make(chan error, 1)
	go func() {
		connectResult <- startup.Connect(context.Background())
	}()
	<-connectStarted

	closeResult := make(chan error, 1)
	go func() {
		_, closeErr := manager.CloseOwnerSessions(context.Background(), "owner-a")
		closeResult <- closeErr
	}()
	waitOwnerReapFence(t, manager, "owner-a")
	select {
	case owner := <-reaperStarted:
		t.Fatalf("in-flight Connect 退出前 reaper 提前执行: %s", owner)
	default:
	}

	close(releaseConnect)
	if err = <-connectResult; !errors.Is(err, agentclient.ErrAborted) {
		startup.Close()
		t.Fatalf("Connect() error = %v, want aborted", err)
	}
	startup.Close()
	if owner := <-reaperStarted; owner != "owner-a" {
		t.Fatalf("reaper owner = %q, want owner-a", owner)
	}
	if err = <-closeResult; err != nil {
		t.Fatalf("CloseOwnerSessions() error = %v", err)
	}
}

func waitOwnerReapFence(t *testing.T, manager *Manager, ownerUserID string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		manager.mu.RLock()
		active := manager.ownerReapActiveLocked(ownerUserID)
		manager.mu.RUnlock()
		if active {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("owner %s reaper fence 未建立", ownerUserID)
		}
		time.Sleep(time.Millisecond)
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

func TestAgentClientConnectFailureRetriesWithLatestConfiguration(t *testing.T) {
	staleStartErr := errors.New("old runtime configuration rejected")
	latestStartErr := errors.New("new runtime executable unavailable")
	firstAttemptStarted := make(chan struct{})
	firstAttemptRelease := make(chan struct{})
	attempts := make(chan agentclient.Options, 2)
	client := &agentClient{
		options: agentclient.Options{Model: "old-model"},
		newSession: func(_ context.Context, options agentclient.Options) (*agentclient.Session, error) {
			attempts <- options
			if options.Model == "old-model" {
				close(firstAttemptStarted)
				<-firstAttemptRelease
				return nil, staleStartErr
			}
			return nil, latestStartErr
		},
	}
	ownerDone := make(chan error, 1)
	go func() { ownerDone <- client.Connect(context.Background()) }()
	select {
	case <-firstAttemptStarted:
	case <-time.After(time.Second):
		t.Fatal("旧配置 Connect 未启动")
	}
	waiterCtx := newObservedDoneContext(context.Background())
	waiterDone := make(chan error, 1)
	go func() { waiterDone <- client.Connect(waiterCtx) }()
	select {
	case <-waiterCtx.observed:
	case <-time.After(time.Second):
		t.Fatal("waiter 未加入旧配置 Connect flight")
	}
	if err := client.Reconfigure(context.Background(), agentclient.Options{Model: "new-model"}); err != nil {
		t.Fatalf("连接期间 Reconfigure 失败: %v", err)
	}
	close(firstAttemptRelease)
	select {
	case options := <-attempts:
		if options.Model != "old-model" {
			t.Fatalf("首次启动配置=%q", options.Model)
		}
	default:
		t.Fatal("缺少旧配置启动记录")
	}
	select {
	case options := <-attempts:
		if options.Model != "new-model" {
			t.Fatalf("失败后重试配置=%q", options.Model)
		}
	case <-time.After(time.Second):
		t.Fatal("旧配置失败后未使用新配置重试")
	}
	for name, done := range map[string]<-chan error{
		"owner":  ownerDone,
		"waiter": waiterDone,
	} {
		select {
		case err := <-done:
			if !errors.Is(err, latestStartErr) {
				t.Fatalf("%s 应共享新配置启动错误而不是旧错误: %v", name, err)
			}
		case <-time.After(time.Second):
			t.Fatalf("%s Connect 未结束", name)
		}
	}
}

func TestAgentClientReconfigureRollsBackRejectedDesiredState(t *testing.T) {
	reconfigureErr := errors.New("invalid model")
	client := &agentClient{
		options: agentclient.Options{Model: "old-model"},
		session: &agentclient.Session{},
		reconfigureSession: func(context.Context, *agentclient.Session, agentclient.Options) error {
			return reconfigureErr
		},
	}
	if err := client.Reconfigure(context.Background(), agentclient.Options{Model: "bad-model"}); !errors.Is(err, reconfigureErr) {
		t.Fatalf("Reconfigure 错误=%v", err)
	}
	client.mu.Lock()
	options := client.options
	configVersion := client.configVersion
	client.mu.Unlock()
	if options.Model != "old-model" {
		t.Fatalf("失败配置未回滚: %q", options.Model)
	}
	if configVersion != 2 {
		t.Fatalf("提交与回滚应各推进一次版本，实际=%d", configVersion)
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

func TestManagerSessionDeletionFenceBlocksAdmissionAndCanAbort(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	sessionKey := "agent:agent-a:ws:dm:delete-fence"
	options := agentclient.Options{
		Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner-a"},
	}
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, options); err != nil {
		t.Fatal(err)
	}
	lease, err := manager.BeginSessionDeletion(sessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = manager.GetOrCreate(
		context.Background(),
		sessionKey,
		options,
	); !errors.Is(err, ErrRuntimeSessionDeleted) {
		t.Fatalf("new runtime admission was not blocked: %v", err)
	}
	canceled := false
	if err := manager.StartRound(t.Context(), sessionKey, "late-round", func() { canceled = true }); !errors.Is(err, ErrRuntimeSessionDeleted) {
		t.Fatalf("late round error = %v, want session deleted", err)
	}
	if !canceled {
		t.Fatal("rejected round must cancel its context")
	}
	manager.AbortSessionDeletion(lease)
	if _, err = manager.GetOrCreate(context.Background(), sessionKey, options); err != nil {
		t.Fatalf("aborted deletion fence still blocks runtime: %v", err)
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

func TestManagerShutdownSetsRecoveryReasonBeforeCancellation(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	reason := make(chan string, 1)
	if err := manager.StartRound(t.Context(), "session", "round", func() {
		reason <- manager.GetInterruptReason("session", "round")
		manager.MarkRoundFinished("session", "round")
	}); err != nil {
		t.Fatal(err)
	}
	if err := manager.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := <-reason; got != InterruptHostShutdown {
		t.Fatalf("cancel reason=%q", got)
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

func TestManagedAutoDreamCancellationForcesUnresponsiveControl(t *testing.T) {
	for _, cause := range []string{"caller", "shutdown", "agent_revocation"} {
		t.Run(cause, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			started := make(chan struct{})
			c := &dreamTestClient{closed: make(chan struct{})}
			c.run = func(context.Context) (agentclient.AutoDreamResult, error) {
				close(started)
				<-c.closed // Deliberately ignore RPC context; only transport closure releases it.
				return agentclient.AutoDreamResult{}, context.Canceled
			}
			m := NewManagerWithFactory(runtimeFactoryFunc(func(agentclient.Options) Client { return c }))
			result := make(chan error, 1)
			go func() {
				_, err := m.TryAutoDream(ctx, "memory-maintenance:agent", dreamOptions())
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
