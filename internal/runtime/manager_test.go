package runtime

import (
	"context"
	"errors"
	"io"
	"maps"
	"slices"
	"strings"
	"sync"
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
	disconnectErr      error
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

type fakeOwnerProcessReaper struct {
	owners []string
	err    error
}

func (r *fakeOwnerProcessReaper) ReapOwnerProcesses(_ context.Context, ownerUserID string) error {
	r.owners = append(r.owners, ownerUserID)
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
		CWD: "/tmp/b",
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
	if client.lastOptions.CWD != "/tmp/b" {
		t.Fatalf("Reconfigure 未收到最新配置: %+v", client.lastOptions)
	}
	if client.lastOptions.Runtime.PermissionMode != sdkpermission.ModeAcceptEdits {
		t.Fatalf("Reconfigure 未收到权限模式: %+v", client.lastOptions)
	}
	if client.lastOptions.Env["NEXUS_OPENAI_PROTOCOL"] != "responses" {
		t.Fatalf("Reconfigure 未收到 Responses 协议更新: %+v", client.lastOptions.Env)
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
	manager.StartRound("session-a-1", "round-a", func() {
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
	if !slices.Equal(reaper.owners, []string{"owner-a"}) {
		t.Fatalf("owner cgroup 回收调用=%v，want [owner-a]", reaper.owners)
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
	if !slices.Equal(reaper.owners, []string{"owner-orphan"}) {
		t.Fatalf("owner cgroup 回收调用=%v，want [owner-orphan]", reaper.owners)
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

func TestWaitSDKClientTransitionReturnsWhenCleanupCompletes(t *testing.T) {
	done := make(chan struct{})
	close(done)
	if err := waitSDKClientTransition(context.Background(), done); err != nil {
		t.Fatalf("已完成 cleanup 不应报错: %v", err)
	}
}

func TestWaitSDKClientTransitionHonorsContext(t *testing.T) {
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitSDKClientTransition(ctx, done); !errors.Is(err, context.Canceled) {
		t.Fatalf("等待 cleanup 应遵守调用方 context: %v", err)
	}
}

func TestSDKClientAdapterCleanupFenceBlocksReconnect(t *testing.T) {
	cleanupStarted := make(chan struct{})
	cleanupRelease := make(chan struct{})
	newSessionCalled := make(chan struct{}, 1)
	var cleanupGate sync.Once
	client := &sdkClientAdapter{
		session:  &agentclient.Session{},
		messages: make(chan sdkprotocol.ReceivedMessage),
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

	client.DiscardUncleanSession()
	select {
	case <-cleanupStarted:
	case <-time.After(time.Second):
		t.Fatal("旧 session cleanup 未启动")
	}
	connectDone := make(chan error, 1)
	go func() { connectDone <- client.Connect(context.Background()) }()
	select {
	case <-newSessionCalled:
		t.Fatal("旧 session cleanup 完成前不应启动新 runtime")
	case <-time.After(30 * time.Millisecond):
	}

	close(cleanupRelease)
	select {
	case <-newSessionCalled:
	case <-time.After(time.Second):
		t.Fatal("旧 session cleanup 完成后未启动新 runtime")
	}
	select {
	case err := <-connectDone:
		if err != nil {
			t.Fatalf("cleanup 后重连失败: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cleanup 后重连未结束")
	}
}

func TestSDKClientAdapterDisconnectDeadlineDoesNotCancelSharedCleanup(t *testing.T) {
	cleanupStarted := make(chan struct{})
	cleanupRelease := make(chan struct{})
	newSessionCalled := make(chan struct{}, 1)
	var cleanupGate sync.Once
	client := &sdkClientAdapter{
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

func TestSDKClientAdapterDisconnectInvalidatesInFlightConnect(t *testing.T) {
	connectStarted := make(chan struct{})
	connectRelease := make(chan struct{})
	staleSessionClosed := make(chan struct{}, 1)
	client := &sdkClientAdapter{
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

func TestSDKClientAdapterConnectRetriesWithLatestConfiguration(t *testing.T) {
	firstAttemptRelease := make(chan struct{})
	attempts := make(chan agentclient.Options, 2)
	client := &sdkClientAdapter{
		options: agentclient.Options{Model: "old-model"},
		newSession: func(_ context.Context, options agentclient.Options) (*agentclient.Session, error) {
			attempts <- options
			if options.Model == "old-model" {
				<-firstAttemptRelease
			}
			return &agentclient.Session{}, nil
		},
		closeSession: func(*agentclient.Session) error { return nil },
	}
	connectDone := make(chan error, 1)
	go func() { connectDone <- client.Connect(context.Background()) }()
	select {
	case options := <-attempts:
		if options.Model != "old-model" {
			t.Fatalf("首次启动配置=%q", options.Model)
		}
	case <-time.After(time.Second):
		t.Fatal("首次 Connect 未启动")
	}

	if err := client.Reconfigure(context.Background(), agentclient.Options{Model: "new-model"}); err != nil {
		t.Fatalf("连接期间 Reconfigure 失败: %v", err)
	}
	close(firstAttemptRelease)
	select {
	case options := <-attempts:
		if options.Model != "new-model" {
			t.Fatalf("重试未使用最新配置: %q", options.Model)
		}
	case <-time.After(time.Second):
		t.Fatal("配置变化后 Connect 未重试")
	}
	select {
	case err := <-connectDone:
		if err != nil {
			t.Fatalf("使用最新配置重试失败: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("配置变化后的 Connect 未结束")
	}
}

func TestSDKClientAdapterDiscardCancelsInFlightConnect(t *testing.T) {
	connectStarted := make(chan struct{})
	client := &sdkClientAdapter{
		newSession: func(ctx context.Context, _ agentclient.Options) (*agentclient.Session, error) {
			close(connectStarted)
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	connectDone := make(chan error, 1)
	go func() { connectDone <- client.Connect(context.Background()) }()
	select {
	case <-connectStarted:
	case <-time.After(time.Second):
		t.Fatal("Connect 未进入 session 启动阶段")
	}

	client.DiscardUncleanSession()
	select {
	case err := <-connectDone:
		if !errors.Is(err, agentclient.ErrAborted) {
			t.Fatalf("Discard 后 Connect 错误=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Discard 未取消正在启动的 session")
	}
}

func TestSDKClientAdapterConcurrentConnectWaiterHonorsContext(t *testing.T) {
	connectStarted := make(chan struct{})
	connectRelease := make(chan struct{})
	client := &sdkClientAdapter{
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

func TestSDKClientAdapterConnectOwnerCancellationDoesNotPoisonWaiter(t *testing.T) {
	firstOpenStarted := make(chan struct{})
	var attemptsMu sync.Mutex
	attempts := 0
	client := &sdkClientAdapter{
		newSession: func(ctx context.Context, _ agentclient.Options) (*agentclient.Session, error) {
			attemptsMu.Lock()
			attempts++
			attempt := attempts
			attemptsMu.Unlock()
			if attempt == 1 {
				close(firstOpenStarted)
				<-ctx.Done()
				return nil, ctx.Err()
			}
			return &agentclient.Session{}, nil
		},
		closeSession: func(*agentclient.Session) error { return nil },
	}
	ownerCtx, cancelOwner := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancelOwner()
	ownerDone := make(chan error, 1)
	go func() { ownerDone <- client.Connect(ownerCtx) }()
	select {
	case <-firstOpenStarted:
	case <-time.After(time.Second):
		t.Fatal("Connect owner 未启动 runtime")
	}
	waiterDone := make(chan error, 1)
	go func() { waiterDone <- client.Connect(context.Background()) }()
	select {
	case err := <-ownerDone:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("owner Connect 错误=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("owner Connect 未按 deadline 退出")
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

func TestSDKClientAdapterDisconnectDuringConfigRetryCannotReviveSession(t *testing.T) {
	firstOpenRelease := make(chan struct{})
	staleCloseStarted := make(chan struct{})
	staleCloseRelease := make(chan struct{})
	attempts := make(chan agentclient.Options, 2)
	client := &sdkClientAdapter{
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

func TestSDKClientAdapterDisconnectReturnsCleanupErrorAndAllowsReconnect(t *testing.T) {
	closeErr := errors.New("runtime close failed")
	newSessionCalled := false
	client := &sdkClientAdapter{
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

func TestSDKClientAdapterRejectedSessionCleanupBlocksRetryUntilCompletion(t *testing.T) {
	firstOpenRelease := make(chan struct{})
	cleanupStarted := make(chan struct{})
	cleanupRelease := make(chan struct{})
	attempts := make(chan agentclient.Options, 2)
	var cleanupGate sync.Once
	client := &sdkClientAdapter{
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
			t.Fatalf("cleanup 完成后的 Connect 失败: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cleanup 完成后的 Connect 未结束")
	}
}

func TestSDKClientAdapterRejectedSessionDiagnosticAllowsRetry(t *testing.T) {
	firstOpenRelease := make(chan struct{})
	attempts := make(chan agentclient.Options, 2)
	client := &sdkClientAdapter{
		options: agentclient.Options{Model: "old-model"},
		newSession: func(_ context.Context, options agentclient.Options) (*agentclient.Session, error) {
			attempts <- options
			if options.Model == "old-model" {
				<-firstOpenRelease
			}
			return &agentclient.Session{}, nil
		},
		closeSession: func(*agentclient.Session) error {
			return errors.New("stale runtime exited with status 2")
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
	case options := <-attempts:
		if options.Model != "new-model" {
			t.Fatalf("诊断错误后重试配置=%q", options.Model)
		}
	case <-time.After(time.Second):
		t.Fatal("已完成的旧 runtime cleanup 不应阻止重试")
	}
	select {
	case err := <-connectDone:
		if err != nil {
			t.Fatalf("已完成的旧 runtime cleanup 不应让 Connect 失败: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("诊断错误后的 Connect 未结束")
	}
}

func TestSDKClientAdapterReconfigurePublishesAndSerializesDesiredState(t *testing.T) {
	firstStarted := make(chan struct{})
	firstRelease := make(chan struct{})
	secondStarted := make(chan struct{})
	client := &sdkClientAdapter{
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

func TestSDKClientAdapterUpdateEnvironmentPublishesDesiredBeforeRPC(t *testing.T) {
	updateStarted := make(chan struct{})
	updateRelease := make(chan struct{})
	client := &sdkClientAdapter{
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

func TestSDKClientAdapterReconfigureRollsBackRejectedDesiredState(t *testing.T) {
	reconfigureErr := errors.New("invalid model")
	client := &sdkClientAdapter{
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

func TestSDKClientAdapterConfigurationWaiterHonorsContext(t *testing.T) {
	configuring := &sdkClientConfigFlight{done: make(chan struct{})}
	client := &sdkClientAdapter{
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

func TestSDKClientAdapterPreCanceledConfigurationDoesNotMutateDesiredState(t *testing.T) {
	tests := map[string]func(*sdkClientAdapter, context.Context) error{
		"reconfigure": func(client *sdkClientAdapter, ctx context.Context) error {
			return client.Reconfigure(ctx, agentclient.Options{Model: "new-model"})
		},
		"environment": func(client *sdkClientAdapter, ctx context.Context) error {
			return client.UpdateEnvironment(ctx, map[string]string{"NEW": "2"})
		},
		"permission": func(client *sdkClientAdapter, ctx context.Context) error {
			return client.SetPermissionMode(ctx, sdkpermission.ModeAcceptEdits)
		},
	}
	for name, apply := range tests {
		t.Run(name, func(t *testing.T) {
			client := &sdkClientAdapter{options: agentclient.Options{
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

func TestSDKClientAdapterConfigFailureAfterGenerationChangeKeepsDesiredState(t *testing.T) {
	rpcStarted := make(chan struct{})
	rpcRelease := make(chan struct{})
	reconfigureErr := errors.New("old runtime rejected configuration")
	client := &sdkClientAdapter{
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
