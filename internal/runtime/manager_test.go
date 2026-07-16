package runtime

import (
	"context"
	"errors"
	"io"
	"maps"
	"strings"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type fakeRuntimeClient struct {
	reconfigureCalls   int
	lastOptions        agentclient.Options
	sentContents       []string
	reconfigureErr     error
	disconnectCalls    int
	stoppedTasks       []string
	taskMessages       []fakeTaskMessage
	stopTaskErr        error
	permissionModes    []sdkpermission.Mode
	environmentUpdates []map[string]string
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

func (c *fakeRuntimeClient) Interrupt(context.Context) error { return nil }

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
	return nil
}

func (c *fakeRuntimeClient) Disconnect(context.Context) error {
	c.disconnectCalls++
	return nil
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
	return nil
}

func (c *fakeRuntimeClient) Supports(capability agentclient.Capability) bool {
	return c.hookResponseAck && capability == agentclient.CapabilityHookResponseAck
}

func (c *fakeRuntimeClient) SessionID() string { return "" }

func TestSDKClientAdapterWaitReturnsStreamError(t *testing.T) {
	processErr := errors.New("process: command exited with error: exit status 2")
	client := &sdkClientAdapter{streamErr: processErr}

	if err := client.Wait(); !errors.Is(err, processErr) {
		t.Fatalf("Wait() error = %v，期望返回 stream error", err)
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

func TestManagerGetOrCreateReconfiguresExistingClient(t *testing.T) {
	client := &fakeRuntimeClient{}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: client})

	first, err := manager.GetOrCreate(context.Background(), "agent:nexus:ws:dm:test", agentclient.Options{
		CWD: "/tmp/a",
	})
	if err != nil {
		t.Fatalf("首次创建 client 失败: %v", err)
	}
	second, err := manager.GetOrCreate(context.Background(), "agent:nexus:ws:dm:test", agentclient.Options{
		CWD: "/tmp/b",
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
	if client.lastOptions.CWD != "/tmp/b" {
		t.Fatalf("Reconfigure 未收到最新配置: %+v", client.lastOptions)
	}
	if client.lastOptions.Runtime.PermissionMode != sdkpermission.ModeAcceptEdits {
		t.Fatalf("Reconfigure 未收到权限模式: %+v", client.lastOptions)
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

func TestManagerStopTaskForwardsToRuntimeClient(t *testing.T) {
	client := &fakeRuntimeClient{}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: client})
	sessionKey := "agent:nexus:ws:dm:test"
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{}); err != nil {
		t.Fatalf("创建 runtime client 失败: %v", err)
	}

	if err := manager.StopTask(context.Background(), sessionKey, "task-1"); err != nil {
		t.Fatalf("StopTask 返回错误: %v", err)
	}
	if len(client.stoppedTasks) != 1 || client.stoppedTasks[0] != "task-1" {
		t.Fatalf("stoppedTasks = %+v, want task-1", client.stoppedTasks)
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

func TestManagerStartRoundCancelsIdleMessageDrain(t *testing.T) {
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
		return true
	})
	manager.StartRound(sessionKey, "round-1", nil)
	messages <- sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeTaskNotification}

	select {
	case <-handled:
		t.Fatal("StartRound 后 idle drain 不应继续消费消息")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestManagedGoalMCPCheckResolvesLegacySDKServers(t *testing.T) {
	options := agentclient.Options{
		MCP: agentclient.MCPOptions{
			Servers: map[string]sdkmcp.ServerConfig{
				"nexus_goal": sdkmcp.HTTPServerConfig{URL: "https://example.test/mcp"},
			},
			SDKServers: map[string]sdkmcp.SDKMCPServer{
				"nexus_goal":       fakeSDKMCPServer{},
				"nexus_automation": fakeSDKMCPServer{},
			},
		},
	}

	servers := resolvedMCPServersForManagedGoalCheck(options)
	if len(servers) != 2 {
		t.Fatalf("resolved servers = %+v, want 2", servers)
	}
	if _, ok := servers["nexus_goal"].(sdkmcp.HTTPServerConfig); !ok {
		t.Fatalf("显式 MCP.Servers 应优先于旧式 SDKServers: %+v", servers["nexus_goal"])
	}
	if _, ok := servers["nexus_automation"].(sdkmcp.SDKServerConfig); !ok {
		t.Fatalf("旧式 SDKServers 应合并为 SDKServerConfig: %+v", servers["nexus_automation"])
	}
}

func TestRuntimeRestartsWhenManagedGoalMCPServerSetChanges(t *testing.T) {
	currentOptions := agentclient.Options{
		MCP: agentclient.MCPOptions{
			Servers: map[string]sdkmcp.ServerConfig{
				"nexus_automation": sdkmcp.SDKServerConfig{Name: "nexus_automation", Instance: fakeSDKMCPServer{}},
			},
		},
	}
	nextOptions := agentclient.Options{
		MCP: agentclient.MCPOptions{
			Servers: map[string]sdkmcp.ServerConfig{
				"nexus_automation": sdkmcp.SDKServerConfig{Name: "nexus_automation", Instance: fakeSDKMCPServer{}},
				"nexus_goal":       sdkmcp.SDKServerConfig{Name: "nexus_goal", Instance: fakeSDKMCPServer{}},
			},
		},
	}

	if !shouldRestartForManagedGoalMCPServerSetChange(currentOptions, nextOptions) {
		t.Fatal("新增托管 Goal MCP server 时应重建 SDK client")
	}
	if shouldRestartForManagedGoalMCPServerSetChange(nextOptions, nextOptions) {
		t.Fatal("Goal MCP server 集合未变化时不应重建 SDK client")
	}
	if !shouldReplaceRuntimeClientAfterReconfigureError(errManagedGoalMCPServerSetChanged) {
		t.Fatal("托管 Goal MCP server 集合变化错误应触发 client 替换")
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
				"nexus_goal": sdkmcp.SDKServerConfig{Name: "nexus_goal", Instance: fakeSDKMCPServer{}},
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
	manager.StartRound(sessionKey, "round-queue", func() {})

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

func TestManagerActivateGoalAccounting(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	sessionKey := "agent:nexus:ws:dm:test-goal-activate"
	calls := []string{}
	manager.RegisterGoalAccountingActivate(sessionKey, "round-b", func(context.Context) error {
		calls = append(calls, "round-b")
		return nil
	})
	manager.RegisterGoalAccountingActivate(sessionKey, "round-a", func(context.Context) error {
		calls = append(calls, "round-a")
		return nil
	})

	roundIDs, err := manager.ActivateGoalAccounting(context.Background(), sessionKey)
	if err != nil {
		t.Fatalf("ActivateGoalAccounting() error = %v", err)
	}
	if strings.Join(roundIDs, ",") != "round-a,round-b" {
		t.Fatalf("roundIDs = %#v, want sorted round-a/round-b", roundIDs)
	}
	if strings.Join(calls, ",") != "round-a,round-b" {
		t.Fatalf("calls = %#v, want sorted round-a/round-b", calls)
	}

	manager.RegisterGoalAccountingActivate(sessionKey, "round-a", nil)
	calls = nil
	roundIDs, err = manager.ActivateGoalAccounting(context.Background(), sessionKey)
	if err != nil {
		t.Fatalf("ActivateGoalAccounting() after unregister error = %v", err)
	}
	if strings.Join(roundIDs, ",") != "round-b" || strings.Join(calls, ",") != "round-b" {
		t.Fatalf("after unregister roundIDs=%#v calls=%#v, want only round-b", roundIDs, calls)
	}

	manager.MarkRoundFinished(sessionKey, "round-b")
	roundIDs, err = manager.ActivateGoalAccounting(context.Background(), sessionKey)
	if err != nil || len(roundIDs) != 0 {
		t.Fatalf("after round finished roundIDs=%#v err=%v, want empty nil", roundIDs, err)
	}
}

func TestManagerGuidanceHookInjectsPostToolUseAdditionalContext(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	sessionKey := "agent:nexus:ws:dm:test-guide"
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{}); err != nil {
		t.Fatalf("创建 client 失败: %v", err)
	}
	manager.StartRound(sessionKey, "round-guide", func() {})

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
	manager.StartRound(sessionKey, "round-guide", func() {})

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
	manager.StartRound(sessionKey, "round-recipient", func() {})
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
	manager.StartRound(sessionKey, "round-recipient", func() {})
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
	manager.StartRound(activeKey, "round-active", nil)

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
	manager.StartRound(sessionKey, "round-finish", nil)

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
