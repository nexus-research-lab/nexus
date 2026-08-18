package runtime

import (
	"context"
	"errors"
	"io"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
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

func (r *fakeOwnerProcessReaper) ownerCalls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.owners)
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

func (c *fakeRuntimeClient) Disconnect(context.Context) error {
	c.disconnectCalls++
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

func TestAgentClientWaitReturnsStreamError(t *testing.T) {
	processErr := errors.New("process: command exited with error: exit status 2")
	client := &agentClient{streamErr: processErr}

	if err := client.Wait(); !errors.Is(err, processErr) {
		t.Fatalf("Wait() error = %v，期望返回 stream error", err)
	}
}

func TestAgentClientRetirePermanentlyPreventsReconnect(t *testing.T) {
	newSessionCalls := 0
	client := &agentClient{
		newSession: func(context.Context, agentclient.Options) (*agentclient.Session, error) {
			newSessionCalls++
			return nil, errors.New("retired client should not create a session")
		},
	}

	client.Retire()
	client.Retire()
	if err := client.Connect(context.Background()); !errors.Is(err, agentclient.ErrAborted) {
		t.Fatalf("Connect() error = %v，期望 ErrAborted", err)
	}
	if err := client.Reconfigure(context.Background(), agentclient.Options{}); !errors.Is(err, agentclient.ErrAborted) {
		t.Fatalf("Reconfigure() error = %v，期望 ErrAborted", err)
	}
	if err := client.Disconnect(context.Background()); err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
	if err := client.Connect(context.Background()); !errors.Is(err, agentclient.ErrAborted) {
		t.Fatalf("Disconnect() 后 Connect() error = %v，期望 ErrAborted", err)
	}
	if newSessionCalls != 0 {
		t.Fatalf("retired client 创建了 %d 次 session，期望 0", newSessionCalls)
	}
}

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

type fakeSDKMCPServer struct{}

func (fakeSDKMCPServer) HandleMessage(context.Context, map[string]any) (map[string]any, error) {
	return map[string]any{"ok": true}, nil
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

type runtimeClientResult struct {
	client Client
	err    error
}

type runtimeFactoryFunc func(agentclient.Options) Client

func (f runtimeFactoryFunc) New(options agentclient.Options) Client {
	return f(options)
}

func TestManagerSetPermissionModeForAgentUpdatesMatchingClients(t *testing.T) {
	manager := NewManager()
	matching := &fakeRuntimeClient{}
	other := &fakeRuntimeClient{}
	manager.sessions["agent:agent-a:conversation:1"] = &sessionState{Client: matching}
	manager.sessions["agent:agent-b:conversation:1"] = &sessionState{Client: other}

	if err := manager.SetPermissionModeForAgent(context.Background(), "agent-a", sdkpermission.ModePlan); err != nil {
		t.Fatalf("SetPermissionModeForAgent() error = %v", err)
	}
	if len(matching.permissionModes) != 1 || matching.permissionModes[0] != sdkpermission.ModePlan {
		t.Fatalf("matching permission modes = %#v，期望 [plan]", matching.permissionModes)
	}
	if len(other.permissionModes) != 0 {
		t.Fatalf("other permission modes = %#v，期望空", other.permissionModes)
	}
}

func TestManagerSetPermissionModeClosesFailedRuntimeAndContinues(t *testing.T) {
	manager := NewManager()
	failed := &fakeRuntimeClient{permissionModeErr: errors.New("unsupported live update")}
	succeeded := &fakeRuntimeClient{}
	failedSession := "agent:agent-a:conversation:failed"
	succeededSession := "agent:agent-a:conversation:succeeded"
	manager.sessions[failedSession] = &sessionState{Client: failed}
	manager.sessions[succeededSession] = &sessionState{Client: succeeded}

	err := manager.SetPermissionModeForAgent(context.Background(), "agent-a", sdkpermission.ModePlan)
	if err == nil || !strings.Contains(err.Error(), "已关闭旧 runtime") {
		t.Fatalf("SetPermissionModeForAgent() error = %v", err)
	}
	if failed.disconnectCalls != 1 || manager.sessions[failedSession] != nil {
		t.Fatalf("failed runtime was not removed safely: disconnect=%d state=%+v", failed.disconnectCalls, manager.sessions[failedSession])
	}
	if len(succeeded.permissionModes) != 1 || succeeded.permissionModes[0] != sdkpermission.ModePlan {
		t.Fatalf("later matching runtime was not updated: %#v", succeeded.permissionModes)
	}
	if manager.sessions[succeededSession] == nil {
		t.Fatal("successfully updated runtime should remain active")
	}
}

func TestManagerUpdateEnvironmentForAgentUpdatesMatchingNXSClients(t *testing.T) {
	manager := NewManager()
	matching := &fakeRuntimeClient{}
	otherRuntime := &fakeRuntimeClient{}
	otherAgent := &fakeRuntimeClient{}
	manager.sessions["agent:agent-a:conversation:1"] = &sessionState{
		Client:      matching,
		RuntimeKind: agentclient.RuntimeNXS,
	}
	manager.sessions["agent:agent-a:conversation:2"] = &sessionState{
		Client:      otherRuntime,
		RuntimeKind: agentclient.RuntimeClaude,
	}
	manager.sessions["agent:agent-b:conversation:1"] = &sessionState{
		Client:      otherAgent,
		RuntimeKind: agentclient.RuntimeNXS,
	}

	environment := map[string]string{"NEXUS_WEBSEARCH_CONFIG": `{"enabled":false}`}
	if err := manager.UpdateEnvironmentForAgent(context.Background(), "agent-a", environment); err != nil {
		t.Fatalf("UpdateEnvironmentForAgent() error = %v", err)
	}
	if len(matching.environmentUpdates) != 1 || matching.environmentUpdates[0]["NEXUS_WEBSEARCH_CONFIG"] == "" {
		t.Fatalf("matching environment updates = %#v", matching.environmentUpdates)
	}
	if len(otherRuntime.environmentUpdates) != 0 || len(otherAgent.environmentUpdates) != 0 {
		t.Fatalf("non-matching clients were updated: runtime=%#v other=%#v", otherRuntime.environmentUpdates, otherAgent.environmentUpdates)
	}
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
		map[string]string{"NEXUS_WEBSEARCH_CONFIG": `{"enabled":false}`},
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

func TestManagerGetOrCreateReconfiguresExistingClient(t *testing.T) {
	client := &fakeRuntimeClient{}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: client})

	first, err := manager.GetOrCreate(context.Background(), "agent:nexus:ws:dm:test", agentclient.Options{
		CWD: "/tmp/a",
		Env: map[string]string{"NEXUS_OPENAI_PROTOCOL": "chat_completions"},
	})
	if err != nil {
		t.Fatalf("首次创建 client 失败: %v", err)
	}
	second, err := manager.GetOrCreate(context.Background(), "agent:nexus:ws:dm:test", agentclient.Options{
		CWD: "/tmp/a",
		Env: map[string]string{"NEXUS_OPENAI_PROTOCOL": "responses"},
		Runtime: agentclient.RuntimeOptions{
			PermissionMode: sdkpermission.ModeAcceptEdits,
		},
	})
	if err != nil {
		t.Fatalf("复用 client 失败: %v", err)
	}

	if first != second {
		t.Fatal("期望复用同一个 client 实例")
	}
	if client.reconfigureCalls != 1 {
		t.Fatalf("期望调用一次 Reconfigure，实际 %d", client.reconfigureCalls)
	}
	if client.lastOptions.CWD != "/tmp/a" {
		t.Fatalf("Reconfigure 未收到最新配置: %+v", client.lastOptions)
	}
	if client.lastOptions.Runtime.PermissionMode != sdkpermission.ModeAcceptEdits {
		t.Fatalf("Reconfigure 未收到权限模式: %+v", client.lastOptions)
	}
	if client.lastOptions.Env["NEXUS_OPENAI_PROTOCOL"] != "responses" {
		t.Fatalf("Reconfigure 未收到 Responses 协议更新: %+v", client.lastOptions.Env)
	}
}

func TestManagerReplacesRuntimeWhenProcessPolicyChanges(t *testing.T) {
	stale := &fakeRuntimeClient{}
	fresh := &fakeRuntimeClient{}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{
		clients: []*fakeRuntimeClient{stale, fresh},
	})
	sessionKey := "agent:nexus:ws:dm:process-policy"
	firstOptions := agentclient.Options{
		CLIPath: "/opt/nexus/nxs",
		CWD:     "/srv/nexus/users/owner/workspace",
		Env: map[string]string{
			"NEXUS_RUNTIME_USER_ID":        "owner",
			"NEXUS_RUNTIME_ISOLATION_MODE": "enforce",
			"NEXUS_OPENAI_PROTOCOL":        "chat_completions",
		},
	}
	first, err := manager.GetOrCreate(context.Background(), sessionKey, firstOptions)
	if err != nil {
		t.Fatal(err)
	}
	nextOptions := firstOptions
	nextOptions.CWD = "/srv/nexus/users/owner/other-workspace"
	nextOptions.Env = maps.Clone(firstOptions.Env)
	nextOptions.Env["NEXUS_OPENAI_PROTOCOL"] = "responses"
	second, err := manager.GetOrCreate(context.Background(), sessionKey, nextOptions)
	if err != nil {
		t.Fatal(err)
	}
	if first != stale || second != fresh {
		t.Fatalf("process policy change did not replace runtime: first=%T second=%T", first, second)
	}
	if stale.reconfigureCalls != 0 || stale.disconnectCalls != 1 {
		t.Fatalf(
			"unsafe runtime should be replaced before Reconfigure: reconfigure=%d disconnect=%d",
			stale.reconfigureCalls,
			stale.disconnectCalls,
		)
	}
}

func TestManagerDisconnectsCandidateThatLosesConcurrentReplacement(t *testing.T) {
	stale := &fakeRuntimeClient{}
	winner := &fakeRuntimeClient{}
	loser := &fakeRuntimeClient{}
	manager := NewManager()
	sessionKey := "agent:nexus:ws:dm:concurrent-replacement"
	expectedState := &sessionState{Client: stale}
	manager.sessions[sessionKey] = expectedState
	startup, err := manager.BeginClientStartup(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatal(err)
	}
	defer startup.Close()

	candidateCreated := make(chan struct{})
	releaseCandidate := make(chan struct{})
	result := make(chan Client, 1)
	resultErr := make(chan error, 1)
	go func() {
		client, replaceErr := manager.replaceRuntimeClient(
			context.Background(),
			startup,
			sessionKey,
			expectedState,
			stale,
			agentclient.Options{},
			runtimeFactoryFunc(func(agentclient.Options) Client {
				close(candidateCreated)
				<-releaseCandidate
				return loser
			}),
		)
		result <- client
		resultErr <- replaceErr
	}()
	<-candidateCreated
	manager.mu.Lock()
	manager.sessions[sessionKey].Client = winner
	manager.mu.Unlock()
	close(releaseCandidate)

	if replaceErr := <-resultErr; !errors.Is(replaceErr, errRuntimeClientChanged) {
		t.Fatalf("replace error = %v, want errRuntimeClientChanged", replaceErr)
	}
	if client := <-result; client != nil {
		t.Fatalf("失去替换事务的候选不应返回 client: got=%T", client)
	}
	if loser.disconnectCalls != 1 {
		t.Fatalf("失去竞赛的候选 runtime 未关闭: disconnect=%d", loser.disconnectCalls)
	}
	if stale.disconnectCalls != 0 || winner.disconnectCalls != 0 {
		t.Fatalf(
			"并发竞赛不应关闭 stale/winner: stale=%d winner=%d",
			stale.disconnectCalls,
			winner.disconnectCalls,
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

func TestManagerRejectsSessionReuseAcrossOwners(t *testing.T) {
	client := &fakeRuntimeClient{}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: client})
	sessionKey := "agent:nexus:ws:dm:owner-boundary"

	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{
		Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner-a"},
	}); err != nil {
		t.Fatalf("创建 owner-a runtime 失败: %v", err)
	}
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{
		Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner-b"},
	}); err == nil || !strings.Contains(err.Error(), "runtime session owner mismatch") {
		t.Fatalf("跨 owner 复用应失败，err=%v", err)
	}
	if _, err := manager.GetOrCreate(
		context.Background(),
		sessionKey,
		agentclient.Options{},
	); err == nil || !strings.Contains(err.Error(), "runtime session owner mismatch") {
		t.Fatalf("缺失 owner 的请求也不能复用已绑定 session，err=%v", err)
	}
	if client.reconfigureCalls != 0 {
		t.Fatalf("跨 owner 请求不应进入旧 client: calls=%d", client.reconfigureCalls)
	}
}

func TestManagerGetOrCreateWithFactoryUsesRoomSlotFactory(t *testing.T) {
	defaultClient := &fakeRuntimeClient{}
	slotClient := &fakeRuntimeClient{}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: defaultClient})
	sessionKey := "agent:host:ws:group:conversation-1"

	got, err := manager.GetOrCreateWithFactory(
		context.Background(),
		sessionKey,
		agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeClaude}},
		&fakeRuntimeFactory{client: slotClient},
	)
	if err != nil {
		t.Fatalf("GetOrCreateWithFactory() error = %v", err)
	}
	if got != slotClient {
		t.Fatalf("client = %#v, want Room slot factory client", got)
	}
	if kind := manager.RuntimeKind(sessionKey); kind != agentclient.RuntimeClaude {
		t.Fatalf("RuntimeKind() = %q, want claude", kind)
	}
	manager.MarkSubagentHistory(sessionKey)
	if !manager.HasSubagentHistory(sessionKey) {
		t.Fatal("Room slot 的 subagent history 标记未保留")
	}
}

func TestManagerHasSessionTracksLiveClient(t *testing.T) {
	client := &fakeRuntimeClient{}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: client})
	sessionKey := "agent:host:ws:room:conversation-1"
	if manager.HasSession(sessionKey) {
		t.Fatal("尚未创建 client 时不应视为热会话")
	}
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{}); err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	if !manager.HasSession(sessionKey) {
		t.Fatal("创建 client 后应视为热会话")
	}
	if err := manager.CloseSession(context.Background(), sessionKey); err != nil {
		t.Fatalf("CloseSession() error = %v", err)
	}
	if manager.HasSession(sessionKey) {
		t.Fatal("CloseSession 后不应继续视为热会话")
	}
}

func TestManagerKeepsUnknownRuntimeKindConservative(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	sessionKey := "agent:host:ws:dm:unknown-runtime"
	if _, err := manager.GetOrCreate(
		context.Background(),
		sessionKey,
		agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeKind("custom")}},
	); err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	if kind := manager.RuntimeKind(sessionKey); kind != "" {
		t.Fatalf("unknown RuntimeKind() = %q, want empty conservative kind", kind)
	}
}

func TestManagerTaskControlsRefreshIdleDeadline(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	client := &fakeRuntimeClient{}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: client})
	manager.now = func() time.Time { return now }
	sessionKey := "agent:nexus:ws:dm:task-control-touch"
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{}); err != nil {
		t.Fatalf("创建 runtime client 失败: %v", err)
	}

	now = now.Add(5 * time.Minute)
	if err := manager.StopTask(context.Background(), sessionKey, "task-1"); err != nil {
		t.Fatalf("StopTask() error = %v", err)
	}
	if got := manager.sessions[sessionKey].LastUsedAt; !got.Equal(now) {
		t.Fatalf("StopTask LastUsedAt = %s, want %s", got, now)
	}
	if len(client.stoppedTasks) != 1 || client.stoppedTasks[0] != "task-1" {
		t.Fatalf("stoppedTasks = %+v, want task-1", client.stoppedTasks)
	}

	now = now.Add(3 * time.Minute)
	if err := manager.SendTaskMessage(context.Background(), sessionKey, "task-1", "继续", "继续"); err != nil {
		t.Fatalf("SendTaskMessage() error = %v", err)
	}
	if got := manager.sessions[sessionKey].LastUsedAt; !got.Equal(now) {
		t.Fatalf("SendTaskMessage LastUsedAt = %s, want %s", got, now)
	}
	if len(client.taskMessages) != 1 || client.taskMessages[0].TaskID != "task-1" {
		t.Fatalf("taskMessages = %+v, want task-1", client.taskMessages)
	}
}

func TestManagerInterruptSessionPublishesReasonBeforeInterruptingClient(t *testing.T) {
	manager := NewManager()
	sessionKey := "agent:nexus:ws:dm:interrupt"
	roundID := "round-1"
	reasonObserved := ""
	client := &fakeRuntimeClient{}
	client.interruptHook = func() {
		reasonObserved = manager.GetInterruptReason(sessionKey, roundID)
		manager.MarkRoundFinished(sessionKey, roundID)
	}

	manager.mu.Lock()
	state := manager.ensureStateLocked(sessionKey)
	state.Client = client
	manager.mu.Unlock()
	if err := manager.StartRound(context.Background(), sessionKey, roundID, nil); err != nil {
		t.Fatalf("StartRound() error = %v", err)
	}

	roundIDs, err := manager.InterruptSession(context.Background(), sessionKey, "  stop now  ")
	if err != nil {
		t.Fatalf("InterruptSession() error = %v", err)
	}
	if !slices.Equal(roundIDs, []string{roundID}) {
		t.Fatalf("InterruptSession() roundIDs = %v, want [%s]", roundIDs, roundID)
	}
	if reasonObserved != "stop now" {
		t.Fatalf("client interrupt 观察到 reason = %q, want %q", reasonObserved, "stop now")
	}
	if client.interruptCalls != 1 {
		t.Fatalf("Interrupt() calls = %d, want 1", client.interruptCalls)
	}
}

func TestManagerInterruptSessionWithoutRunningRoundDoesNotTouchClient(t *testing.T) {
	manager := NewManager()
	sessionKey := "agent:nexus:ws:dm:interrupt-idle"
	client := &fakeRuntimeClient{}

	manager.mu.Lock()
	state := manager.ensureStateLocked(sessionKey)
	state.Client = client
	manager.mu.Unlock()

	roundIDs, err := manager.InterruptSession(context.Background(), sessionKey, "stop")
	if err != nil {
		t.Fatalf("InterruptSession() error = %v", err)
	}
	if len(roundIDs) != 0 {
		t.Fatalf("InterruptSession() roundIDs = %v, want empty", roundIDs)
	}
	if client.interruptCalls != 0 {
		t.Fatalf("Interrupt() calls = %d, want 0", client.interruptCalls)
	}
}

func TestManagerIdleMessageDrainHandlesMessages(t *testing.T) {
	messages := make(chan sdkprotocol.ReceivedMessage, 1)
	client := &fakeRuntimeClient{messages: messages}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: client})
	sessionKey := "agent:nexus:ws:dm:test"
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{}); err != nil {
		t.Fatalf("创建 runtime client 失败: %v", err)
	}

	handled := make(chan struct{}, 1)
	manager.StartIdleMessageDrain(sessionKey, func(context.Context, sdkprotocol.ReceivedMessage) bool {
		handled <- struct{}{}
		return false
	})
	messages <- sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeTaskNotification}

	select {
	case <-handled:
	case <-time.After(time.Second):
		t.Fatal("idle drain 未处理后台 task 通知")
	}
}

func TestManagerStartRoundWaitsForIdleHandlerExit(t *testing.T) {
	messages := make(chan sdkprotocol.ReceivedMessage, 1)
	client := &fakeRuntimeClient{messages: messages}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: client})
	sessionKey := "agent:nexus:ws:dm:idle-handoff"
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

	startResult := make(chan error, 1)
	go func() {
		startResult <- manager.StartRound(context.Background(), sessionKey, "round-1", nil)
	}()
	<-handlerCanceled
	select {
	case err := <-startResult:
		t.Fatalf("idle handler 退出前 StartRound() 提前返回: %v", err)
	default:
	}

	close(releaseHandler)
	if err := <-startResult; err != nil {
		t.Fatalf("StartRound() error = %v", err)
	}
	manager.MarkRoundFinished(sessionKey, "round-1")
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

func TestManagerCloseSessionWaitsForIdleHandlerExit(t *testing.T) {
	messages := make(chan sdkprotocol.ReceivedMessage, 1)
	client := &fakeRuntimeClient{messages: messages}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: client})
	sessionKey := "agent:nexus:ws:dm:idle-close"
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

	closeResult := make(chan error, 1)
	go func() {
		closeResult <- manager.CloseSession(context.Background(), sessionKey)
	}()
	<-handlerCanceled
	select {
	case err := <-closeResult:
		t.Fatalf("idle handler 退出前 CloseSession() 提前返回: %v", err)
	default:
	}

	close(releaseHandler)
	if err := <-closeResult; err != nil {
		t.Fatalf("CloseSession() error = %v", err)
	}
}

func TestHasMCPServerChecksBothServerMaps(t *testing.T) {
	const serverName = "search"
	tests := []struct {
		name    string
		options agentclient.Options
		want    bool
	}{
		{
			name: "server config",
			options: agentclient.Options{MCP: agentclient.MCPOptions{Servers: map[string]sdkmcp.ServerConfig{
				serverName: sdkmcp.HTTPServerConfig{URL: "https://example.test/mcp"},
			}}},
			want: true,
		},
		{
			name: "legacy sdk server",
			options: agentclient.Options{MCP: agentclient.MCPOptions{SDKServers: map[string]sdkmcp.SDKMCPServer{
				serverName: fakeSDKMCPServer{},
			}}},
			want: true,
		},
		{name: "missing", options: agentclient.Options{}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := hasMCPServer(test.options, serverName); got != test.want {
				t.Fatalf("hasMCPServer() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestManagerGetOrCreateReplacesClientAfterTransportClosed(t *testing.T) {
	stale := &fakeRuntimeClient{
		reconfigureErr: errors.New("client: send control request failed: process: write payload failed: write |1: The pipe has been ended"),
	}
	fresh := &fakeRuntimeClient{}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{clients: []*fakeRuntimeClient{stale, fresh}})
	sessionKey := "agent:nexus:ws:dm:stale-client"

	first, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{
		CWD: "/tmp/a",
	})
	if err != nil {
		t.Fatalf("首次创建 client 失败: %v", err)
	}
	second, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{
		CWD: "/tmp/b",
	})
	if err != nil {
		t.Fatalf("transport 断开后应创建新 client: %v", err)
	}

	if first != stale {
		t.Fatalf("首次 client 不正确: %#v", first)
	}
	if second != fresh {
		t.Fatalf("transport 断开后未替换 client: got=%#v want=%#v", second, fresh)
	}
	if stale.disconnectCalls != 1 {
		t.Fatalf("旧 client 应被关闭一次: %d", stale.disconnectCalls)
	}
}

func TestManagerReplacementRetiresStaleClient(t *testing.T) {
	stale := &ownershipFenceClient{}
	fresh := &ownershipFenceClient{}
	manager := NewManagerWithFactory(&runtimeClientSequenceFactory{clients: []Client{stale, fresh}})
	sessionKey := "agent:nexus:ws:dm:retire-stale-client"
	t.Cleanup(func() {
		_ = manager.CloseSession(context.Background(), sessionKey)
	})

	first, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{})
	if err != nil {
		t.Fatalf("首次创建 client 失败: %v", err)
	}
	second, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{
		Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeClaude},
	})
	if err != nil {
		t.Fatalf("切换 runtime 失败: %v", err)
	}
	if first != stale || second != fresh {
		t.Fatalf("runtime 切换 client 不正确: first=%#v second=%#v", first, second)
	}
	if err := stale.Connect(context.Background()); !errors.Is(err, agentclient.ErrAborted) {
		t.Fatalf("旧 client 在换代后仍可 Connect(): %v", err)
	}

	stale.mu.Lock()
	retireCalls := stale.retireCalls
	disconnectCalls := stale.disconnectCalls
	stale.mu.Unlock()
	if retireCalls != 1 || disconnectCalls != 1 {
		t.Fatalf("旧 client 清理次数 = retire:%d disconnect:%d，期望各 1 次", retireCalls, disconnectCalls)
	}
}

func TestManagerReconfigureLostOwnershipRetriesCurrentClient(t *testing.T) {
	reconfigureStarted := make(chan struct{}, 1)
	reconfigureRelease := make(chan struct{}, 1)
	defer func() {
		select {
		case reconfigureRelease <- struct{}{}:
		default:
		}
	}()
	stale := &ownershipFenceClient{
		reconfigureStarted: reconfigureStarted,
		reconfigureRelease: reconfigureRelease,
	}
	fresh := &ownershipFenceClient{}
	manager := NewManagerWithFactory(&runtimeClientSequenceFactory{clients: []Client{stale}})
	sessionKey := "agent:nexus:ws:dm:reconfigure-ownership"
	t.Cleanup(func() {
		_ = manager.CloseSession(context.Background(), sessionKey)
	})
	options := agentclient.Options{CWD: "/tmp/runtime-owner"}

	initialStartup, err := manager.BeginClientStartup(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatalf("开始首次启动事务失败: %v", err)
	}
	if _, err = initialStartup.GetOrCreateWithFactory(context.Background(), options, nil); err != nil {
		initialStartup.Close()
		t.Fatalf("首次创建 client 失败: %v", err)
	}
	if err = initialStartup.Connect(context.Background()); err != nil {
		initialStartup.Close()
		t.Fatalf("首次连接 client 失败: %v", err)
	}
	initialStartup.Close()
	staleLease, ok := manager.CaptureClientLease(sessionKey, stale)
	if !ok {
		t.Fatal("未捕获旧 client lease")
	}
	resultCh := make(chan runtimeClientResult, 1)
	go func() {
		client, err := manager.GetOrCreate(context.Background(), sessionKey, options)
		resultCh <- runtimeClientResult{client: client, err: err}
	}()

	select {
	case <-reconfigureStarted:
	case <-time.After(time.Second):
		t.Fatal("旧 client 未进入 Reconfigure()")
	}
	manager.mu.RLock()
	expectedState := manager.sessions[sessionKey]
	manager.mu.RUnlock()
	if _, err := manager.replaceRuntimeClient(
		context.Background(),
		nil,
		sessionKey,
		expectedState,
		stale,
		options,
		&runtimeClientSequenceFactory{clients: []Client{fresh}},
	); err != nil {
		t.Fatalf("并发替换 client 失败: %v", err)
	}
	reconfigureRelease <- struct{}{}

	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("失去所有权后的 GetOrCreate() error = %v", result.err)
		}
		if result.client != fresh {
			t.Fatalf("失去所有权后返回 client = %#v，期望当前 fresh client", result.client)
		}
	case <-time.After(time.Second):
		t.Fatal("失去所有权后的 GetOrCreate() 未返回")
	}

	closed, err := manager.CloseSessionIfLease(context.Background(), staleLease)
	if err != nil || closed {
		t.Fatalf("旧 client 条件关闭结果 = closed:%v err:%v，期望忽略", closed, err)
	}
	if current := manager.SessionClient(sessionKey); current != fresh {
		t.Fatalf("旧 client 条件关闭影响了当前 client: %#v", current)
	}
	fresh.mu.Lock()
	freshReconfigureCalls := fresh.reconfigureCalls
	freshDisconnectCalls := fresh.disconnectCalls
	fresh.mu.Unlock()
	if freshReconfigureCalls != 1 || freshDisconnectCalls != 0 {
		t.Fatalf("当前 client 调用次数 = reconfigure:%d disconnect:%d", freshReconfigureCalls, freshDisconnectCalls)
	}
}

func TestManagerReplacementDoesNotReturnNextAfterConcurrentClose(t *testing.T) {
	disconnectStarted := make(chan struct{}, 2)
	disconnectRelease := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(disconnectRelease)
		}
	}()
	stale := &ownershipFenceClient{
		disconnectStarted: disconnectStarted,
		disconnectRelease: disconnectRelease,
	}
	next := &ownershipFenceClient{}
	manager := NewManagerWithFactory(&runtimeClientSequenceFactory{clients: []Client{stale}})
	sessionKey := "agent:nexus:ws:dm:replace-close-race"
	options := agentclient.Options{}

	if _, err := manager.GetOrCreate(context.Background(), sessionKey, options); err != nil {
		t.Fatalf("首次创建 client 失败: %v", err)
	}
	manager.mu.RLock()
	expectedState := manager.sessions[sessionKey]
	manager.mu.RUnlock()
	resultCh := make(chan runtimeClientResult, 1)
	go func() {
		client, err := manager.replaceRuntimeClient(
			context.Background(),
			nil,
			sessionKey,
			expectedState,
			stale,
			options,
			&runtimeClientSequenceFactory{clients: []Client{next}},
		)
		resultCh <- runtimeClientResult{client: client, err: err}
	}()

	select {
	case <-disconnectStarted:
	case <-time.After(time.Second):
		t.Fatal("替换流程未进入旧 client Disconnect()")
	}
	closeResult := make(chan error, 1)
	go func() {
		closeResult <- manager.CloseSession(context.Background(), sessionKey)
	}()
	select {
	case <-disconnectStarted:
	case <-time.After(time.Second):
		t.Fatal("并发 CloseSession() 未进入旧 client Disconnect()")
	}
	close(disconnectRelease)
	released = true
	if err := <-closeResult; err != nil {
		t.Fatalf("并发 CloseSession() error = %v", err)
	}

	select {
	case result := <-resultCh:
		if result.client != nil {
			t.Fatalf("并发关闭后替换流程仍返回 client: %#v", result.client)
		}
		if !errors.Is(result.err, agentclient.ErrAborted) && !errors.Is(result.err, ErrRuntimeSessionClosing) {
			t.Fatalf("并发关闭后替换流程 error = %v，期望 ownership 中止", result.err)
		}
	case <-time.After(time.Second):
		t.Fatal("并发关闭后的替换流程未返回")
	}
	if err := next.Connect(context.Background()); !errors.Is(err, agentclient.ErrAborted) {
		t.Fatalf("并发关闭后的 next client 仍可 Connect(): %v", err)
	}
}

func TestManagerStartupWaitsForRetiredClientCleanupBeforePublishingNext(t *testing.T) {
	disconnectStarted := make(chan struct{}, 1)
	disconnectRelease := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(disconnectRelease)
		}
	}()
	stale := &ownershipFenceClient{
		disconnectStarted: disconnectStarted,
		disconnectRelease: disconnectRelease,
	}
	next := &ownershipFenceClient{}
	factory := &runtimeClientSequenceFactory{clients: []Client{stale, next}}
	manager := NewManagerWithFactory(factory)
	sessionKey := "agent:nexus:ws:dm:replacement-cleanup-gate"
	t.Cleanup(func() {
		_ = manager.CloseSession(context.Background(), sessionKey)
	})
	options := agentclient.Options{
		Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeClaude},
	}

	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{}); err != nil {
		t.Fatalf("首次创建 client 失败: %v", err)
	}
	replacementCh := make(chan runtimeClientResult, 1)
	go func() {
		client, err := manager.GetOrCreate(context.Background(), sessionKey, options)
		replacementCh <- runtimeClientResult{client: client, err: err}
	}()
	select {
	case <-disconnectStarted:
	case <-time.After(time.Second):
		t.Fatal("替换流程未进入旧 client Disconnect()")
	}

	observerCh := make(chan runtimeClientResult, 1)
	go func() {
		startup, err := manager.BeginClientStartup(context.Background(), sessionKey, "")
		if err != nil {
			observerCh <- runtimeClientResult{err: err}
			return
		}
		defer startup.Close()
		client, err := startup.GetOrCreateWithFactory(context.Background(), options, nil)
		if err == nil {
			err = startup.Connect(context.Background())
		}
		observerCh <- runtimeClientResult{client: client, err: err}
	}()
	waitRuntimeStartupGateRefs(t, manager, sessionKey, 2)
	if current := manager.SessionClient(sessionKey); current != stale {
		t.Fatalf("旧进程清理完成前发布了 next client: %#v", current)
	}
	next.mu.Lock()
	nextConnectCalls := next.connectCalls
	next.mu.Unlock()
	if nextConnectCalls != 0 {
		t.Fatalf("旧进程清理完成前 next Connect() 次数 = %d", nextConnectCalls)
	}
	select {
	case result := <-observerCh:
		t.Fatalf("观察者绕过 cleanup gate 提前返回: %+v", result)
	default:
	}

	close(disconnectRelease)
	released = true
	select {
	case result := <-replacementCh:
		if result.err != nil || result.client != next {
			t.Fatalf("替换结果 = client:%#v err:%v", result.client, result.err)
		}
	case <-time.After(time.Second):
		t.Fatal("旧进程清理后替换流程未返回")
	}
	select {
	case result := <-observerCh:
		if result.err != nil || result.client != next {
			t.Fatalf("排队观察者结果 = client:%#v err:%v", result.client, result.err)
		}
	case <-time.After(time.Second):
		t.Fatal("旧进程清理后排队观察者未返回")
	}
}

func TestManagerCloseDeadlineKeepsLifecycleFenceUntilClientCleanupFinishes(t *testing.T) {
	disconnectStarted := make(chan struct{}, 2)
	disconnectRelease := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(disconnectRelease)
		}
	}()
	client := &ownershipFenceClient{
		disconnectStarted: disconnectStarted,
		disconnectRelease: disconnectRelease,
	}
	factory := &runtimeClientSequenceFactory{clients: []Client{client, &ownershipFenceClient{}}}
	manager := NewManagerWithFactory(factory)
	sessionKey := "agent:nexus:ws:dm:close-cleanup-fence"
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{}); err != nil {
		t.Fatalf("首次创建 client 失败: %v", err)
	}

	closeCtx, cancelClose := context.WithCancel(context.Background())
	closeResult := make(chan error, 1)
	go func() {
		closeResult <- manager.CloseSession(closeCtx, sessionKey)
	}()
	select {
	case <-disconnectStarted:
	case <-time.After(time.Second):
		t.Fatal("CloseSession() 未进入 client Disconnect()")
	}
	cancelClose()
	select {
	case err := <-closeResult:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("CloseSession() error = %v，期望 context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("取消后 CloseSession() 未返回")
	}
	select {
	case <-disconnectStarted:
	case <-time.After(time.Second):
		t.Fatal("CloseSession() 未在后台继续等待 client cleanup")
	}

	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{}); !errors.Is(err, ErrRuntimeSessionClosing) {
		t.Fatalf("client cleanup 未完成时 GetOrCreate() error = %v，期望 session closing", err)
	}
	factory.mu.Lock()
	factoryCalls := factory.index
	factory.mu.Unlock()
	if factoryCalls != 1 {
		t.Fatalf("client cleanup 未完成时 factory 调用次数 = %d，期望 1", factoryCalls)
	}

	close(disconnectRelease)
	released = true
	waitRuntimeSessionRemoved(t, manager, sessionKey)
}

func TestManagerOldClientLeaseCannotCloseNewConnectionGeneration(t *testing.T) {
	client := &ownershipFenceClient{}
	manager := NewManagerWithFactory(&runtimeClientSequenceFactory{clients: []Client{client}})
	sessionKey := "agent:nexus:ws:dm:client-generation-lease"
	t.Cleanup(func() {
		_ = manager.CloseSession(context.Background(), sessionKey)
	})

	firstStartup, err := manager.BeginClientStartup(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatalf("开始首次启动事务失败: %v", err)
	}
	if _, err = firstStartup.GetOrCreateWithFactory(context.Background(), agentclient.Options{}, nil); err != nil {
		t.Fatalf("首次 GetOrCreate() 失败: %v", err)
	}
	if err = firstStartup.Connect(context.Background()); err != nil {
		t.Fatalf("首次 Connect() 失败: %v", err)
	}
	firstStartup.Close()
	oldLease, ok := manager.CaptureClientLease(sessionKey, client)
	if !ok {
		t.Fatal("未捕获首次连接 lease")
	}

	secondStartup, err := manager.BeginClientStartup(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatalf("开始第二次启动事务失败: %v", err)
	}
	if _, err = secondStartup.GetOrCreateWithFactory(context.Background(), agentclient.Options{}, nil); err != nil {
		t.Fatalf("第二次 GetOrCreate() 失败: %v", err)
	}
	if err = secondStartup.Connect(context.Background()); err != nil {
		t.Fatalf("第二次 Connect() 失败: %v", err)
	}
	secondStartup.Close()

	closed, err := manager.CloseSessionIfLease(context.Background(), oldLease)
	if err != nil || closed {
		t.Fatalf("旧 lease 关闭结果 = closed:%v err:%v，期望忽略", closed, err)
	}
	if current := manager.SessionClient(sessionKey); current != client {
		t.Fatalf("旧 lease 影响了新连接: %#v", current)
	}
	client.mu.Lock()
	retireCalls := client.retireCalls
	disconnectCalls := client.disconnectCalls
	connectCalls := client.connectCalls
	client.mu.Unlock()
	if retireCalls != 0 || disconnectCalls != 0 || connectCalls != 2 {
		t.Fatalf(
			"client 调用次数 = connect:%d retire:%d disconnect:%d",
			connectCalls,
			retireCalls,
			disconnectCalls,
		)
	}
}

func TestManagerCloseInvalidatesStartupThatHasNotCreatedState(t *testing.T) {
	client := &ownershipFenceClient{}
	factory := &runtimeClientSequenceFactory{clients: []Client{client}}
	manager := NewManagerWithFactory(factory)
	sessionKey := "agent:nexus:ws:dm:close-before-state"
	startup, err := manager.BeginClientStartup(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatalf("开始启动事务失败: %v", err)
	}

	closeResult := make(chan error, 1)
	go func() {
		closeResult <- manager.CloseSession(context.Background(), sessionKey)
	}()
	select {
	case err = <-closeResult:
		if err != nil {
			t.Fatalf("CloseSession() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("CloseSession() 未使尚未创建 state 的启动事务失效")
	}
	if _, err = startup.GetOrCreateWithFactory(context.Background(), agentclient.Options{}, nil); !errors.Is(err, agentclient.ErrAborted) {
		t.Fatalf("关闭后的启动事务 GetOrCreate() error = %v，期望 ErrAborted", err)
	}
	startup.Close()
	if current := manager.SessionClient(sessionKey); current != nil {
		t.Fatalf("CloseSession() 后仍有 client: %#v", current)
	}
	factory.mu.Lock()
	factoryCalls := factory.index
	factory.mu.Unlock()
	if factoryCalls != 0 {
		t.Fatalf("失效的启动事务仍创建了 %d 个 client", factoryCalls)
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

func TestManagerRetireCurrentPreservesBackgroundTaskForStartupRetry(t *testing.T) {
	stale := &ownershipFenceClient{}
	fresh := &ownershipFenceClient{}
	manager := NewManagerWithFactory(&runtimeClientSequenceFactory{clients: []Client{stale, fresh}})
	sessionKey := "agent:nexus:ws:dm:retire-current-background-retry"
	taskStarted := make(chan struct{})
	taskRelease := make(chan struct{})
	taskCanceled := make(chan struct{}, 1)
	if !manager.StartBackgroundTask(sessionKey, func(ctx context.Context) {
		close(taskStarted)
		select {
		case <-ctx.Done():
			taskCanceled <- struct{}{}
		case <-taskRelease:
		}
	}) {
		t.Fatal("登记后台任务失败")
	}
	<-taskStarted

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
	retired, err := startup.RetireCurrent(context.Background())
	if err != nil || !retired {
		startup.Close()
		t.Fatalf("RetireCurrent() = retired:%v err:%v", retired, err)
	}
	select {
	case <-taskCanceled:
		startup.Close()
		t.Fatal("client 启动失败清理不应取消当前 session 的后台任务")
	default:
	}
	if current := manager.SessionClient(sessionKey); current != nil {
		startup.Close()
		t.Fatalf("RetireCurrent() 后仍发布旧 client: %#v", current)
	}
	client, err := startup.GetOrCreateWithFactory(context.Background(), agentclient.Options{}, nil)
	if err != nil || client != fresh {
		startup.Close()
		t.Fatalf("RetireCurrent() 后 retry client = %#v err = %v，期望 fresh", client, err)
	}
	if err = startup.Connect(context.Background()); err != nil {
		startup.Close()
		t.Fatalf("连接 retry client 失败: %v", err)
	}
	startup.Close()
	close(taskRelease)
	waitCtx, cancelWait := context.WithTimeout(context.Background(), time.Second)
	defer cancelWait()
	if err = manager.WaitBackgroundTasks(waitCtx, sessionKey); err != nil {
		t.Fatalf("等待后台任务退出失败: %v", err)
	}
	if err = manager.CloseSession(context.Background(), sessionKey); err != nil {
		t.Fatalf("清理 retry client 失败: %v", err)
	}
}

func TestManagerRetireExistingReplacesWarmClientBeforeGetOrCreate(t *testing.T) {
	stale := &ownershipFenceClient{}
	fresh := &ownershipFenceClient{}
	manager := NewManagerWithFactory(&runtimeClientSequenceFactory{clients: []Client{stale, fresh}})
	sessionKey := "agent:nexus:ws:dm:retire-existing-warm-client"

	initial, err := manager.BeginClientStartup(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatalf("开始首次启动事务失败: %v", err)
	}
	if _, err = initial.GetOrCreateWithFactory(context.Background(), agentclient.Options{}, nil); err != nil {
		initial.Close()
		t.Fatalf("创建 warm client 失败: %v", err)
	}
	if err = initial.Connect(context.Background()); err != nil {
		initial.Close()
		t.Fatalf("连接 warm client 失败: %v", err)
	}
	initial.Close()

	replacement, err := manager.BeginClientStartup(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatalf("开始换代事务失败: %v", err)
	}
	retired, err := replacement.RetireExisting(context.Background())
	if err != nil || !retired {
		replacement.Close()
		t.Fatalf("RetireExisting() = retired:%v err:%v", retired, err)
	}
	if current := manager.SessionClient(sessionKey); current != nil {
		replacement.Close()
		t.Fatalf("RetireExisting() 后仍发布旧 client: %#v", current)
	}
	client, err := replacement.GetOrCreateWithFactory(context.Background(), agentclient.Options{}, nil)
	if err != nil || client != fresh {
		replacement.Close()
		t.Fatalf("换代后 client = %#v err=%v, want fresh", client, err)
	}
	if err = replacement.Connect(context.Background()); err != nil {
		replacement.Close()
		t.Fatalf("连接 fresh client 失败: %v", err)
	}
	replacement.Close()

	stale.mu.Lock()
	retireCalls := stale.retireCalls
	disconnectCalls := stale.disconnectCalls
	stale.mu.Unlock()
	if retireCalls != 1 || disconnectCalls != 1 {
		t.Fatalf("warm client cleanup = retire:%d disconnect:%d, want 1/1", retireCalls, disconnectCalls)
	}
	if err = manager.CloseSession(context.Background(), sessionKey); err != nil {
		t.Fatalf("清理 fresh client 失败: %v", err)
	}
}

func TestManagerRetireExistingRejectsOwnerMismatch(t *testing.T) {
	stale := &ownershipFenceClient{}
	manager := NewManagerWithFactory(&runtimeClientSequenceFactory{clients: []Client{stale}})
	sessionKey := "agent:nexus:ws:dm:retire-existing-owner-fence"

	initial, err := manager.BeginClientStartup(context.Background(), sessionKey, "owner-a")
	if err != nil {
		t.Fatalf("开始 owner-a 启动事务失败: %v", err)
	}
	if _, err = initial.GetOrCreateWithFactory(context.Background(), agentclient.Options{
		Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner-a"},
	}, nil); err != nil {
		initial.Close()
		t.Fatalf("创建 owner-a client 失败: %v", err)
	}
	if err = initial.Connect(context.Background()); err != nil {
		initial.Close()
		t.Fatalf("连接 owner-a client 失败: %v", err)
	}
	initial.Close()

	replacement, err := manager.BeginClientStartup(context.Background(), sessionKey, "owner-b")
	if err != nil {
		t.Fatalf("开始 owner-b 启动事务失败: %v", err)
	}
	retired, retireErr := replacement.RetireExisting(context.Background())
	replacement.Close()
	if retired || retireErr == nil || !strings.Contains(retireErr.Error(), "runtime session owner mismatch") {
		t.Fatalf("owner mismatch RetireExisting() = retired:%v err:%v", retired, retireErr)
	}
	if current := manager.SessionClient(sessionKey); current != stale {
		t.Fatalf("owner mismatch 不应移除原 client: %#v", current)
	}
	if err = manager.CloseSession(context.Background(), sessionKey); err != nil {
		t.Fatalf("清理 owner-a client 失败: %v", err)
	}
}

func TestManagerRetireCurrentTimeoutDoesNotAbortInFlightStartupRetry(t *testing.T) {
	disconnectStarted := make(chan struct{}, 2)
	disconnectRelease := make(chan struct{})
	reconfigureStarted := make(chan struct{}, 1)
	reconfigureRelease := make(chan struct{})
	stale := &ownershipFenceClient{
		disconnectStarted:  disconnectStarted,
		disconnectRelease:  disconnectRelease,
		reconfigureStarted: reconfigureStarted,
		reconfigureRelease: reconfigureRelease,
	}
	fresh := &ownershipFenceClient{}
	manager := NewManagerWithFactory(&runtimeClientSequenceFactory{clients: []Client{stale, fresh}})
	sessionKey := "agent:nexus:ws:dm:retire-current-timeout-retry"

	startup, err := manager.BeginClientStartup(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatalf("开始首次启动事务失败: %v", err)
	}
	if _, err = startup.GetOrCreateWithFactory(context.Background(), agentclient.Options{}, nil); err != nil {
		startup.Close()
		t.Fatalf("创建旧 client 失败: %v", err)
	}
	if err = startup.Connect(context.Background()); err != nil {
		startup.Close()
		t.Fatalf("连接旧 client 失败: %v", err)
	}
	disconnectCtx, cancelDisconnect := context.WithCancel(context.Background())
	cancelDisconnect()
	retired, err := startup.RetireCurrent(disconnectCtx)
	if !retired || !errors.Is(err, context.Canceled) {
		startup.Close()
		t.Fatalf("RetireCurrent() = retired:%v err:%v，期望 timeout 后台清理", retired, err)
	}
	for range 2 {
		select {
		case <-disconnectStarted:
		case <-time.After(time.Second):
			startup.Close()
			t.Fatal("retired client 未进入后台 Disconnect()")
		}
	}
	startup.Close()

	retryStartup, err := manager.BeginClientStartup(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatalf("开始 retry 启动事务失败: %v", err)
	}
	retryResult := make(chan runtimeClientResult, 1)
	go func() {
		defer retryStartup.Close()
		client, retryErr := retryStartup.GetOrCreateWithFactory(context.Background(), agentclient.Options{}, nil)
		if retryErr == nil {
			retryErr = retryStartup.Connect(context.Background())
		}
		retryResult <- runtimeClientResult{client: client, err: retryErr}
	}()
	select {
	case <-reconfigureStarted:
	case <-time.After(time.Second):
		close(reconfigureRelease)
		close(disconnectRelease)
		t.Fatal("retry 未在锁外重配置 retired client")
	}

	close(disconnectRelease)
	waitRuntimeSessionClient(t, manager, sessionKey, nil)
	close(reconfigureRelease)
	select {
	case result := <-retryResult:
		if result.err != nil || result.client != fresh {
			t.Fatalf("timeout cleanup 期间 retry = client:%#v err:%v，期望 fresh", result.client, result.err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout cleanup 后 retry 未返回")
	}
	if err = manager.CloseSession(context.Background(), sessionKey); err != nil {
		t.Fatalf("清理 retry client 失败: %v", err)
	}
}

func TestManagerRetiredClientlessStateRemovedAfterLastLifecycleExits(t *testing.T) {
	t.Run("background", func(t *testing.T) {
		stale := &ownershipFenceClient{}
		manager := NewManagerWithFactory(&runtimeClientSequenceFactory{clients: []Client{stale}})
		sessionKey := "agent:nexus:ws:dm:retired-background-exit"
		taskStarted := make(chan struct{})
		taskRelease := make(chan struct{})
		if !manager.StartBackgroundTask(sessionKey, func(context.Context) {
			close(taskStarted)
			<-taskRelease
		}) {
			t.Fatal("登记后台任务失败")
		}
		<-taskStarted

		startup := connectRuntimeStartupForTest(t, manager, sessionKey)
		if retired, err := startup.RetireCurrent(context.Background()); err != nil || !retired {
			startup.Close()
			t.Fatalf("RetireCurrent() = retired:%v err:%v", retired, err)
		}
		startup.Close()
		if state := runtimeSessionStateForTest(manager, sessionKey); state == nil {
			t.Fatal("后台任务退出前不应删除 clientless state")
		}
		close(taskRelease)
		waitRuntimeSessionRemoved(t, manager, sessionKey)
	})

	t.Run("round", func(t *testing.T) {
		stale := &ownershipFenceClient{}
		manager := NewManagerWithFactory(&runtimeClientSequenceFactory{clients: []Client{stale}})
		sessionKey := "agent:nexus:ws:dm:retired-round-exit"
		startup := connectRuntimeStartupForTest(t, manager, sessionKey)
		if err := manager.StartRound(context.Background(), sessionKey, "round-1", nil); err != nil {
			startup.Close()
			t.Fatalf("登记 round 失败: %v", err)
		}
		if retired, err := startup.RetireCurrent(context.Background()); err != nil || !retired {
			startup.Close()
			t.Fatalf("RetireCurrent() = retired:%v err:%v", retired, err)
		}
		startup.Close()
		if state := runtimeSessionStateForTest(manager, sessionKey); state == nil {
			t.Fatal("round 退出前不应删除 clientless state")
		}
		manager.MarkRoundFinished(sessionKey, "round-1")
		waitRuntimeSessionRemoved(t, manager, sessionKey)
	})

	t.Run("idle drain", func(t *testing.T) {
		messages := make(chan sdkprotocol.ReceivedMessage)
		receiveStarted := make(chan struct{}, 1)
		stale := &ownershipFenceClient{fakeRuntimeClient: fakeRuntimeClient{
			messages:       messages,
			receiveStarted: receiveStarted,
		}}
		manager := NewManagerWithFactory(&runtimeClientSequenceFactory{clients: []Client{stale}})
		sessionKey := "agent:nexus:ws:dm:retired-idle-drain-exit"
		startup := connectRuntimeStartupForTest(t, manager, sessionKey)
		manager.StartIdleMessageDrain(sessionKey, func(context.Context, sdkprotocol.ReceivedMessage) bool {
			return true
		})
		select {
		case <-receiveStarted:
		case <-time.After(time.Second):
			startup.Close()
			t.Fatal("idle drain 未启动")
		}
		if retired, err := startup.RetireCurrent(context.Background()); err != nil || !retired {
			startup.Close()
			t.Fatalf("RetireCurrent() = retired:%v err:%v", retired, err)
		}
		startup.Close()
		waitRuntimeSessionRemoved(t, manager, sessionKey)
	})
}

func TestManagerRetireCurrentImmediateRetryKeepsNewIdleDrain(t *testing.T) {
	staleMessages := make(chan sdkprotocol.ReceivedMessage)
	freshMessages := make(chan sdkprotocol.ReceivedMessage)
	staleStarted := make(chan struct{}, 2)
	freshStarted := make(chan struct{}, 1)
	stale := &ownershipFenceClient{fakeRuntimeClient: fakeRuntimeClient{
		messages:       staleMessages,
		receiveStarted: staleStarted,
	}}
	fresh := &ownershipFenceClient{fakeRuntimeClient: fakeRuntimeClient{
		messages:       freshMessages,
		receiveStarted: freshStarted,
	}}
	manager := NewManagerWithFactory(&runtimeClientSequenceFactory{clients: []Client{stale, fresh}})
	sessionKey := "agent:nexus:ws:dm:retire-current-new-idle-drain"
	startup := connectRuntimeStartupForTest(t, manager, sessionKey)
	manager.StartIdleMessageDrain(sessionKey, func(context.Context, sdkprotocol.ReceivedMessage) bool {
		return true
	})
	select {
	case <-staleStarted:
	case <-time.After(time.Second):
		startup.Close()
		t.Fatal("旧 idle drain 未启动")
	}
	if retired, err := startup.RetireCurrent(context.Background()); err != nil || !retired {
		startup.Close()
		t.Fatalf("RetireCurrent() = retired:%v err:%v", retired, err)
	}
	client, err := startup.GetOrCreateWithFactory(context.Background(), agentclient.Options{}, nil)
	if err != nil || client != fresh {
		startup.Close()
		t.Fatalf("立即 retry = client:%#v err:%v，期望 fresh", client, err)
	}
	if err = startup.Connect(context.Background()); err != nil {
		startup.Close()
		t.Fatalf("连接 fresh client 失败: %v", err)
	}
	manager.StartIdleMessageDrain(sessionKey, func(context.Context, sdkprotocol.ReceivedMessage) bool {
		return true
	})
	select {
	case <-freshStarted:
	case <-time.After(time.Second):
		startup.Close()
		t.Fatal("新 idle drain 未启动")
	}

	state := runtimeSessionStateForTest(manager, sessionKey)
	if state == nil {
		startup.Close()
		t.Fatal("retry 后 session state 不存在")
	}
	newDrain := state.IdleMessageDrain
	staleCtx, cancelStale := context.WithCancel(context.Background())
	cancelStale()
	staleDrain := &idleMessageDrain{
		cancel: func() {},
		done:   make(chan struct{}),
	}
	staleDrainDone := make(chan struct{})
	go func() {
		manager.runIdleMessageDrain(staleCtx, sessionKey, state, staleDrain, nil, stale, func(context.Context, sdkprotocol.ReceivedMessage) bool {
			return true
		})
		close(staleDrainDone)
	}()
	select {
	case <-staleDrainDone:
	case <-time.After(time.Second):
		startup.Close()
		t.Fatal("旧 idle drain 未退出")
	}
	manager.mu.RLock()
	current := manager.sessions[sessionKey]
	newDrainKept := current == state && current.Client == fresh && current.IdleMessageDrain == newDrain
	manager.mu.RUnlock()
	if !newDrainKept {
		startup.Close()
		t.Fatal("旧 idle drain defer 误清了 retry 后的新 drain")
	}
	startup.Close()
	if err = manager.CloseSession(context.Background(), sessionKey); err != nil {
		t.Fatalf("清理 fresh client 失败: %v", err)
	}
}

func TestManagerRetireCurrentPreservesClientlessStateOwner(t *testing.T) {
	stale := &ownershipFenceClient{}
	fresh := &ownershipFenceClient{}
	factory := &runtimeClientSequenceFactory{clients: []Client{stale, fresh}}
	manager := NewManagerWithFactory(factory)
	sessionKey := "agent:nexus:ws:dm:retire-current-owner"
	taskStarted := make(chan struct{})
	taskRelease := make(chan struct{})
	if !manager.StartBackgroundTaskForOwner(sessionKey, "owner-a", func(context.Context) {
		close(taskStarted)
		<-taskRelease
	}) {
		t.Fatal("登记 owner 后台任务失败")
	}
	<-taskStarted

	startup, err := manager.BeginClientStartup(context.Background(), sessionKey, "owner-a")
	if err != nil {
		t.Fatalf("开始启动事务失败: %v", err)
	}
	ownerOptions := agentclient.Options{Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner-a"}}
	if _, err = startup.GetOrCreateWithFactory(context.Background(), ownerOptions, nil); err != nil {
		startup.Close()
		t.Fatalf("owner-a 创建 client 失败: %v", err)
	}
	if err = startup.Connect(context.Background()); err != nil {
		startup.Close()
		t.Fatalf("owner-a 连接 client 失败: %v", err)
	}
	if retired, retireErr := startup.RetireCurrent(context.Background()); retireErr != nil || !retired {
		startup.Close()
		t.Fatalf("RetireCurrent() = retired:%v err:%v", retired, retireErr)
	}
	otherOwnerOptions := agentclient.Options{Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner-b"}}
	if client, otherErr := startup.GetOrCreateWithFactory(context.Background(), otherOwnerOptions, nil); otherErr == nil || client != nil {
		startup.Close()
		t.Fatalf("owner-b 接管 clientless state = client:%#v err:%v", client, otherErr)
	}
	factory.mu.Lock()
	factoryCalls := factory.index
	factory.mu.Unlock()
	if factoryCalls != 1 {
		startup.Close()
		t.Fatalf("owner mismatch 后 factory 调用次数 = %d，期望 1", factoryCalls)
	}
	client, err := startup.GetOrCreateWithFactory(context.Background(), ownerOptions, nil)
	if err != nil || client != fresh {
		startup.Close()
		t.Fatalf("原 owner retry = client:%#v err:%v，期望 fresh", client, err)
	}
	if err = startup.Connect(context.Background()); err != nil {
		startup.Close()
		t.Fatalf("原 owner 连接 fresh client 失败: %v", err)
	}
	startup.Close()
	close(taskRelease)
	if err = manager.CloseSession(context.Background(), sessionKey); err != nil {
		t.Fatalf("清理 owner retry client 失败: %v", err)
	}
}

func TestManagerConcurrentCloseFencesDrainAfterOneCallerTimesOut(t *testing.T) {
	disconnectStarted := make(chan struct{}, 2)
	disconnectRelease := make(chan struct{})
	stale := &ownershipFenceClient{
		disconnectStarted: disconnectStarted,
		disconnectRelease: disconnectRelease,
	}
	fresh := &ownershipFenceClient{}
	factory := &runtimeClientSequenceFactory{clients: []Client{stale, fresh}}
	manager := NewManagerWithFactory(factory)
	sessionKey := "agent:nexus:ws:dm:concurrent-close-fences"
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{}); err != nil {
		t.Fatalf("创建旧 client 失败: %v", err)
	}

	firstCtx, cancelFirst := context.WithCancel(context.Background())
	firstResult := make(chan error, 1)
	go func() {
		firstResult <- manager.CloseSession(firstCtx, sessionKey)
	}()
	select {
	case <-disconnectStarted:
	case <-time.After(time.Second):
		t.Fatal("首次 CloseSession() 未进入 Disconnect()")
	}
	secondResult := make(chan error, 1)
	go func() {
		secondResult <- manager.CloseSession(context.Background(), sessionKey)
	}()
	waitRuntimeStartupGateCloseBlocks(t, manager, sessionKey, 2)
	cancelFirst()
	if err := <-firstResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("首次 CloseSession() error = %v，期望 context.Canceled", err)
	}
	if _, err := manager.BeginClientStartup(context.Background(), sessionKey, ""); !errors.Is(err, ErrRuntimeSessionClosing) {
		t.Fatalf("并发关闭未完成时 BeginClientStartup() error = %v，期望 session closing", err)
	}
	close(disconnectRelease)
	select {
	case err := <-secondResult:
		if err != nil {
			t.Fatalf("第二次 CloseSession() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("client cleanup 完成后第二次 CloseSession() 未返回")
	}

	startup, err := manager.BeginClientStartup(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatalf("关闭栅栏 drain 后 BeginClientStartup() error = %v", err)
	}
	client, err := startup.GetOrCreateWithFactory(context.Background(), agentclient.Options{}, nil)
	startup.Close()
	if err != nil || client != fresh {
		t.Fatalf("关闭栅栏 drain 后 client = %#v error = %v，期望 fresh", client, err)
	}
	if err := manager.CloseSession(context.Background(), sessionKey); err != nil {
		t.Fatalf("清理新 client 失败: %v", err)
	}
}

func connectRuntimeStartupForTest(t *testing.T, manager *Manager, sessionKey string) *ClientStartup {
	t.Helper()
	startup, err := manager.BeginClientStartup(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatalf("开始启动事务失败: %v", err)
	}
	if _, err = startup.GetOrCreateWithFactory(context.Background(), agentclient.Options{}, nil); err != nil {
		startup.Close()
		t.Fatalf("创建 runtime client 失败: %v", err)
	}
	if err = startup.Connect(context.Background()); err != nil {
		startup.Close()
		t.Fatalf("连接 runtime client 失败: %v", err)
	}
	return startup
}

func runtimeSessionStateForTest(manager *Manager, sessionKey string) *sessionState {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.sessions[sessionKey]
}

func waitRuntimeSessionClient(t *testing.T, manager *Manager, sessionKey string, want Client) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if manager.SessionClient(sessionKey) == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("session client 未变为 %#v", want)
}

func waitRuntimeStartupGateRefs(t *testing.T, manager *Manager, sessionKey string, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		manager.mu.RLock()
		gate := manager.startupGates[sessionKey]
		refs := 0
		if gate != nil {
			refs = gate.refs
		}
		manager.mu.RUnlock()
		if refs == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("startup gate refs 未达到 %d", want)
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

func waitRuntimeSessionRemoved(t *testing.T, manager *Manager, sessionKey string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		manager.mu.RLock()
		state := manager.sessions[sessionKey]
		manager.mu.RUnlock()
		if state == nil {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("client cleanup 完成后 session state 未移除")
}

func TestManagerRuntimeSwitchDoesNotFailOnStaleClientCleanup(t *testing.T) {
	staleErr := errors.New("old runtime stream failed after provider error")
	stale := &fakeRuntimeClient{disconnectErr: staleErr}
	fresh := &fakeRuntimeClient{}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{clients: []*fakeRuntimeClient{stale, fresh}})
	sessionKey := "agent:nexus:ws:dm:runtime-switch-cleanup"

	first, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{
		Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeClaude},
	})
	if err != nil {
		t.Fatalf("首次创建 Claude client 失败: %v", err)
	}
	second, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{
		Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS},
	})
	if err != nil {
		t.Fatalf("旧 runtime 清理错误不应阻断切换: %v", err)
	}
	if first != stale || second != fresh {
		t.Fatalf("runtime 切换 client 不正确: first=%#v second=%#v", first, second)
	}
	if stale.disconnectCalls != 1 {
		t.Fatalf("旧 client 应尝试关闭一次: %d", stale.disconnectCalls)
	}
	if kind := manager.RuntimeKind(sessionKey); kind != agentclient.RuntimeNXS {
		t.Fatalf("切换后 RuntimeKind() = %q, want nxs", kind)
	}
}

func TestManagerGetOrCreateReplacesClientWhenBridgeRequiresRestart(t *testing.T) {
	stale := &fakeRuntimeClient{
		reconfigureErr: &agentclient.RestartRequiredError{Reason: agentclient.RestartReasonProcessEnvChanged},
	}
	fresh := &fakeRuntimeClient{}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{clients: []*fakeRuntimeClient{stale, fresh}})
	sessionKey := "agent:nexus:ws:dm:restart-required"

	first, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{
		Env: map[string]string{"ANTHROPIC_AUTH_TOKEN": "old-token"},
	})
	if err != nil {
		t.Fatalf("首次创建 client 失败: %v", err)
	}
	second, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{
		Env: map[string]string{"ANTHROPIC_AUTH_TOKEN": "new-token"},
	})
	if err != nil {
		t.Fatalf("bridge 要求重启后应创建新 client: %v", err)
	}

	if first != stale {
		t.Fatalf("首次 client 不正确: %#v", first)
	}
	if second != fresh {
		t.Fatalf("bridge 要求重启后未替换 client: got=%#v want=%#v", second, fresh)
	}
	if stale.disconnectCalls != 1 {
		t.Fatalf("旧 client 应被关闭一次: %d", stale.disconnectCalls)
	}
}

func TestManagerGetOrCreateReplacesClientWhenBypassSwitchRequiresLaunchFlag(t *testing.T) {
	stale := &fakeRuntimeClient{
		reconfigureErr: agentclient.ErrBypassPermissionsNotAllowed,
	}
	fresh := &fakeRuntimeClient{}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{clients: []*fakeRuntimeClient{stale, fresh}})
	sessionKey := "agent:nexus:ws:dm:bypass-switch"

	first, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{
		Runtime: agentclient.RuntimeOptions{PermissionMode: sdkpermission.ModeDefault},
	})
	if err != nil {
		t.Fatalf("首次创建 client 失败: %v", err)
	}
	second, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{
		Runtime: agentclient.RuntimeOptions{
			PermissionMode:                  sdkpermission.ModeBypassPermissions,
			AllowDangerouslySkipPermissions: true,
		},
	})
	if err != nil {
		t.Fatalf("bypass 切换受限时应创建新 client: %v", err)
	}

	if first != stale {
		t.Fatalf("首次 client 不正确: %#v", first)
	}
	if second != fresh {
		t.Fatalf("bypass 切换受限后未替换 client: got=%#v want=%#v", second, fresh)
	}
	if stale.disconnectCalls != 1 {
		t.Fatalf("旧 client 应被关闭一次: %d", stale.disconnectCalls)
	}
}

func TestManagerGetOrCreateReplacesClientWhenMCPControlUnsupported(t *testing.T) {
	stale := &fakeRuntimeClient{
		reconfigureErr: &agentclient.RestartRequiredError{
			Reason: agentclient.RestartReasonMCPControlUnsupported,
			Cause:  errors.New("unsupported control request subtype: mcp_set_servers"),
		},
	}
	fresh := &fakeRuntimeClient{}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{clients: []*fakeRuntimeClient{stale, fresh}})
	sessionKey := "agent:nexus:ws:dm:mcp-control"

	first, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{})
	if err != nil {
		t.Fatalf("首次创建 client 失败: %v", err)
	}
	second, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{
		MCP: agentclient.MCPOptions{
			Servers: map[string]sdkmcp.ServerConfig{
				"search": sdkmcp.SDKServerConfig{Name: "search", Instance: fakeSDKMCPServer{}},
			},
		},
	})
	if err != nil {
		t.Fatalf("MCP 控制面不支持时应重建 client: %v", err)
	}

	if first != stale {
		t.Fatalf("首次 client 不正确: %#v", first)
	}
	if second != fresh {
		t.Fatalf("MCP 控制面不支持后未替换 client: got=%#v want=%#v", second, fresh)
	}
	if stale.disconnectCalls != 1 {
		t.Fatalf("旧 client 应被关闭一次: %d", stale.disconnectCalls)
	}
}

func TestManagerGetOrCreateKeepsNonTransportReconfigureError(t *testing.T) {
	expectedErr := errors.New("permission mode is not supported")
	stale := &fakeRuntimeClient{reconfigureErr: expectedErr}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{clients: []*fakeRuntimeClient{stale, &fakeRuntimeClient{}}})
	sessionKey := "agent:nexus:ws:dm:reconfigure-error"

	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{}); err != nil {
		t.Fatalf("首次创建 client 失败: %v", err)
	}
	_, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("非 transport 错误不应被吞掉: %v", err)
	}
	if stale.disconnectCalls != 0 {
		t.Fatalf("非 transport 错误不应关闭旧 client: %d", stale.disconnectCalls)
	}
}

func TestIsRuntimeTransportClosedError(t *testing.T) {
	cases := []error{
		agentclient.ErrNotConnected,
		io.ErrClosedPipe,
		errors.New("process: write payload failed: write |1: The pipe has been ended"),
		errors.New("write payload failed: file already closed"),
		errors.New("broken pipe"),
		errors.New("Error in hook callback hook_1: Stream closed"),
		errors.New("client: send control response failed: process: stdin unavailable"),
	}
	for _, err := range cases {
		if !IsRuntimeTransportClosedError(err) {
			t.Fatalf("应识别为 transport 断开: %v", err)
		}
	}
	if IsRuntimeTransportClosedError(errors.New("permission mode is not supported")) {
		t.Fatal("普通控制错误不应识别为 transport 断开")
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

func TestManagerSendContentWithoutRunningRound(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	_, err := manager.SendContentToRunningRound(context.Background(), "agent:nexus:ws:dm:missing", "补充信息")
	if !errors.Is(err, ErrNoRunningRound) {
		t.Fatalf("期望 ErrNoRunningRound，实际 %v", err)
	}
}

func TestManagerFlushGoalAccounting(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	sessionKey := "agent:nexus:ws:dm:test-goal-flush"
	calls := []string{}
	manager.RegisterGoalAccountingFlush(sessionKey, "round-b", func(context.Context) error {
		calls = append(calls, "round-b")
		return nil
	})
	manager.RegisterGoalAccountingFlush(sessionKey, "round-a", func(context.Context) error {
		calls = append(calls, "round-a")
		return nil
	})

	roundIDs, err := manager.FlushGoalAccounting(context.Background(), sessionKey)
	if err != nil {
		t.Fatalf("FlushGoalAccounting() error = %v", err)
	}
	if strings.Join(roundIDs, ",") != "round-a,round-b" {
		t.Fatalf("roundIDs = %#v, want sorted round-a/round-b", roundIDs)
	}
	if strings.Join(calls, ",") != "round-a,round-b" {
		t.Fatalf("calls = %#v, want sorted round-a/round-b", calls)
	}

	manager.RegisterGoalAccountingFlush(sessionKey, "round-a", nil)
	calls = nil
	roundIDs, err = manager.FlushGoalAccounting(context.Background(), sessionKey)
	if err != nil {
		t.Fatalf("FlushGoalAccounting() after unregister error = %v", err)
	}
	if strings.Join(roundIDs, ",") != "round-b" || strings.Join(calls, ",") != "round-b" {
		t.Fatalf("after unregister roundIDs=%#v calls=%#v, want only round-b", roundIDs, calls)
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

func TestManagerClearGoalAccounting(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	sessionKey := "agent:nexus:ws:dm:test-goal-clear"
	calls := []string{}
	manager.RegisterGoalAccountingClear(sessionKey, "round-b", func() {
		calls = append(calls, "round-b")
	})
	manager.RegisterGoalAccountingClear(sessionKey, "round-a", func() {
		calls = append(calls, "round-a")
	})

	roundIDs := manager.ClearGoalAccounting(sessionKey)
	if strings.Join(roundIDs, ",") != "round-a,round-b" {
		t.Fatalf("roundIDs = %#v, want sorted round-a/round-b", roundIDs)
	}
	if strings.Join(calls, ",") != "round-a,round-b" {
		t.Fatalf("calls = %#v, want sorted round-a/round-b", calls)
	}

	manager.RegisterGoalAccountingClear(sessionKey, "round-a", nil)
	calls = nil
	roundIDs = manager.ClearGoalAccounting(sessionKey)
	if strings.Join(roundIDs, ",") != "round-b" || strings.Join(calls, ",") != "round-b" {
		t.Fatalf("after unregister roundIDs=%#v calls=%#v, want only round-b", roundIDs, calls)
	}

	manager.MarkRoundFinished(sessionKey, "round-b")
	if roundIDs = manager.ClearGoalAccounting(sessionKey); len(roundIDs) != 0 {
		t.Fatalf("after round finished roundIDs=%#v, want empty", roundIDs)
	}
}

func TestManagerBeginGoalAccountingFinalizing(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	sessionKey := "agent:nexus:ws:dm:test-goal-finalize"
	calls := []string{}
	manager.RegisterGoalAccountingFinalize(sessionKey, "round-b", func() bool {
		calls = append(calls, "round-b")
		return true
	})
	manager.RegisterGoalAccountingFinalize(sessionKey, "round-a", func() bool {
		calls = append(calls, "round-a")
		return true
	})

	roundIDs := manager.BeginGoalAccountingFinalizing(sessionKey)
	if strings.Join(roundIDs, ",") != "round-a,round-b" ||
		strings.Join(calls, ",") != "round-a,round-b" {
		t.Fatalf("roundIDs=%#v calls=%#v, want sorted round-a/round-b", roundIDs, calls)
	}

	manager.RegisterGoalAccountingFinalize(sessionKey, "round-a", nil)
	calls = nil
	roundIDs = manager.BeginGoalAccountingFinalizing(sessionKey)
	if strings.Join(roundIDs, ",") != "round-b" ||
		strings.Join(calls, ",") != "round-b" {
		t.Fatalf("after unregister roundIDs=%#v calls=%#v, want only round-b", roundIDs, calls)
	}

	manager.MarkRoundFinished(sessionKey, "round-b")
	if roundIDs = manager.BeginGoalAccountingFinalizing(sessionKey); len(roundIDs) != 0 {
		t.Fatalf("after round finished roundIDs=%#v, want empty", roundIDs)
	}
}

func TestManagerActivateGoalAccounting(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	sessionKey := "agent:nexus:ws:dm:test-goal-activate"
	calls := []string{}
	manager.RegisterGoalAccountingActivate(sessionKey, "round-b", func(_ context.Context, goalID string) error {
		calls = append(calls, "round-b:"+goalID)
		return nil
	})
	manager.RegisterGoalAccountingActivate(sessionKey, "round-a", func(_ context.Context, goalID string) error {
		calls = append(calls, "round-a:"+goalID)
		return nil
	})

	roundIDs, err := manager.ActivateGoalAccounting(context.Background(), sessionKey, "goal-1")
	if err != nil {
		t.Fatalf("ActivateGoalAccounting() error = %v", err)
	}
	if strings.Join(roundIDs, ",") != "round-a,round-b" {
		t.Fatalf("roundIDs = %#v, want sorted round-a/round-b", roundIDs)
	}
	if strings.Join(calls, ",") != "round-a:goal-1,round-b:goal-1" {
		t.Fatalf("calls = %#v, want sorted round-a/round-b", calls)
	}

	manager.RegisterGoalAccountingActivate(sessionKey, "round-a", nil)
	calls = nil
	roundIDs, err = manager.ActivateGoalAccounting(context.Background(), sessionKey, "goal-2")
	if err != nil {
		t.Fatalf("ActivateGoalAccounting() after unregister error = %v", err)
	}
	if strings.Join(roundIDs, ",") != "round-b" || strings.Join(calls, ",") != "round-b:goal-2" {
		t.Fatalf("after unregister roundIDs=%#v calls=%#v, want only round-b", roundIDs, calls)
	}

	manager.MarkRoundFinished(sessionKey, "round-b")
	roundIDs, err = manager.ActivateGoalAccounting(context.Background(), sessionKey, "goal-2")
	if err != nil || len(roundIDs) != 0 {
		t.Fatalf("after round finished roundIDs=%#v err=%v, want empty nil", roundIDs, err)
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

func TestManagerGoalAccountingCreateConflictsAreScopeAwareAndLive(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	sessionKey := "agent:nexus:ws:room:test-goal-create-guard"
	roundAConsumed := false
	manager.RegisterGoalAccountingCreateGuard(sessionKey, "round-a", "root-1", func() bool {
		return roundAConsumed
	})
	manager.RegisterGoalAccountingCreateGuard(sessionKey, "round-b", "root-1", func() bool {
		return true
	})
	manager.RegisterGoalAccountingCreateGuard(sessionKey, "round-c", "root-2", func() bool {
		return true
	})

	if got := manager.GoalAccountingCreateConflicts(sessionKey, "root-1"); strings.Join(got, ",") != "round-b" {
		t.Fatalf("root-1 conflicts = %#v, want only consumed round-b", got)
	}
	if got := manager.GoalAccountingCreateConflicts(sessionKey, "root-2"); strings.Join(got, ",") != "round-c" {
		t.Fatalf("root-2 conflicts = %#v, want only consumed round-c", got)
	}
	if got := manager.GoalAccountingCreateConflicts(sessionKey, ""); strings.Join(got, ",") != "round-b,round-c" {
		t.Fatalf("session conflicts = %#v, want every consumed live scope", got)
	}

	roundAConsumed = true
	if got := manager.GoalAccountingCreateConflicts(sessionKey, "root-1"); strings.Join(got, ",") != "round-a,round-b" {
		t.Fatalf("updated root-1 conflicts = %#v, want dynamic consumed state", got)
	}

	manager.RegisterGoalAccountingCreateGuard(sessionKey, "round-b", "root-1", nil)
	manager.MarkRoundFinished(sessionKey, "round-a")
	if got := manager.GoalAccountingCreateConflicts(sessionKey, "root-1"); len(got) != 0 {
		t.Fatalf("finished/unregistered conflicts = %#v, want empty", got)
	}
	if got := manager.GoalAccountingCreateConflicts(sessionKey, ""); strings.Join(got, ",") != "round-c" {
		t.Fatalf("remaining session conflicts = %#v, want only round-c", got)
	}
}

func TestManagerGuidanceHookInjectsPostToolUseAdditionalContext(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	sessionKey := "agent:nexus:ws:dm:test-guide"
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{}); err != nil {
		t.Fatalf("创建 client 失败: %v", err)
	}
	_ = manager.StartRound(context.Background(), sessionKey, "round-guide", func() {})

	roundIDs, err := manager.QueueGuidanceInput(context.Background(), sessionKey, "round-guide-msg", "请优先检查日志")
	if err != nil {
		t.Fatalf("登记引导输入失败: %v", err)
	}
	if len(roundIDs) != 1 || roundIDs[0] != "round-guide" {
		t.Fatalf("返回运行中 round 不正确: %+v", roundIDs)
	}
	if count := manager.PendingGuidanceCount(sessionKey); count != 1 {
		t.Fatalf("PendingGuidanceCount = %d, want 1", count)
	}

	options := manager.WithGuidanceHook(agentclient.Options{}, sessionKey)
	matchers := options.Hooks.Matchers[sdkhook.EventPostToolUse]
	if len(matchers) != 1 || len(matchers[0].Hooks) != 1 {
		t.Fatalf("PostToolUse hook 未注册: %+v", matchers)
	}
	output, err := matchers[0].Hooks[0](context.Background(), sdkhook.Input{
		EventName: sdkhook.EventPostToolUse,
	}, "tool-1")
	if err != nil {
		t.Fatalf("执行 PostToolUse hook 失败: %v", err)
	}
	additionalContext := output.SpecificOutput.AdditionalContext
	if !strings.Contains(additionalContext, "请优先检查日志") || !strings.Contains(additionalContext, "round-guide-msg") {
		t.Fatalf("additionalContext 未包含引导内容: %q", additionalContext)
	}
	if count := manager.PendingGuidanceCount(sessionKey); count != 0 {
		t.Fatalf("PendingGuidanceCount = %d, want 0", count)
	}
}

func TestManagerGuidanceHookInjectsContextualAdditionalContext(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	sessionKey := "agent:nexus:ws:dm:test-goal-guide"
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{}); err != nil {
		t.Fatalf("创建 client 失败: %v", err)
	}
	_ = manager.StartRound(context.Background(), sessionKey, "round-guide", func() {})

	if _, err := manager.QueueContextualGuidanceInput(context.Background(), sessionKey, "goal-event-1", "goal", "Budget reached."); err != nil {
		t.Fatalf("登记 Goal 上下文失败: %v", err)
	}

	options := manager.WithGuidanceHook(agentclient.Options{}, sessionKey)
	output, err := options.Hooks.Matchers[sdkhook.EventPostToolUse][0].Hooks[0](
		context.Background(),
		sdkhook.Input{EventName: sdkhook.EventPostToolUse},
		"tool-1",
	)
	if err != nil {
		t.Fatalf("执行 PostToolUse hook 失败: %v", err)
	}
	additionalContext := output.SpecificOutput.AdditionalContext
	if !strings.Contains(additionalContext, "<internal_context source=\"goal\">\nBudget reached.\n</internal_context>") {
		t.Fatalf("additionalContext 未包含 Goal context: %q", additionalContext)
	}
	if strings.Contains(additionalContext, "<nexus_guidance>") {
		t.Fatalf("Goal context 不应包在 nexus_guidance 中: %q", additionalContext)
	}
}

func TestManagerContextualGuidanceRunsConsumedCallbackOnlyAtPostToolUse(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	sessionKey := "agent:nexus:ws:group:goal-retarget"
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{}); err != nil {
		t.Fatal(err)
	}
	_ = manager.StartRound(context.Background(), sessionKey, "round-recipient", func() {})
	consumed := false
	if _, err := manager.QueueContextualGuidanceInputOnConsumed(
		context.Background(),
		sessionKey,
		"goal-event-retarget",
		"goal",
		"The objective changed.",
		func() { consumed = true },
	); err != nil {
		t.Fatal(err)
	}
	if consumed {
		t.Fatal("callback ran while guidance was only queued")
	}

	options := manager.WithGuidanceHook(agentclient.Options{}, sessionKey)
	output, err := options.Hooks.Matchers[sdkhook.EventPostToolUse][0].Hooks[0](
		context.Background(),
		sdkhook.Input{EventName: sdkhook.EventPostToolUse},
		"tool-before-retarget",
	)
	if err != nil {
		t.Fatal(err)
	}
	if output.SpecificOutput == nil || !strings.Contains(output.SpecificOutput.AdditionalContext, "The objective changed.") {
		t.Fatalf("output = %#v, want retarget context", output)
	}
	if !consumed {
		t.Fatal("callback did not run when PostToolUse consumed guidance")
	}
}

func TestManagerContextualGuidanceWaitsForRuntimeAppliedAck(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{hookResponseAck: true}})
	sessionKey := "agent:nexus:ws:group:goal-retarget-ack"
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{}); err != nil {
		t.Fatal(err)
	}
	_ = manager.StartRound(context.Background(), sessionKey, "round-recipient", func() {})
	consumed := false
	if _, err := manager.QueueContextualGuidanceInputOnConsumed(
		context.Background(), sessionKey, "goal-event-retarget", "goal", "The objective changed.", func() { consumed = true },
	); err != nil {
		t.Fatal(err)
	}

	options := manager.WithGuidanceHook(agentclient.Options{}, sessionKey)
	output, err := options.Hooks.Matchers[sdkhook.EventPostToolUse][0].Hooks[0](
		context.Background(), sdkhook.Input{EventName: sdkhook.EventPostToolUse}, "tool-before-retarget",
	)
	if err != nil {
		t.Fatal(err)
	}
	if consumed || output.OnApplied == nil {
		t.Fatalf("consumed=%v OnApplied=%v, want callback deferred until applied ACK", consumed, output.OnApplied != nil)
	}
	output.OnApplied(sdkhook.AppliedAck{RequestID: "hook-request-1"})
	if !consumed {
		t.Fatal("callback did not run after runtime applied ACK")
	}
}

func TestManagerCloseIdleSessionsClosesOnlyIdleClients(t *testing.T) {
	now := time.Date(2026, 6, 2, 15, 0, 0, 0, time.UTC)
	idleClient := &fakeRuntimeClient{}
	activeClient := &fakeRuntimeClient{}
	recentClient := &fakeRuntimeClient{}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{clients: []*fakeRuntimeClient{
		idleClient,
		activeClient,
		recentClient,
	}})
	manager.now = func() time.Time { return now }

	idleKey := "agent:nexus:ws:dm:idle"
	activeKey := "agent:nexus:ws:dm:active"
	recentKey := "agent:nexus:ws:dm:recent"
	if _, err := manager.GetOrCreate(context.Background(), idleKey, agentclient.Options{}); err != nil {
		t.Fatalf("创建 idle client 失败: %v", err)
	}
	if _, err := manager.GetOrCreate(context.Background(), activeKey, agentclient.Options{}); err != nil {
		t.Fatalf("创建 active client 失败: %v", err)
	}
	if _, err := manager.GetOrCreate(context.Background(), recentKey, agentclient.Options{}); err != nil {
		t.Fatalf("创建 recent client 失败: %v", err)
	}
	_ = manager.StartRound(context.Background(), activeKey, "round-active", nil)

	manager.mu.Lock()
	manager.sessions[idleKey].LastUsedAt = now.Add(-20 * time.Minute)
	manager.sessions[activeKey].LastUsedAt = now.Add(-20 * time.Minute)
	manager.sessions[recentKey].LastUsedAt = now.Add(-2 * time.Minute)
	manager.mu.Unlock()

	closed, err := manager.CloseIdleSessions(context.Background(), 10*time.Minute)
	if err != nil {
		t.Fatalf("回收空闲 session 失败: %v", err)
	}
	if closed != 1 {
		t.Fatalf("回收数量 = %d, want 1", closed)
	}
	if idleClient.disconnectCalls != 1 {
		t.Fatalf("idle client 应关闭一次: %d", idleClient.disconnectCalls)
	}
	if activeClient.disconnectCalls != 0 {
		t.Fatalf("active client 不应关闭: %d", activeClient.disconnectCalls)
	}
	if recentClient.disconnectCalls != 0 {
		t.Fatalf("recent client 不应关闭: %d", recentClient.disconnectCalls)
	}
	if got := manager.GetRunningRoundIDs(activeKey); len(got) != 1 || got[0] != "round-active" {
		t.Fatalf("active round 不应被清理: %+v", got)
	}
}

func TestManagerCloseOwnerSessionsClosesOnlyMatchingOwner(t *testing.T) {
	clientA1 := &fakeRuntimeClient{}
	clientA2 := &fakeRuntimeClient{}
	clientB := &fakeRuntimeClient{}
	reaper := &fakeOwnerProcessReaper{}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{clients: []*fakeRuntimeClient{
		clientA1,
		clientA2,
		clientB,
	}})
	manager.SetOwnerProcessReaper(reaper)
	optionsForOwner := func(ownerUserID string) agentclient.Options {
		return agentclient.Options{
			Env: map[string]string{"NEXUS_RUNTIME_USER_ID": ownerUserID},
		}
	}
	for _, input := range []struct {
		sessionKey  string
		ownerUserID string
	}{
		{sessionKey: "session-a-1", ownerUserID: "owner-a"},
		{sessionKey: "session-a-2", ownerUserID: "owner-a"},
		{sessionKey: "session-b", ownerUserID: "owner-b"},
	} {
		if _, err := manager.GetOrCreate(
			context.Background(),
			input.sessionKey,
			optionsForOwner(input.ownerUserID),
		); err != nil {
			t.Fatalf("创建 %s runtime 失败: %v", input.sessionKey, err)
		}
	}
	roundCanceled := false
	_ = manager.StartRound(context.Background(), "session-a-1", "round-a", func() {
		roundCanceled = true
		manager.MarkRoundFinished("session-a-1", "round-a")
	})

	closed, err := manager.CloseOwnerSessions(context.Background(), "owner-a")
	if err != nil {
		t.Fatalf("关闭 owner runtime 失败: %v", err)
	}
	if closed != 2 {
		t.Fatalf("关闭数量=%d，want 2", closed)
	}
	if !roundCanceled {
		t.Fatal("owner runtime 回收必须取消运行中的 round")
	}
	if clientA1.disconnectCalls != 1 || clientA2.disconnectCalls != 1 {
		t.Fatalf(
			"owner-a clients 未全部关闭: first=%d second=%d",
			clientA1.disconnectCalls,
			clientA2.disconnectCalls,
		)
	}
	if clientB.disconnectCalls != 0 || !manager.HasSession("session-b") {
		t.Fatalf(
			"owner-b runtime 不应受影响: disconnect=%d exists=%v",
			clientB.disconnectCalls,
			manager.HasSession("session-b"),
		)
	}
	if calls := reaper.ownerCalls(); !slices.Equal(calls, []string{"owner-a"}) {
		t.Fatalf("owner cgroup 回收调用=%v，want [owner-a]", calls)
	}
}

func TestManagerCloseOwnerSessionsReapsOwnerWithoutTrackedSession(t *testing.T) {
	reaper := &fakeOwnerProcessReaper{}
	manager := NewManager()
	manager.SetOwnerProcessReaper(reaper)

	closed, err := manager.CloseOwnerSessions(context.Background(), "owner-orphan")
	if err != nil {
		t.Fatalf("回收 orphan owner 失败: %v", err)
	}
	if closed != 0 {
		t.Fatalf("没有 tracked session 时 closed=%d，want 0", closed)
	}
	if calls := reaper.ownerCalls(); !slices.Equal(calls, []string{"owner-orphan"}) {
		t.Fatalf("owner cgroup 回收调用=%v，want [owner-orphan]", calls)
	}
}

func TestManagerOwnerReaperDoesNotBlockOtherOwners(t *testing.T) {
	reaperStarted := make(chan string, 1)
	releaseReaper := make(chan struct{})
	reaper := &fakeOwnerProcessReaper{
		started: reaperStarted,
		release: releaseReaper,
	}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	manager.SetOwnerProcessReaper(reaper)
	ownerOptions := agentclient.Options{
		Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner-a"},
	}
	if _, err := manager.GetOrCreate(context.Background(), "session-a", ownerOptions); err != nil {
		t.Fatalf("创建 owner-a runtime 失败: %v", err)
	}

	closeResult := make(chan error, 1)
	go func() {
		_, err := manager.CloseOwnerSessions(context.Background(), "owner-a")
		closeResult <- err
	}()
	if owner := <-reaperStarted; owner != "owner-a" {
		t.Fatalf("reaper owner = %q, want owner-a", owner)
	}

	if _, err := manager.BeginClientStartup(context.Background(), "session-a-new", "owner-a"); !errors.Is(err, ErrRuntimeSessionClosing) {
		t.Fatalf("owner-a fence 期间 BeginClientStartup() error = %v", err)
	}
	ownerBStartup := make(chan *ClientStartup, 1)
	ownerBError := make(chan error, 1)
	go func() {
		startup, err := manager.BeginClientStartup(context.Background(), "session-b", "owner-b")
		ownerBStartup <- startup
		ownerBError <- err
	}()
	select {
	case err := <-ownerBError:
		startup := <-ownerBStartup
		if err != nil {
			t.Fatalf("owner-b startup 被 owner-a reaper 阻塞: %v", err)
		}
		startup.Close()
	case <-time.After(time.Second):
		t.Fatal("owner-a reaper 持有了全局 Manager 锁")
	}
	if manager.StartBackgroundTaskForOwner("session-a-background", "owner-a", func(context.Context) {}) {
		t.Fatal("owner-a fence 期间不应登记后台任务")
	}
	ownerBTaskDone := make(chan struct{})
	if !manager.StartBackgroundTaskForOwner("session-b-background", "owner-b", func(context.Context) {
		close(ownerBTaskDone)
	}) {
		t.Fatal("owner-b 后台任务不应受 owner-a fence 影响")
	}
	<-ownerBTaskDone

	close(releaseReaper)
	if err := <-closeResult; err != nil {
		t.Fatalf("CloseOwnerSessions() error = %v", err)
	}
	startup, err := manager.BeginClientStartup(context.Background(), "session-a-new", "owner-a")
	if err != nil {
		t.Fatalf("reaper 完成后 owner-a startup 仍被拒绝: %v", err)
	}
	startup.Close()
}

func TestManagerOwnerReaperWaitsForStaleStartup(t *testing.T) {
	reaperStarted := make(chan string, 1)
	releaseReaper := make(chan struct{})
	reaper := &fakeOwnerProcessReaper{
		started: reaperStarted,
		release: releaseReaper,
	}
	factory := &fakeRuntimeFactory{clients: []*fakeRuntimeClient{{}}}
	manager := NewManagerWithFactory(factory)
	manager.SetOwnerProcessReaper(reaper)
	startup, err := manager.BeginClientStartup(context.Background(), "session-a", "owner-a")
	if err != nil {
		t.Fatalf("BeginClientStartup() error = %v", err)
	}

	closeResult := make(chan error, 1)
	go func() {
		_, closeErr := manager.CloseOwnerSessions(context.Background(), "owner-a")
		closeResult <- closeErr
	}()
	waitOwnerReapFence(t, manager, "owner-a")
	select {
	case owner := <-reaperStarted:
		t.Fatalf("旧 startup 退出前 reaper 提前执行: %s", owner)
	default:
	}
	options := agentclient.Options{Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner-a"}}
	if client, createErr := startup.GetOrCreateWithFactory(context.Background(), options, factory); !errors.Is(createErr, agentclient.ErrAborted) || client != nil {
		t.Fatalf("旧 startup 未失效: client=%#v err=%v", client, createErr)
	}
	startup.Close()
	if owner := <-reaperStarted; owner != "owner-a" {
		t.Fatalf("reaper owner = %q, want owner-a", owner)
	}
	if factory.index != 0 {
		t.Fatalf("失效 startup 调用了 factory: %d", factory.index)
	}
	close(releaseReaper)
	if err := <-closeResult; err != nil {
		t.Fatalf("CloseOwnerSessions() error = %v", err)
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

func TestManagerOwnerReaperCancelsInflightReconfigure(t *testing.T) {
	reconfigureStarted := make(chan struct{})
	client := &agentClient{
		session: &agentclient.Session{},
		reconfigureSession: func(ctx context.Context, _ *agentclient.Session, _ agentclient.Options) error {
			close(reconfigureStarted)
			<-ctx.Done()
			return ctx.Err()
		},
		closeSession: func(*agentclient.Session) error { return nil },
	}
	reaperStarted := make(chan string, 1)
	manager := NewManager()
	manager.SetOwnerProcessReaper(&fakeOwnerProcessReaper{started: reaperStarted})
	const sessionKey = "session-a-reconfigure"
	const ownerUserID = "owner-a"
	ownerOptions := agentclient.Options{
		Env: map[string]string{"NEXUS_RUNTIME_USER_ID": ownerUserID},
	}
	manager.mu.Lock()
	state := manager.ensureStateLocked(sessionKey)
	state.Client = client
	state.OwnerUserID = ownerUserID
	state.RuntimeKind = agentclient.RuntimeNXS
	manager.mu.Unlock()

	startup, err := manager.BeginClientStartup(context.Background(), sessionKey, ownerUserID)
	if err != nil {
		t.Fatalf("BeginClientStartup() error = %v", err)
	}
	reconfigureResult := make(chan error, 1)
	go func() {
		defer startup.Close()
		_, configureErr := startup.GetOrCreateWithFactory(context.Background(), ownerOptions, nil)
		reconfigureResult <- configureErr
	}()
	<-reconfigureStarted

	closeResult := make(chan error, 1)
	go func() {
		_, closeErr := manager.CloseOwnerSessions(context.Background(), ownerUserID)
		closeResult <- closeErr
	}()
	select {
	case err := <-reconfigureResult:
		if !errors.Is(err, agentclient.ErrAborted) {
			t.Fatalf("Reconfigure() error = %v, want ErrAborted", err)
		}
	case <-time.After(time.Second):
		t.Fatal("owner Retire 未取消 in-flight Reconfigure")
	}
	select {
	case owner := <-reaperStarted:
		if owner != ownerUserID {
			t.Fatalf("reaper owner = %q, want %q", owner, ownerUserID)
		}
	case <-time.After(time.Second):
		t.Fatal("startup 退出后 owner reaper 未启动")
	}
	if err := <-closeResult; err != nil {
		t.Fatalf("CloseOwnerSessions() error = %v", err)
	}
}

func TestManagerOwnerReaperWaitsForAllClosingSessions(t *testing.T) {
	firstDisconnectStarted := make(chan struct{}, 1)
	releaseFirstDisconnect := make(chan struct{})
	firstClient := &ownershipFenceClient{
		disconnectStarted: firstDisconnectStarted,
		disconnectRelease: releaseFirstDisconnect,
	}
	secondClient := &ownershipFenceClient{}
	reaperStarted := make(chan string, 1)
	reaper := &fakeOwnerProcessReaper{started: reaperStarted}
	manager := NewManagerWithFactory(&runtimeClientSequenceFactory{clients: []Client{
		firstClient,
		secondClient,
	}})
	manager.SetOwnerProcessReaper(reaper)
	ownerOptions := agentclient.Options{
		Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner-a"},
	}
	const firstSessionKey = "session-a-first-closing"
	const secondSessionKey = "session-a-last-closing"
	for _, sessionKey := range []string{firstSessionKey, secondSessionKey} {
		if _, err := manager.GetOrCreate(context.Background(), sessionKey, ownerOptions); err != nil {
			t.Fatalf("创建 %s runtime 失败: %v", sessionKey, err)
		}
	}

	firstCloseResult := make(chan error, 1)
	go func() {
		firstCloseResult <- manager.CloseSession(context.Background(), firstSessionKey)
	}()
	<-firstDisconnectStarted

	secondCloseResult := make(chan error, 1)
	go func() {
		secondCloseResult <- manager.CloseSession(context.Background(), secondSessionKey)
	}()
	if owner := <-reaperStarted; owner != "owner-a" {
		t.Fatalf("reaper owner = %q, want owner-a", owner)
	}
	waitRuntimeSessionRemoved(t, manager, secondSessionKey)
	if _, err := manager.BeginClientStartup(
		context.Background(),
		"session-a-after-close",
		"owner-a",
	); !errors.Is(err, ErrRuntimeSessionClosing) {
		t.Fatalf("首个 closing session 未退出时 owner fence 提前释放: %v", err)
	}
	select {
	case err := <-secondCloseResult:
		t.Fatalf("首个 closing session 未退出时最后关闭提前返回: %v", err)
	default:
	}

	close(releaseFirstDisconnect)
	if err := <-firstCloseResult; err != nil {
		t.Fatalf("首次 CloseSession() error = %v", err)
	}
	if err := <-secondCloseResult; err != nil {
		t.Fatalf("末次 CloseSession() error = %v", err)
	}
	startup, err := manager.BeginClientStartup(
		context.Background(),
		"session-a-after-close",
		"owner-a",
	)
	if err != nil {
		t.Fatalf("全部 closing session 退出后 startup 仍被拒绝: %v", err)
	}
	startup.Close()
}

func TestManagerOwnerReaperDrainsStartupForClosingSession(t *testing.T) {
	reaperStarted := make(chan string, 1)
	reaper := &fakeOwnerProcessReaper{started: reaperStarted}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	manager.SetOwnerProcessReaper(reaper)
	const sessionKey = "session-a-closing-startup"
	ownerOptions := agentclient.Options{
		Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner-a"},
	}
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, ownerOptions); err != nil {
		t.Fatalf("创建 owner runtime 失败: %v", err)
	}
	startup, err := manager.BeginClientStartup(context.Background(), sessionKey, "owner-a")
	if err != nil {
		t.Fatalf("BeginClientStartup() error = %v", err)
	}

	closeResult := make(chan error, 1)
	go func() {
		closeResult <- manager.CloseSession(context.Background(), sessionKey)
	}()
	waitOwnerReapFence(t, manager, "owner-a")
	select {
	case owner := <-reaperStarted:
		startup.Close()
		t.Fatalf("同 session 旧 startup drain 前 reaper 提前执行: %s", owner)
	default:
	}
	if client, createErr := startup.GetOrCreateWithFactory(
		context.Background(),
		ownerOptions,
		nil,
	); !errors.Is(createErr, agentclient.ErrAborted) || client != nil {
		startup.Close()
		t.Fatalf("closing session 的旧 startup 未失效: client=%#v err=%v", client, createErr)
	}
	startup.Close()
	if owner := <-reaperStarted; owner != "owner-a" {
		t.Fatalf("reaper owner = %q, want owner-a", owner)
	}
	if err := <-closeResult; err != nil {
		t.Fatalf("CloseSession() error = %v", err)
	}
}

func TestManagerOwnerReaperCloseCurrentDoesNotWaitForCallingStartup(t *testing.T) {
	reaper := &fakeOwnerProcessReaper{}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	manager.SetOwnerProcessReaper(reaper)
	const sessionKey = "session-a-close-current"
	ownerOptions := agentclient.Options{
		Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner-a"},
	}
	startup, err := manager.BeginClientStartup(context.Background(), sessionKey, "owner-a")
	if err != nil {
		t.Fatalf("BeginClientStartup() error = %v", err)
	}
	defer startup.Close()
	if _, err = startup.GetOrCreateWithFactory(context.Background(), ownerOptions, nil); err != nil {
		t.Fatalf("GetOrCreateWithFactory() error = %v", err)
	}
	closed, err := startup.CloseCurrent(context.Background())
	if err != nil || !closed {
		t.Fatalf("CloseCurrent() closed=%v err=%v", closed, err)
	}
	if calls := reaper.ownerCalls(); !slices.Equal(calls, []string{"owner-a"}) {
		t.Fatalf("owner cgroup 回收调用=%v，want [owner-a]", calls)
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

func TestManagerCloseIdleSessionsCancelsSubagentMessageDrain(t *testing.T) {
	now := time.Date(2026, 7, 10, 15, 0, 0, 0, time.UTC)
	messages := make(chan sdkprotocol.ReceivedMessage)
	client := &fakeRuntimeClient{
		messages:       messages,
		receiveStarted: make(chan struct{}, 1),
		receiveStopped: make(chan struct{}, 1),
	}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: client})
	manager.now = func() time.Time { return now }
	sessionKey := "agent:nexus:ws:dm:idle-subagent-drain"
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{}); err != nil {
		t.Fatalf("创建 runtime client 失败: %v", err)
	}
	manager.MarkSubagentHistory(sessionKey)
	manager.StartIdleMessageDrain(sessionKey, func(context.Context, sdkprotocol.ReceivedMessage) bool { return true })
	select {
	case <-client.receiveStarted:
	case <-time.After(time.Second):
		t.Fatal("idle message drain 未启动")
	}

	now = now.Add(11 * time.Minute)
	closed, err := manager.CloseIdleSessions(context.Background(), 10*time.Minute)
	if err != nil || closed != 1 {
		t.Fatalf("CloseIdleSessions() closed=%d err=%v", closed, err)
	}
	select {
	case <-client.receiveStopped:
	case <-time.After(time.Second):
		t.Fatal("idle reaper 未取消 subagent message drain")
	}
}

func TestManagerCloseIdleSessionsCountsIdleFromRoundFinish(t *testing.T) {
	now := time.Date(2026, 6, 2, 15, 0, 0, 0, time.UTC)
	client := &fakeRuntimeClient{}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: client})
	manager.now = func() time.Time { return now }
	sessionKey := "agent:nexus:ws:dm:finish-idle"
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{}); err != nil {
		t.Fatalf("创建 client 失败: %v", err)
	}
	_ = manager.StartRound(context.Background(), sessionKey, "round-finish", nil)

	now = now.Add(20 * time.Minute)
	manager.MarkRoundFinished(sessionKey, "round-finish")
	closed, err := manager.CloseIdleSessions(context.Background(), 10*time.Minute)
	if err != nil {
		t.Fatalf("回收空闲 session 失败: %v", err)
	}
	if closed != 0 {
		t.Fatalf("round 刚结束不应立即回收: %d", closed)
	}

	now = now.Add(11 * time.Minute)
	closed, err = manager.CloseIdleSessions(context.Background(), 10*time.Minute)
	if err != nil {
		t.Fatalf("第二次回收空闲 session 失败: %v", err)
	}
	if closed != 1 {
		t.Fatalf("超过结束后 TTL 应回收: %d", closed)
	}
	if client.disconnectCalls != 1 {
		t.Fatalf("client 应关闭一次: %d", client.disconnectCalls)
	}
}

func TestAgentClientDisconnectDeadlineDoesNotCancelSharedCleanup(t *testing.T) {
	cleanupStarted := make(chan struct{})
	cleanupRelease := make(chan struct{})
	newSessionCalled := make(chan struct{}, 1)
	var cleanupGate sync.Once
	client := &agentClient{
		session: &agentclient.Session{},
		newSession: func(context.Context, agentclient.Options) (*agentclient.Session, error) {
			newSessionCalled <- struct{}{}
			return &agentclient.Session{}, nil
		},
		closeSession: func(*agentclient.Session) error {
			shouldWait := false
			cleanupGate.Do(func() { shouldWait = true })
			if !shouldWait {
				return nil
			}
			close(cleanupStarted)
			<-cleanupRelease
			return nil
		},
	}
	disconnectCtx, cancelDisconnect := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelDisconnect()
	if err := client.Disconnect(disconnectCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Disconnect 等待应遵守调用方 deadline: %v", err)
	}
	select {
	case <-cleanupStarted:
	default:
		t.Fatal("Disconnect 应启动共享 cleanup")
	}

	connectCtx, cancelConnect := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelConnect()
	if err := client.Connect(connectCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cleanup 未完成时 Connect 应等待同一 fence: %v", err)
	}
	select {
	case <-newSessionCalled:
		t.Fatal("调用方 Disconnect 超时不代表旧 runtime 已回收")
	default:
	}

	close(cleanupRelease)
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("共享 cleanup 完成后应允许重连: %v", err)
	}
	select {
	case <-newSessionCalled:
	case <-time.After(time.Second):
		t.Fatal("共享 cleanup 完成后未启动新 runtime")
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

func TestAgentClientConcurrentConnectWaiterHonorsContext(t *testing.T) {
	connectStarted := make(chan struct{})
	connectRelease := make(chan struct{})
	client := &agentClient{
		newSession: func(context.Context, agentclient.Options) (*agentclient.Session, error) {
			close(connectStarted)
			<-connectRelease
			return &agentclient.Session{}, nil
		},
		closeSession: func(*agentclient.Session) error { return nil },
	}
	ownerDone := make(chan error, 1)
	go func() { ownerDone <- client.Connect(context.Background()) }()
	select {
	case <-connectStarted:
	case <-time.After(time.Second):
		t.Fatal("owner Connect 未进入 session 启动阶段")
	}

	waiterCtx, cancelWaiter := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelWaiter()
	if err := client.Connect(waiterCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("并发 Connect waiter 应遵守自己的 context: %v", err)
	}
	close(connectRelease)
	select {
	case err := <-ownerDone:
		if err != nil {
			t.Fatalf("owner Connect 失败: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("owner Connect 未结束")
	}
}

func TestAgentClientConcurrentConnectSharesStableFailure(t *testing.T) {
	const callerCount = 8
	startErr := errors.New("runtime executable is unavailable")
	firstStarted := make(chan struct{})
	firstRelease := make(chan struct{})
	var attemptsMu sync.Mutex
	attempts := 0
	client := &agentClient{
		newSession: func(context.Context, agentclient.Options) (*agentclient.Session, error) {
			attemptsMu.Lock()
			attempts++
			attempt := attempts
			attemptsMu.Unlock()
			if attempt == 1 {
				close(firstStarted)
				<-firstRelease
			}
			return nil, startErr
		},
	}

	results := make(chan error, callerCount)
	go func() { results <- client.Connect(context.Background()) }()
	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("首次 Connect 未启动 runtime")
	}
	for index := 1; index < callerCount; index++ {
		waiterCtx := newObservedDoneContext(context.Background())
		go func() { results <- client.Connect(waiterCtx) }()
		select {
		case <-waiterCtx.observed:
		case <-time.After(time.Second):
			t.Fatalf("waiter %d 未加入首次 Connect flight", index)
		}
	}
	close(firstRelease)
	for index := 0; index < callerCount; index++ {
		select {
		case err := <-results:
			if !errors.Is(err, startErr) {
				t.Fatalf("caller %d 错误=%v", index, err)
			}
		case <-time.After(time.Second):
			t.Fatalf("caller %d 未收到共享启动错误", index)
		}
	}
	attemptsMu.Lock()
	sharedAttempts := attempts
	attemptsMu.Unlock()
	if sharedAttempts != 1 {
		t.Fatalf("同一并发 cohort 应只启动一次 runtime，实际=%d", sharedAttempts)
	}

	if err := client.Connect(context.Background()); !errors.Is(err, startErr) {
		t.Fatalf("后续独立 Connect 应重新尝试并返回启动错误: %v", err)
	}
	attemptsMu.Lock()
	finalAttempts := attempts
	attemptsMu.Unlock()
	if finalAttempts != 2 {
		t.Fatalf("共享错误不应永久缓存，最终启动次数=%d", finalAttempts)
	}
}

func TestAgentClientConnectOwnerCancellationDoesNotPoisonWaiter(t *testing.T) {
	startErr := errors.New("runtime startup failed after cancellation")
	firstOpenStarted := make(chan struct{})
	firstOpenRelease := make(chan struct{})
	var attemptsMu sync.Mutex
	attempts := 0
	client := &agentClient{
		newSession: func(context.Context, agentclient.Options) (*agentclient.Session, error) {
			attemptsMu.Lock()
			attempts++
			attempt := attempts
			attemptsMu.Unlock()
			if attempt == 1 {
				close(firstOpenStarted)
				<-firstOpenRelease
				return nil, startErr
			}
			return &agentclient.Session{}, nil
		},
		closeSession: func(*agentclient.Session) error { return nil },
	}
	ownerCtx, cancelOwner := context.WithCancel(context.Background())
	ownerDone := make(chan error, 1)
	go func() { ownerDone <- client.Connect(ownerCtx) }()
	select {
	case <-firstOpenStarted:
	case <-time.After(time.Second):
		t.Fatal("Connect owner 未启动 runtime")
	}
	waiterCtx := newObservedDoneContext(context.Background())
	waiterDone := make(chan error, 1)
	go func() { waiterDone <- client.Connect(waiterCtx) }()
	select {
	case <-waiterCtx.observed:
	case <-time.After(time.Second):
		t.Fatal("健康 waiter 未加入 owner Connect flight")
	}
	cancelOwner()
	close(firstOpenRelease)
	select {
	case err := <-ownerDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("owner Connect 应优先返回自己的取消错误而不是启动错误: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("owner Connect 未按取消退出")
	}
	select {
	case err := <-waiterDone:
		if err != nil {
			t.Fatalf("健康 waiter 不应继承 owner context 错误: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("健康 waiter 未重试 runtime")
	}
	attemptsMu.Lock()
	openAttempts := attempts
	attemptsMu.Unlock()
	if openAttempts != 2 {
		t.Fatalf("owner 取消后 waiter 应独立重试一次，实际=%d", openAttempts)
	}
}

func TestAgentClientConnectFailureDoesNotCrossLifecycle(t *testing.T) {
	startErr := errors.New("stale runtime failed to initialize")
	firstStarted := make(chan struct{})
	firstRelease := make(chan struct{})
	var attemptsMu sync.Mutex
	attempts := 0
	client := &agentClient{
		newSession: func(context.Context, agentclient.Options) (*agentclient.Session, error) {
			attemptsMu.Lock()
			attempts++
			attempt := attempts
			attemptsMu.Unlock()
			if attempt == 1 {
				close(firstStarted)
				<-firstRelease
				return nil, startErr
			}
			return &agentclient.Session{}, nil
		},
		closeSession: func(*agentclient.Session) error { return nil },
	}
	ownerDone := make(chan error, 1)
	go func() { ownerDone <- client.Connect(context.Background()) }()
	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("旧 lifecycle Connect 未启动")
	}

	client.DiscardUncleanSession()
	waiterCtx := newObservedDoneContext(context.Background())
	waiterDone := make(chan error, 1)
	go func() { waiterDone <- client.Connect(waiterCtx) }()
	select {
	case <-waiterCtx.observed:
	case <-time.After(time.Second):
		t.Fatal("新 lifecycle Connect 未等待旧 flight 退出")
	}
	close(firstRelease)
	select {
	case err := <-ownerDone:
		if !errors.Is(err, agentclient.ErrAborted) {
			t.Fatalf("旧 lifecycle Connect 错误=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("旧 lifecycle Connect 未退出")
	}
	select {
	case err := <-waiterDone:
		if err != nil {
			t.Fatalf("新 lifecycle 不应继承旧启动失败: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("新 lifecycle Connect 未独立重试")
	}
	attemptsMu.Lock()
	openAttempts := attempts
	attemptsMu.Unlock()
	if openAttempts != 2 {
		t.Fatalf("换代后应启动一次新 runtime，实际=%d", openAttempts)
	}
}

func TestAgentClientDisconnectDuringConfigRetryCannotReviveSession(t *testing.T) {
	firstOpenRelease := make(chan struct{})
	staleCloseStarted := make(chan struct{})
	staleCloseRelease := make(chan struct{})
	attempts := make(chan agentclient.Options, 2)
	client := &agentClient{
		options: agentclient.Options{Model: "old-model"},
		newSession: func(_ context.Context, options agentclient.Options) (*agentclient.Session, error) {
			attempts <- options
			if options.Model == "old-model" {
				<-firstOpenRelease
			}
			return &agentclient.Session{}, nil
		},
		closeSession: func(*agentclient.Session) error {
			close(staleCloseStarted)
			<-staleCloseRelease
			return nil
		},
	}
	connectDone := make(chan error, 1)
	go func() { connectDone <- client.Connect(context.Background()) }()
	select {
	case <-attempts:
	case <-time.After(time.Second):
		t.Fatal("首次 Connect 未启动")
	}
	if err := client.Reconfigure(context.Background(), agentclient.Options{Model: "new-model"}); err != nil {
		t.Fatalf("连接期间 Reconfigure 失败: %v", err)
	}
	close(firstOpenRelease)
	select {
	case <-staleCloseStarted:
	case <-time.After(time.Second):
		t.Fatal("过期配置创建的 session 未进入关闭阶段")
	}

	client.DiscardUncleanSession()
	close(staleCloseRelease)
	select {
	case err := <-connectDone:
		if !errors.Is(err, agentclient.ErrAborted) {
			t.Fatalf("生命周期失效后的配置重试错误=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("生命周期失效后的 Connect 未退出")
	}
	select {
	case options := <-attempts:
		t.Fatalf("旧 Connect 不应采纳新 lifecycle 再次启动: %+v", options)
	default:
	}
	if client.IsConnected() {
		t.Fatal("生命周期失效后的 Connect 不应安装 session")
	}
}

func TestAgentClientDisconnectReturnsCleanupErrorAndAllowsReconnect(t *testing.T) {
	closeErr := errors.New("runtime close failed")
	newSessionCalled := false
	client := &agentClient{
		session: &agentclient.Session{},
		newSession: func(context.Context, agentclient.Options) (*agentclient.Session, error) {
			newSessionCalled = true
			return &agentclient.Session{}, nil
		},
		closeSession: func(*agentclient.Session) error {
			return closeErr
		},
	}
	if err := client.Disconnect(context.Background()); !errors.Is(err, closeErr) {
		t.Fatalf("Disconnect 应保留底层 close 错误: %v", err)
	}
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("旧 runtime 已关闭后的诊断错误不应阻止重连: %v", err)
	}
	if !newSessionCalled {
		t.Fatal("旧 runtime 已关闭后应允许启动新 runtime")
	}
}

func TestAgentClientRejectedSessionCleanupBlocksRetryAndIgnoresDiagnostic(t *testing.T) {
	cleanupErr := errors.New("stale runtime exited with status 2")
	firstOpenRelease := make(chan struct{})
	cleanupStarted := make(chan struct{})
	cleanupRelease := make(chan struct{})
	attempts := make(chan agentclient.Options, 2)
	var cleanupGate sync.Once
	client := &agentClient{
		options: agentclient.Options{Model: "old-model"},
		newSession: func(_ context.Context, options agentclient.Options) (*agentclient.Session, error) {
			attempts <- options
			if options.Model == "old-model" {
				<-firstOpenRelease
			}
			return &agentclient.Session{}, nil
		},
		closeSession: func(*agentclient.Session) error {
			shouldWait := false
			cleanupGate.Do(func() { shouldWait = true })
			if !shouldWait {
				return nil
			}
			close(cleanupStarted)
			<-cleanupRelease
			return cleanupErr
		},
	}
	connectDone := make(chan error, 1)
	go func() { connectDone <- client.Connect(context.Background()) }()
	select {
	case <-attempts:
	case <-time.After(time.Second):
		t.Fatal("首次 Connect 未启动")
	}
	if err := client.Reconfigure(context.Background(), agentclient.Options{Model: "new-model"}); err != nil {
		t.Fatalf("连接期间 Reconfigure 失败: %v", err)
	}
	close(firstOpenRelease)
	select {
	case <-cleanupStarted:
	case <-time.After(time.Second):
		t.Fatal("过期 session cleanup 未启动")
	}
	select {
	case options := <-attempts:
		t.Fatalf("旧 runtime 回收完成前不应启动新 runtime: %+v", options)
	case <-time.After(30 * time.Millisecond):
	}
	close(cleanupRelease)
	select {
	case options := <-attempts:
		if options.Model != "new-model" {
			t.Fatalf("cleanup 完成后重试配置=%q", options.Model)
		}
	case <-time.After(time.Second):
		t.Fatal("cleanup 完成后未重试新 runtime")
	}
	select {
	case err := <-connectDone:
		if err != nil {
			t.Fatalf("过期 session 的 cleanup 诊断不应阻止重试: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cleanup 完成后的 Connect 未结束")
	}
}

func TestAgentClientReconfigurePublishesAndSerializesDesiredState(t *testing.T) {
	firstStarted := make(chan struct{})
	firstRelease := make(chan struct{})
	secondStarted := make(chan struct{})
	client := &agentClient{
		options: agentclient.Options{Model: "old-model"},
		session: &agentclient.Session{},
		reconfigureSession: func(_ context.Context, _ *agentclient.Session, options agentclient.Options) error {
			switch options.Model {
			case "first-model":
				close(firstStarted)
				<-firstRelease
			case "second-model":
				close(secondStarted)
			}
			return nil
		},
	}
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- client.Reconfigure(context.Background(), agentclient.Options{Model: "first-model"})
	}()
	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("首次 Reconfigure 未进入 runtime RPC")
	}
	client.mu.Lock()
	modelDuringRPC := client.options.Model
	client.mu.Unlock()
	if modelDuringRPC != "first-model" {
		t.Fatalf("runtime RPC 期间期望配置=%q", modelDuringRPC)
	}

	secondDone := make(chan error, 1)
	go func() {
		secondDone <- client.Reconfigure(context.Background(), agentclient.Options{Model: "second-model"})
	}()
	select {
	case <-secondStarted:
		t.Fatal("前一配置 RPC 完成前不应逆序触达 runtime")
	case <-time.After(30 * time.Millisecond):
	}
	close(firstRelease)
	for name, done := range map[string]<-chan error{
		"first":  firstDone,
		"second": secondDone,
	} {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("%s Reconfigure 失败: %v", name, err)
			}
		case <-time.After(time.Second):
			t.Fatalf("%s Reconfigure 未结束", name)
		}
	}
	select {
	case <-secondStarted:
	default:
		t.Fatal("第二次配置未触达 runtime")
	}
	client.mu.Lock()
	finalModel := client.options.Model
	client.mu.Unlock()
	if finalModel != "second-model" {
		t.Fatalf("最终期望配置=%q", finalModel)
	}
}

func TestAgentClientUpdateEnvironmentPublishesDesiredBeforeRPC(t *testing.T) {
	updateStarted := make(chan struct{})
	updateRelease := make(chan struct{})
	client := &agentClient{
		options: agentclient.Options{Env: map[string]string{"EXISTING": "1"}},
		session: &agentclient.Session{},
		updateSessionEnvironment: func(_ context.Context, _ *agentclient.Session, environment map[string]string) error {
			if environment["NEW"] != "2" {
				t.Errorf("runtime 环境增量=%v", environment)
			}
			close(updateStarted)
			<-updateRelease
			return nil
		},
	}
	updateDone := make(chan error, 1)
	go func() {
		updateDone <- client.UpdateEnvironment(context.Background(), map[string]string{"NEW": "2"})
	}()
	select {
	case <-updateStarted:
	case <-time.After(time.Second):
		t.Fatal("UpdateEnvironment 未进入 runtime RPC")
	}
	client.mu.Lock()
	environmentDuringRPC := maps.Clone(client.options.Env)
	client.mu.Unlock()
	if environmentDuringRPC["EXISTING"] != "1" || environmentDuringRPC["NEW"] != "2" {
		t.Fatalf("runtime RPC 期间期望环境=%v", environmentDuringRPC)
	}
	close(updateRelease)
	select {
	case err := <-updateDone:
		if err != nil {
			t.Fatalf("UpdateEnvironment 失败: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("UpdateEnvironment 未结束")
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

func TestAgentClientConfigurationWaiterHonorsContext(t *testing.T) {
	configuring := &agentClientConfigFlight{done: make(chan struct{})}
	client := &agentClient{
		options:     agentclient.Options{Model: "old-model"},
		configuring: configuring,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := client.Reconfigure(ctx, agentclient.Options{Model: "new-model"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("配置等待应遵守调用方 context: %v", err)
	}
	if client.options.Model != "old-model" {
		t.Fatalf("未获得配置所有权时不应修改期望状态: %q", client.options.Model)
	}
}

func TestAgentClientPreCanceledConfigurationDoesNotMutateDesiredState(t *testing.T) {
	tests := map[string]func(*agentClient, context.Context) error{
		"reconfigure": func(client *agentClient, ctx context.Context) error {
			return client.Reconfigure(ctx, agentclient.Options{Model: "new-model"})
		},
		"environment": func(client *agentClient, ctx context.Context) error {
			return client.UpdateEnvironment(ctx, map[string]string{"NEW": "2"})
		},
		"permission": func(client *agentClient, ctx context.Context) error {
			return client.SetPermissionMode(ctx, sdkpermission.ModeAcceptEdits)
		},
	}
	for name, apply := range tests {
		t.Run(name, func(t *testing.T) {
			client := &agentClient{options: agentclient.Options{
				Model: "old-model",
				Env:   map[string]string{"EXISTING": "1"},
				Runtime: agentclient.RuntimeOptions{
					PermissionMode: sdkpermission.ModePlan,
				},
			}}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := apply(client, ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("预取消配置错误=%v", err)
			}
			if client.options.Model != "old-model" ||
				!maps.Equal(client.options.Env, map[string]string{"EXISTING": "1"}) ||
				client.options.Runtime.PermissionMode != sdkpermission.ModePlan {
				t.Fatalf("预取消配置不应修改期望状态: %+v", client.options)
			}
			if client.configVersion != 0 {
				t.Fatalf("预取消配置不应推进版本: %d", client.configVersion)
			}
		})
	}
}

func TestAgentClientConfigFailureAfterGenerationChangeKeepsDesiredState(t *testing.T) {
	rpcStarted := make(chan struct{})
	rpcRelease := make(chan struct{})
	reconfigureErr := errors.New("old runtime rejected configuration")
	client := &agentClient{
		options:  agentclient.Options{Model: "old-model"},
		session:  &agentclient.Session{},
		messages: make(chan sdkprotocol.ReceivedMessage),
		closeSession: func(*agentclient.Session) error {
			return nil
		},
		reconfigureSession: func(context.Context, *agentclient.Session, agentclient.Options) error {
			close(rpcStarted)
			<-rpcRelease
			return reconfigureErr
		},
	}
	done := make(chan error, 1)
	go func() {
		done <- client.Reconfigure(context.Background(), agentclient.Options{Model: "new-model"})
	}()
	select {
	case <-rpcStarted:
	case <-time.After(time.Second):
		t.Fatal("Reconfigure 未进入旧 runtime RPC")
	}
	client.DiscardUncleanSession()
	close(rpcRelease)
	select {
	case err := <-done:
		if !errors.Is(err, reconfigureErr) {
			t.Fatalf("旧 runtime 配置错误=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("旧 runtime 配置调用未结束")
	}
	client.mu.Lock()
	model := client.options.Model
	client.mu.Unlock()
	if model != "new-model" {
		t.Fatalf("生命周期换代后不应回滚新代 desired state: %q", model)
	}
}

func TestObserveSubagentUsageUsesSessionTaskHighWater(t *testing.T) {
	manager := NewManager()

	if got := manager.ObserveSubagentUsage("session-a", "task-1", 100); got != 100 {
		t.Fatalf("first delta = %d, want 100", got)
	}
	if got := manager.ObserveSubagentUsage("session-a", "task-1", 150); got != 50 {
		t.Fatalf("second delta = %d, want 50", got)
	}
	if got := manager.ObserveSubagentUsage("session-a", "task-1", 150); got != 0 {
		t.Fatalf("duplicate delta = %d, want 0", got)
	}
	if got := manager.ObserveSubagentUsage("session-a", "task-1", 120); got != 0 {
		t.Fatalf("out-of-order delta = %d, want 0", got)
	}
	if got := manager.ObserveSubagentUsage("session-a", "task-1", 180); got != 30 {
		t.Fatalf("later delta = %d, want 30", got)
	}
	manager.mu.Lock()
	delete(manager.sessions, "session-a")
	manager.mu.Unlock()
	if got := manager.ObserveSubagentUsage("session-a", "task-1", 200); got != 20 {
		t.Fatalf("delta after idle state removal = %d, want retained high-water delta 20", got)
	}
	if got := manager.ObserveSubagentUsage("session-b", "task-1", 180); got != 180 {
		t.Fatalf("other session delta = %d, want 180", got)
	}
}
