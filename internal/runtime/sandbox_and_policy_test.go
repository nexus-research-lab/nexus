// sandbox_and_policy_test.go 覆盖进程策略、资源、沙箱准入、诊断与替换测试。
package runtime

import (
	"context"
	"errors"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// TestProcessPolicyIncludesHostSandboxRequirements 不因 json:"-" 丢失初始化权威。
func TestProcessPolicyIncludesHostSandboxRequirements(t *testing.T) {
	base := agentclient.Options{Sandbox: &agentclient.SandboxSettings{RequireSandbox: true}}
	previous := managedRuntimeProcessPolicyFingerprint(base)
	for _, change := range []func(){
		func() { base.Sandbox.RequireFileTools = true },
		func() { base.Sandbox.RequireSearchTools = true },
		func() { base.Sandbox.RequireMediaFiles = true },
		func() { base.Sandbox.RequireMediaNetwork = true },
		func() { base.Sandbox.RequireMCPNetwork = true },
		func() { base.Sandbox.RequireMCPHelpers = true },
		func() { base.Sandbox.RequireMCPStdio = true },
		func() { base.MCP.StrictConfig = true },
		func() { base.Sandbox.RequireNotebookFiles = true },
		func() { base.Sandbox.RequireSkillFiles = true },
		func() { base.Sandbox.RequireContextFiles = true },
		func() { base.Sandbox.RequireProjectFiles = true },
		func() { base.Sandbox.RequireManagedPolicy = true },
		func() { base.Sandbox.RequireSettingsFiles = true },
		func() { base.Sandbox.RequireSettingsWrites = true },
		func() {
			base.Sandbox.Resources = &agentclient.SandboxResourcePolicy{Version: 1, WriteScope: agentclient.SandboxWriteScopeReadOnly, ScratchRoot: t.TempDir()}
		},
		func() { base.Sandbox.Resources.WriteScope = agentclient.SandboxWriteScopeWorkspaceWrite },
		func() { base.Sandbox.Resources.ScratchRoot = t.TempDir() },
	} {
		change()
		next := managedRuntimeProcessPolicyFingerprint(base)
		if next == previous {
			t.Fatal("host sandbox requirement did not change process identity")
		}
		previous = next
	}
}

func TestProviderOwnershipChangeReplacesRuntime(t *testing.T) {
	for _, key := range []string{"NEXUS_PROVIDER_MANAGED_BY_HOST", "NEXUS_SUBPROCESS_ENV_SCRUB", "CLAUDE_CODE_SUBPROCESS_ENV_SCRUB", "NEXUS_AUTO_DREAM_WAKE_MODE", "NEXUS_MEMORY_DIR", "NEXUS_ENABLE_REMOTE_MEMORY", "NEXUS_REMOTE_MEMORY_DIR"} {
		t.Run(key, func(t *testing.T) {
			stale, fresh := &fakeRuntimeClient{}, &fakeRuntimeClient{}
			manager := NewManagerWithFactory(&fakeRuntimeFactory{clients: []*fakeRuntimeClient{stale, fresh}})
			base := agentclient.Options{CWD: t.TempDir(), Env: map[string]string{key: "0"}}
			first, err := manager.GetOrCreate(t.Context(), "agent:nexus:ws:dm:provider-policy", base)
			if err != nil {
				t.Fatal(err)
			}
			next := base
			next.Env = map[string]string{key: "1"}
			if key == "NEXUS_AUTO_DREAM_WAKE_MODE" {
				next.Env[key] = "host"
			}
			second, err := manager.GetOrCreate(t.Context(), "agent:nexus:ws:dm:provider-policy", next)
			if err != nil {
				t.Fatal(err)
			}
			if first != stale || second != fresh || stale.reconfigureCalls != 0 || stale.disconnectCalls != 1 {
				t.Fatal("process-owned environment change reused the old runtime")
			}
		})
	}
}

func TestResourceAdmissionBoundsStartupQueue(t *testing.T) {
	m := NewManagerWithFactory(runtimeFactoryFunc(func(agentclient.Options) Client { return &fakeRuntimeClient{} }))
	m.SetResourceLimits(ResourceLimits{MaxSessions: 1})
	client, err := m.GetOrCreate(t.Context(), "rollback", agentclient.Options{})
	if err != nil {
		t.Fatal(err)
	}
	lease, ok := m.CaptureClientLease("rollback", client)
	if !ok {
		t.Fatal("missing rollback lease")
	}
	var startups []*ClientStartup
	defer func() {
		for _, startup := range startups {
			startup.Close()
		}
	}()
	for i := range runtimeMaxPendingStartups {
		startup, err := m.BeginClientStartup(t.Context(), strconv.Itoa(i), "")
		if err != nil {
			t.Fatal(err)
		}
		startups = append(startups, startup)
	}
	if _, err := m.BeginClientStartup(t.Context(), "overflow", ""); !errors.Is(err, ErrRuntimeResourceBusy) {
		t.Fatalf("unbounded startup queue: %v", err)
	}
	if closed, err := m.CloseSessionIfLease(t.Context(), lease); !closed || err != nil {
		t.Fatalf("queued startups blocked rollback: %t, %v", closed, err)
	}
	startups[0].Close()
	startup, err := m.BeginClientStartup(t.Context(), "after-release", "")
	if err != nil {
		t.Fatal(err)
	}
	startup.Close()
}

// 同 key 排队与等待全局名额共享一次等待预算。
func TestResourceAdmissionWaitBudgetIncludesSameSessionQueue(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := NewManagerWithFactory(runtimeFactoryFunc(func(agentclient.Options) Client { return &fakeRuntimeClient{} }))
		m.SetResourceLimits(ResourceLimits{MaxSessions: 1})
		if _, err := m.GetOrCreate(t.Context(), "occupied", agentclient.Options{}); err != nil {
			t.Fatal(err)
		}
		if err := m.StartRound(t.Context(), "occupied", "r", nil); err != nil {
			t.Fatal(err)
		}
		started := time.Now()
		done := make(chan error, 3)
		for range cap(done) {
			go func() { _, err := m.GetOrCreate(t.Context(), "same", agentclient.Options{}); done <- err }()
		}
		for range cap(done) {
			if err := <-done; !errors.Is(err, ErrRuntimeResourceBusy) || !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal(err)
			}
		}
		if elapsed := time.Since(started); elapsed != runtimeResourceWaitTimeout {
			t.Fatalf("three concurrent requests waited %s", elapsed)
		}
	})
}

type followupReceiptClient struct {
	fakeRuntimeClient
	deliver func() (sdkprotocol.TaskRunState, error)
}

func (c *followupReceiptClient) SendTaskMessageWithState(context.Context, string, string, string) (sdkprotocol.TaskRunState, error) {
	return c.deliver()
}

// 回执与消息流可以乱序；续跑投递前占位，旧终态和迟到的 running 回执不能覆盖新状态。
func TestResourceAdmissionFollowupSurvivesEvictionAndReleasesOnTerminal(t *testing.T) {
	client := &followupReceiptClient{}
	m := NewManagerWithFactory(runtimeFactoryFunc(func(agentclient.Options) Client { return client }))
	m.SetResourceLimits(ResourceLimits{MaxSubagents: 1})
	if _, err := m.GetOrCreate(t.Context(), "parent", agentclient.Options{}); err != nil {
		t.Fatal(err)
	}
	update := func(gen int, status string) {
		m.ObserveSubagentRun("parent", client, sdkprotocol.ReceivedMessage{TaskUpdated: &sdkprotocol.TaskUpdatedMessage{TaskID: "child", RunGeneration: gen, Patch: sdkprotocol.TaskUpdatedPatch{Status: status}, Additional: map[string]any{"task_type": "local_agent"}}})
	}
	update(1, "completed")
	count := func() int { m.mu.RLock(); defer m.mu.RUnlock(); return activeSubagentBindings(m.sessions["parent"]) }
	client.deliver = func() (sdkprotocol.TaskRunState, error) {
		if closed, err := m.closeIdleSessions(t.Context(), 0, 1); closed != 0 || err != nil {
			t.Fatalf("evicted during delivery: %d, %v", closed, err)
		}
		if err := m.SendTaskMessage(t.Context(), "parent", "other", "message", ""); !errors.Is(err, ErrRuntimeResourceBusy) {
			t.Fatalf("second child admitted: %v", err)
		}
		return sdkprotocol.TaskRunState{Generation: 2, Status: "running"}, nil
	}
	if err := m.SendTaskMessage(t.Context(), "parent", "child", "resume", ""); err != nil {
		t.Fatal(err)
	}
	update(1, "completed")
	if got := count(); got != 1 {
		t.Fatalf("late previous completion released resumed run: %d", got)
	}
	client.deliver = func() (sdkprotocol.TaskRunState, error) {
		update(2, "completed")
		return sdkprotocol.TaskRunState{Generation: 2, Status: "running"}, nil
	}
	if err := m.SendTaskMessage(t.Context(), "parent", "child", "continue", ""); err != nil {
		t.Fatal(err)
	}
	if got := count(); got != 0 {
		t.Fatalf("late receipt resurrected completed run: %d", got)
	}
}

func TestResourceAdmissionReservesBeforeFactoryAndReleasesFailure(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	m := NewManagerWithFactory(runtimeFactoryFunc(func(agentclient.Options) Client {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
			return nil
		}
		return &fakeRuntimeClient{}
	}))
	m.SetResourceLimits(ResourceLimits{MaxSessions: 1})
	first := make(chan error, 1)
	go func() { _, err := m.GetOrCreate(t.Context(), "first", agentclient.Options{}); first <- err }()
	<-entered
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Millisecond)
	defer cancel()
	if _, err := m.GetOrCreate(ctx, "second", agentclient.Options{}); !errors.Is(err, ErrRuntimeResourceBusy) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("pending factory was oversubscribed or cancellation lost: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("factory calls = %d, want 1", got)
	}
	releaseOnce.Do(func() { close(release) })
	if err := <-first; !errors.Is(err, agentclient.ErrNotConnected) {
		t.Fatalf("first startup = %v", err)
	}
	if _, err := m.GetOrCreate(t.Context(), "third", agentclient.Options{}); err != nil {
		t.Fatalf("failed startup retained reservation: %v", err)
	}
}

// 内存压力可能来自宿主外的程序；一个等待者最多回收一个空闲会话。
func TestResourceAdmissionMemoryPressureReclaimsOncePerWaiter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := NewManagerWithFactory(runtimeFactoryFunc(func(agentclient.Options) Client { return &fakeRuntimeClient{} }))
		keys := []string{"a", "b", "c"}
		for _, key := range keys {
			if _, err := m.GetOrCreate(t.Context(), key, agentclient.Options{}); err != nil {
				t.Fatal(err)
			}
		}
		m.SetResourceLimits(ResourceLimits{MemoryAvailable: func() bool { return false }})
		if _, err := m.GetOrCreate(t.Context(), "new", agentclient.Options{}); !errors.Is(err, ErrRuntimeMemoryPressure) {
			t.Fatalf("memory pressure admitted startup: %v", err)
		}
		remaining := 0
		for _, key := range keys {
			if m.SessionClient(key) != nil {
				remaining++
			}
		}
		if remaining != len(keys)-1 {
			t.Fatalf("memory pressure reclaimed %d sessions, want 1", len(keys)-remaining)
		}
	})
}

func TestDesktopSandboxFullAccessKeepsMCPWithoutRestrictedCapabilities(t *testing.T) {
	options := agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS, PermissionMode: sdkpermission.ModeBypassPermissions}}
	options.MCP.StrictConfig = true
	if err := validateDesktopSandboxOptions(options); err != nil {
		t.Fatal(err)
	}
	if len(requiredSandboxCapabilities(options)) != 0 {
		t.Fatal("full access requires sandbox capabilities")
	}
	options.MCP.StrictConfig = false
	if err := validateDesktopSandboxOptions(options); err == nil {
		t.Fatal("full access silently disabled strict MCP")
	}
	options.MCP.StrictConfig = true
	options.Runtime.PermissionMode = sdkpermission.ModeDefault
	if err := validateDesktopSandboxOptions(options); err == nil {
		t.Fatal("restricted mode accepted absent sandbox")
	}
}

func TestDesktopSandboxOptionsRequireMCPContractOnEveryPlatform(t *testing.T) {
	yes := true
	settings := &agentclient.SandboxSettings{
		RequireSandbox: true, RequireFileTools: true, RequireSearchTools: true, RequireMediaFiles: true, RequireMediaNetwork: true, RequireMCPNetwork: true, RequireMCPHelpers: true, RequireMCPStdio: true, RequireNotebookFiles: true, RequireSkillFiles: true, RequireContextFiles: true, RequireProjectFiles: true, RequireManagedPolicy: true, RequireSettingsFiles: true, RequireSettingsWrites: true, Enabled: &yes, FailIfUnavailable: &yes,
	}
	if goruntime.GOOS == "windows" {
		settings.Resources = &agentclient.SandboxResourcePolicy{Version: 1}
	}
	options := agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS}, Sandbox: settings}
	options.MCP.StrictConfig = true
	if err := validateDesktopSandboxOptions(options); err != nil {
		t.Fatalf("complete contract rejected: %v", err)
	}
	if got := len(requiredSandboxCapabilities(options)); got != len(nxsSandboxRequirements) {
		t.Fatalf("required %d capabilities, want %d", got, len(nxsSandboxRequirements))
	}
	options.MCP.StrictConfig = false
	if err := validateDesktopSandboxOptions(options); err == nil || !strings.Contains(err.Error(), "mcp_strict_config") {
		t.Fatalf("missing strict MCP config accepted: %v", err)
	}
	options.MCP.StrictConfig = true
	settings.RequireMCPStdio = false
	if err := validateDesktopSandboxOptions(options); err == nil || !strings.Contains(err.Error(), "mcp_stdio") {
		t.Fatalf("missing MCP stdio requirement accepted: %v", err)
	}
}

func TestSandboxStartupExplanationPreservesCause(t *testing.T) {
	cause := errors.New("failed to load processmodel.dll: The specified module could not be found. (os error 126)")
	client := &agentClient{newSession: func(context.Context, agentclient.Options) (*agentclient.Session, error) { return nil, cause }}
	_, err := client.openSession(context.Background(), agentclient.Options{})
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), "当前 Windows 未提供桌面沙箱所需的系统 API") {
		t.Fatalf("lost explanation or cause: %v", err)
	}
}

func TestSandboxUnsupportedCreateIsNotMissingDependency(t *testing.T) {
	got := DesktopSandboxUnavailableMessage("the Windows sandbox cannot create a process security environment (stage=availability, build=26300): CreateProcessSecurityEnvironment failed: HRESULT 0x80070032")
	if !strings.Contains(got, "不支持当前安全环境创建请求") || !strings.Contains(got, "设置 → 沙箱") {
		t.Fatalf("unexpected message: %s", got)
	}
	if got := DesktopSandboxUnavailableMessage("CreateProcessSecurityEnvironment failed: HRESULT 0x80070005"); got != "" {
		t.Fatalf("access denied misclassified: %s", got)
	}
}

func TestSandboxRustHelperRepairMessage(t *testing.T) {
	for _, reason := range []string{"Windows sandbox helper nxs-mxc.exe is missing", "Windows sandbox helper version mismatch"} {
		got := DesktopSandboxUnavailableMessage(reason)
		if !strings.Contains(got, "完整安装包") || !strings.Contains(got, "无需自行安装 Rust") || !strings.Contains(got, "设置 → 沙箱") {
			t.Fatalf("unexpected message: %s", got)
		}
	}
}

func TestDesktopSandboxModeChangeRetiresInsteadOfHotUpdate(t *testing.T) {
	for _, modes := range [][2]sdkpermission.Mode{{sdkpermission.ModeDefault, sdkpermission.ModeBypassPermissions}, {sdkpermission.ModeBypassPermissions, sdkpermission.ModeAuto}} {
		client := &agentClient{options: agentclient.Options{Env: map[string]string{protocol.NexusDesktopSandboxPolicyEnvName: "1"}, Runtime: agentclient.RuntimeOptions{PermissionMode: modes[0]}}}
		if err := client.SetPermissionMode(context.Background(), modes[1]); !errors.Is(err, ErrDesktopSandboxPolicyChanged) {
			t.Fatalf("transition did not require replacement: %v", err)
		}
		if !client.retired || client.options.Runtime.PermissionMode != modes[0] {
			t.Fatal("old process was hot-updated or remained usable")
		}
		if err := client.Connect(context.Background()); !errors.Is(err, agentclient.ErrAborted) {
			t.Fatalf("retired client reconnected: %v", err)
		}
	}
	client := &agentClient{options: agentclient.Options{Env: map[string]string{protocol.NexusDesktopSandboxPolicyEnvName: "1"}, Runtime: agentclient.RuntimeOptions{PermissionMode: sdkpermission.ModeDefault}}}
	if err := client.SetPermissionMode(context.Background(), sdkpermission.ModeAuto); err != nil || client.retired {
		t.Fatalf("same sandbox boundary could not hot update: %v", err)
	}
}

func TestManagerHandlesSandboxModeReplacementAsExpectedTransition(t *testing.T) {
	stale := &fakeRuntimeClient{permissionModeErr: ErrDesktopSandboxPolicyChanged}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{clients: []*fakeRuntimeClient{stale}})
	key := "agent:agent-a:ws:dm:policy-switch"
	if _, err := manager.GetOrCreate(context.Background(), key, agentclient.Options{}); err != nil {
		t.Fatal(err)
	}
	if err := manager.SetPermissionModeForAgent(context.Background(), "agent-a", sdkpermission.ModeBypassPermissions); err != nil {
		t.Fatal(err)
	}
	if stale.disconnectCalls != 1 || manager.HasSession(key) {
		t.Fatal("old runtime remained available")
	}
}

func TestManagerReplacesRuntimeForSandboxTransitions(t *testing.T) {
	for _, startRestricted := range []bool{false, true} {
		name := "enable"
		if startRestricted {
			name = "disable"
		}
		t.Run(name, func(t *testing.T) {
			stale, fresh := &fakeRuntimeClient{}, &fakeRuntimeClient{}
			manager := NewManagerWithFactory(&fakeRuntimeFactory{clients: []*fakeRuntimeClient{stale, fresh}})
			enabled := true
			restricted := &agentclient.SandboxSettings{Enabled: &enabled, FailIfUnavailable: &enabled}
			before := agentclient.Options{CWD: t.TempDir()}
			if startRestricted {
				before.Sandbox = restricted
			}
			key := "agent:nexus:ws:dm:sandbox-transition"
			if _, err := manager.GetOrCreate(context.Background(), key, before); err != nil {
				t.Fatal(err)
			}
			after := before
			if startRestricted {
				after.Sandbox = nil
			} else {
				after.Sandbox = restricted
			}
			got, err := manager.GetOrCreate(context.Background(), key, after)
			if err != nil {
				t.Fatal(err)
			}
			if got != fresh || stale.disconnectCalls != 1 || stale.reconfigureCalls != 0 {
				t.Fatalf("sandbox transition reused stale runtime: got=%p fresh=%p disconnect=%d reconfigure=%d", got, fresh, stale.disconnectCalls, stale.reconfigureCalls)
			}
		})
	}
}
