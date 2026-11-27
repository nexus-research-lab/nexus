// execution_context_test.go 覆盖缓存面、命令上下文、上下文用量、输入块排序、中断与结果 Hook 测试。
package runtime

import (
	"context"
	"errors"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type cacheSurfaceMCPServer struct {
	description string
}

const cacheSurfaceNexusServerName = "nexus"

type cacheSurfacePanicServer struct{}

func (cacheSurfacePanicServer) HandleMessage(context.Context, map[string]any) (map[string]any, error) {
	panic("must not escape cache observability")
}

type cacheSurfaceErrorServer struct{}

func (cacheSurfaceErrorServer) HandleMessage(context.Context, map[string]any) (map[string]any, error) {
	return nil, errors.New("unavailable")
}

type cacheSurfaceBlockingServer struct{}

func (cacheSurfaceBlockingServer) HandleMessage(ctx context.Context, _ map[string]any) (map[string]any, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (s cacheSurfaceMCPServer) HandleMessage(
	_ context.Context,
	request map[string]any,
) (map[string]any, error) {
	if request["method"] != "tools/list" {
		return map[string]any{}, nil
	}
	return map[string]any{
		"result": map[string]any{
			"tools": []map[string]any{{
				"name":        "show_widget",
				"description": s.description,
				"inputSchema": map[string]any{"type": "object"},
			}},
		},
	}, nil
}

func TestCacheSurfaceInspectionFailureIsContained(t *testing.T) {
	for name, server := range map[string]sdkmcp.SDKMCPServer{
		"panic":    cacheSurfacePanicServer{},
		"error":    cacheSurfaceErrorServer{},
		"blocking": cacheSurfaceBlockingServer{},
	} {
		t.Run(name, func(t *testing.T) {
			options := agentclient.Options{
				CLIPath: "/test/nxs",
				Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS},
				Env:     map[string]string{"NEXUS_CONFIG_DIR": t.TempDir()},
				MCP: agentclient.MCPOptions{Servers: map[string]sdkmcp.ServerConfig{
					cacheSurfaceNexusServerName: sdkmcp.SDKServerConfig{
						Name: cacheSurfaceNexusServerName, Instance: server,
					},
				}},
			}
			profile, err := cacheSurfaceProfileFromOptions(context.Background(), options)
			if err != nil {
				t.Fatalf("cacheSurfaceProfileFromOptions() error = %v", err)
			}
			if profile.HostToolSurfaceComplete {
				t.Fatalf("inspection %s must mark surface incomplete", name)
			}
			if profile.ToolSurfaceFingerprint == "" {
				t.Fatalf("partial profile = %+v", profile)
			}
			if _, complete, fingerprintErr := ModelToolSurfaceFingerprint(context.Background(), options); fingerprintErr != nil || complete {
				t.Fatalf("inspection %s exported completeness = %v err=%v, want incomplete without error", name, complete, fingerprintErr)
			}
		})
	}
}

func TestManagerCacheSurfaceTracksSuccessfulConfiguration(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	sessionKey := "agent:nexus:default:cache-surface"
	options := agentclient.Options{
		CLIPath: "/test/nxs",
		Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS},
		Env:     map[string]string{"NEXUS_CONFIG_DIR": t.TempDir()},
		MCP: agentclient.MCPOptions{Servers: map[string]sdkmcp.ServerConfig{
			cacheSurfaceNexusServerName: sdkmcp.SDKServerConfig{
				Name:     cacheSurfaceNexusServerName,
				Instance: cacheSurfaceMCPServer{description: "goal"},
			},
		}},
	}
	if _, err := manager.GetOrCreate(context.Background(), sessionKey, options); err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	profile, ok := manager.CacheSurface(sessionKey)
	if !ok || profile.ToolSurfaceFingerprint == "" {
		t.Fatalf("CacheSurface() = %+v, ok=%v", profile, ok)
	}
}

func TestRuntimeCommandContextReadsCurrentSDKSessionIdentity(t *testing.T) {
	identity := NewSDKSessionIdentityState("")
	commandContext := RuntimeCommandContext{SDKSessionIdentity: identity}
	if got := commandContext.CurrentSDKSessionID(); got != "" {
		t.Fatalf("initial SDK session identity = %q", got)
	}
	identity.Set(" sdk-session-fresh ")
	if got := commandContext.CurrentSDKSessionID(); got != "sdk-session-fresh" {
		t.Fatalf("updated SDK session identity = %q", got)
	}
	identity.Set("sdk-session-forked")
	if got := commandContext.CurrentSDKSessionID(); got != "sdk-session-forked" {
		t.Fatalf("forked SDK session identity = %q", got)
	}
}

func TestNormalizeContextUsagePrefersRawWindow(t *testing.T) {
	usage, valid := normalizeContextUsage(agentclient.ContextUsageResponse{
		TotalTokens:  196_000,
		MaxTokens:    245_000,
		RawMaxTokens: 258_000,
		Percentage:   99,
		Model:        " glm-5.2 ",
	})

	if !valid {
		t.Fatal("normalizeContextUsage() valid = false, want true")
	}
	if usage.TotalTokens != 196_000 ||
		usage.MaxTokens != 258_000 ||
		usage.Percentage != 76 ||
		usage.Model != "glm-5.2" {
		t.Fatalf("usage = %#v, want normalized raw window snapshot", usage)
	}
}

func TestManagerRecordContextUsageAfterRuntimeReplacement(t *testing.T) {
	manager := NewManager()
	sessionKey := "agent:agent-a:ws:dm:session-a"
	manager.mu.Lock()
	manager.ensureStateLocked(sessionKey).ContextUsageByAgent = nil
	manager.mu.Unlock()

	manager.RecordContextUsage(sessionKey, "agent-a", protocol.ContextUsageData{
		TotalTokens: 10,
		MaxTokens:   100,
		Percentage:  10,
	})

	if snapshots := manager.ContextUsageSnapshots(sessionKey); len(snapshots) != 1 {
		t.Fatalf("ContextUsageSnapshots() = %#v, want replacement runtime snapshot", snapshots)
	}
}

func TestManagerContextUsageSnapshotsIgnoreInvalidIdentity(t *testing.T) {
	manager := NewManager()
	manager.RecordContextUsage("", "agent-a", protocol.ContextUsageData{MaxTokens: 100})
	manager.RecordContextUsage("agent:agent-a:ws:dm:session-a", "", protocol.ContextUsageData{MaxTokens: 100})
	manager.RecordContextUsage(
		"agent:agent-a:ws:dm:session-a",
		"agent-a",
		protocol.ContextUsageData{},
	)

	if snapshots := manager.ContextUsageSnapshots("agent:agent-a:ws:dm:session-a"); len(snapshots) != 0 {
		t.Fatalf("ContextUsageSnapshots() = %#v, want empty", snapshots)
	}
}

func TestNormalizeContextUsageKeepsNamedCategories(t *testing.T) {
	usage, valid := normalizeContextUsage(agentclient.ContextUsageResponse{
		TotalTokens: 30,
		MaxTokens:   100,
		Categories: []agentclient.ContextUsageCategory{
			{Name: " Messages ", Tokens: 30, Color: "#fff"},
			{Name: "", Tokens: 5},
			{Name: "Skills", Tokens: 0},
			{Name: "Deferred MCP tools", Tokens: 12, IsDeferred: true},
		},
	})
	if !valid {
		t.Fatal("normalizeContextUsage() valid = false, want true")
	}
	want := []protocol.ContextUsageCategory{
		{Name: "Messages", Tokens: 30},
		{Name: "Deferred MCP tools", Tokens: 12, Deferred: true},
	}
	if len(usage.Categories) != len(want) {
		t.Fatalf("categories = %#v, want %#v", usage.Categories, want)
	}
	for index := range want {
		if usage.Categories[index] != want[index] {
			t.Fatalf("categories = %#v, want %#v", usage.Categories, want)
		}
	}
}

func TestNormalizeContextualInputBlocksUsesStablePriorityOrder(t *testing.T) {
	blocks := normalizeContextualInputBlocks([]ContextualInputBlock{
		NewContextualInputBlock("zeta", "same", 10, map[string]string{"b": "2"}),
		NewContextualInputBlock("alpha", "same", 10, nil),
		NewContextualInputBlock("later", "high", 20, nil),
	})
	want := []string{"later", "alpha", "zeta"}
	for index, name := range want {
		if blocks[index].Name != name {
			t.Fatalf("blocks[%d].Name = %q, want %q", index, blocks[index].Name, name)
		}
	}
}

type exactInterruptTestClient struct {
	*fakeRuntimeClient
	interruptCalls int
	interruptErr   error
	onInterrupt    func()
	entered        chan struct{}
	release        chan struct{}
}

func (c *exactInterruptTestClient) Interrupt(context.Context) error {
	c.interruptCalls++
	if c.entered != nil {
		close(c.entered)
	}
	if c.release != nil {
		<-c.release
	}
	if c.onInterrupt != nil {
		c.onInterrupt()
	}
	return c.interruptErr
}

func attachExactInterruptClient(
	manager *Manager,
	sessionKey string,
	client Client,
) {
	manager.mu.Lock()
	manager.ensureStateLocked(sessionKey).Client = client
	manager.mu.Unlock()
}

func TestManagerInterruptRoundRecordsLocalFallbackAfterProviderFailure(t *testing.T) {
	manager := NewManager()
	sessionKey := "agent:worker:ws:dm:cancel-provider-failure"
	localCancelled := false
	if err := manager.StartRound(context.Background(), sessionKey, "round-old", func() {
		localCancelled = true
		manager.MarkRoundFinished(sessionKey, "round-old")
	}); err != nil {
		t.Fatalf("failed to start target: %v", err)
	}
	client := &exactInterruptTestClient{
		fakeRuntimeClient: &fakeRuntimeClient{},
		interruptErr:      errors.New("provider interrupt failed"),
	}
	attachExactInterruptClient(manager, sessionKey, client)

	result, err := manager.InterruptRound(
		context.Background(),
		sessionKey,
		"round-old",
		"execution cancelled",
	)
	if err != nil ||
		result.Outcome != ExactRoundLocalCancelled ||
		result.LimitationCode != "provider_interrupt_failed" {
		t.Fatalf("provider fallback = %+v, err=%v", result, err)
	}
	if client.interruptCalls != 1 || !localCancelled {
		t.Fatalf(
			"provider calls=%d localCancelled=%t",
			client.interruptCalls,
			localCancelled,
		)
	}
}

func TestManagerInterruptRoundFencesConcurrentSuccessorStart(t *testing.T) {
	manager := NewManager()
	sessionKey := "agent:worker:ws:dm:cancel-provider-race"
	if err := manager.StartRound(context.Background(), sessionKey, "round-old", func() {
		manager.MarkRoundFinished(sessionKey, "round-old")
	}); err != nil {
		t.Fatalf("failed to start target: %v", err)
	}
	client := &exactInterruptTestClient{
		fakeRuntimeClient: &fakeRuntimeClient{},
		entered:           make(chan struct{}),
		release:           make(chan struct{}),
	}
	client.onInterrupt = func() {
		manager.MarkRoundFinished(sessionKey, "round-old")
	}
	attachExactInterruptClient(manager, sessionKey, client)
	resultCh := make(chan ExactRoundInterruptResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := manager.InterruptRound(
			context.Background(),
			sessionKey,
			"round-old",
			"execution superseded",
		)
		resultCh <- result
		errCh <- err
	}()
	<-client.entered
	successorRejected := false
	if err := manager.StartRound(context.Background(), sessionKey, "round-successor", func() {
		successorRejected = true
	}); err == nil {
		t.Fatal("successor started during provider interrupt fence")
	} else if !errors.Is(err, ErrRuntimeProviderInterruptInProgress) {
		t.Fatalf("successor rejected with unexpected error: %v", err)
	}
	close(client.release)
	result := <-resultCh
	if err := <-errCh; err != nil ||
		result.Outcome != ExactRoundProviderInterrupted {
		t.Fatalf("provider result = %+v, err=%v", result, err)
	}
	if !successorRejected {
		t.Fatal("rejected successor context was not cancelled")
	}
}

// Explicit stops must not reach the provider as a submit interrupt, which
// providers treat as "new user input arrived" rather than a stop.
func TestClientInterruptWireReasonKeepsExplicitStopsSilent(t *testing.T) {
	for _, reason := range []string{"", clientInterruptWithoutMessage, clientInterruptHostControl} {
		if got := clientInterruptWireReason(reason); got != "" {
			t.Fatalf("wire reason for %q = %q, want empty", reason, got)
		}
	}
	if got := clientInterruptWireReason("收到新的用户消息，上一轮已停止"); got != clientInterruptReasonSubmit {
		t.Fatalf("submit interrupt wire reason = %q", got)
	}
}

type resultCapabilityClient struct {
	*fakeRuntimeClient
	ack, policy bool
}

func (c *resultCapabilityClient) Supports(capability agentclient.Capability) bool {
	return capability == agentclient.CapabilityHookResponseAck && c.ack || capability == agentclient.CapabilityMessageExecutionPolicy && c.policy
}

func TestResultHookRequiresCapabilitiesAndRoutesLoneRound(t *testing.T) {
	for _, ack := range []bool{false, true} {
		for _, policy := range []bool{false, true} {
			if got := SupportsWorkResultHooks(&resultCapabilityClient{ack: ack, policy: policy}); got != (ack && policy) {
				t.Fatalf("capabilities ack=%v policy=%v: %v", ack, policy, got)
			}
		}
	}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	options := manager.WithWorkResultHook(agentclient.Options{}, "session")
	callback := options.Hooks.Matchers[sdkhook.EventStop][0].Hooks[0]
	calls := 0
	record := func(context.Context, sdkhook.Input, string) (sdkhook.Output, error) {
		calls++
		return sdkhook.Output{}, nil
	}
	invoke := func(input sdkhook.Input) {
		t.Helper()
		if _, err := callback(t.Context(), input, ""); err != nil {
			t.Fatal(err)
		}
	}
	invoke(sdkhook.Input{})
	manager.SetWorkResultHook("session", "first", record)
	invoke(sdkhook.Input{AgentID: "child"})
	manager.SetWorkResultHook("session", "second", record)
	invoke(sdkhook.Input{})
	manager.ClearWorkResultHook("session", "first")
	invoke(sdkhook.Input{})
	manager.ClearWorkResultHook("session", "second")
	invoke(sdkhook.Input{})
	if calls != 1 {
		t.Fatalf("ambiguous, child or cleared round was routed: calls=%d", calls)
	}
}

func TestResultHookRoutesConcurrentRoundsByDeclaredRound(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	callback := manager.WithWorkResultHook(agentclient.Options{}, "session").Hooks.Matchers[sdkhook.EventStop][0].Hooks[0]
	routed := map[string]int{}
	register := func(round string) {
		manager.SetWorkResultHook("session", round, func(context.Context, sdkhook.Input, string) (sdkhook.Output, error) {
			routed[round]++
			return sdkhook.Output{}, nil
		})
	}
	register("first")
	register("second")
	declare := func(round string) string {
		return "done\n" + protocol.ExecutionResultMarker + "\n{\"version\":1,\"round_id\":\"" + round + "\",\"delivery\":{\"result_summary\":\"x\"}}\n-->"
	}
	output, err := callback(t.Context(), sdkhook.Input{LastAssistantMessage: declare("second")}, "")
	if err != nil || output.Decision != "" || routed["second"] != 1 || routed["first"] != 0 {
		t.Fatalf("declared round was not routed: output=%+v routed=%v err=%v", output, routed, err)
	}
	output, _ = callback(t.Context(), sdkhook.Input{LastAssistantMessage: declare("other")}, "")
	if output.Decision != "block" || routed["first"]+routed["second"] != 1 {
		t.Fatalf("unmatched declaration must be blocked, not dropped: %+v", output)
	}
	output, _ = callback(t.Context(), sdkhook.Input{LastAssistantMessage: "plain reply"}, "")
	if output.Decision != "" {
		t.Fatalf("plain reply without declaration must pass: %+v", output)
	}
}
