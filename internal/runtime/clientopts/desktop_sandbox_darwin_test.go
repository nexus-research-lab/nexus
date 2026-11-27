//go:build darwin

// desktop_sandbox_darwin_test.go 覆盖 macOS 桌面沙箱 live、MCP 与升级回归测试。
package clientopts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

func TestDesktopSandboxLiveProvider(t *testing.T) {
	if os.Getenv("NEXUS_SANDBOX_LIVE_PROVIDER") != "1" {
		t.Skip("explicit live-provider acceptance only; use scripts/desktop/check-live-sandbox.mjs")
	}
	token := os.Getenv("NEXUS_SANDBOX_LIVE_TOKEN")
	baseURL := os.Getenv("NEXUS_SANDBOX_LIVE_BASE_URL")
	model := os.Getenv("NEXUS_SANDBOX_LIVE_MODEL")
	if token == "" || baseURL == "" || model == "" {
		t.Fatal("live acceptance requires a token, base URL and exact model")
	}
	// The probe-only secret must not be inherited by the runtime or its tools.
	t.Setenv("NEXUS_SANDBOX_LIVE_TOKEN", "")
	redact := func(s string) string { return strings.ReplaceAll(s, token, "[redacted]") }
	selected := os.Getenv("NEXUS_SANDBOX_LIVE_RUNTIME")
	for _, kind := range []string{"nxs", "claude"} {
		if selected != "" && selected != "both" && selected != kind {
			continue
		}
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv(appfs.NexusStateRootEnvName, filepath.Join(root, "state"))
			if err := appfs.EnsureUserRuntimeLayoutAt(filepath.Join(root, "state"), "__system__"); err != nil {
				t.Fatal(err)
			}
			workspace := filepath.Join(root, "workspace")
			scratch := filepath.Join(root, "scratch")
			blocked := filepath.Join(appfs.AppDir(), "data")
			blockedShell := filepath.Join(blocked, "blocked-shell")
			publicSkills := appfs.HostSkillRoot()
			for _, dir := range []string{workspace, scratch, blocked, blockedShell, publicSkills} {
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			var mu sync.Mutex
			var diagnostics []string
			var approvals, escapesDenied int
			input := AgentClientOptionsInput{
				WorkspacePath: workspace, OwnerUserID: "__system__", RuntimeKind: kind,
				AppMode: "desktop", DesktopSandboxEnabled: true, PermissionMode: sdkpermission.ModeDefault,
				AutoMemoryDisabled: true, AutoDreamDisabled: true,
				SkillDirectories: []string{publicSkills},
				PermissionHandler: func(_ context.Context, r sdkpermission.Request) (sdkpermission.Decision, error) {
					mu.Lock()
					defer mu.Unlock()
					if r.Boundary == sdkpermission.BoundarySandboxEscape || r.Boundary == sdkpermission.BoundarySandboxNetwork {
						escapesDenied++
						return sdkpermission.Deny("This acceptance probe does not grant sandbox escape or network access.", false), nil
					}
					if r.ToolName != "Read" && r.ToolName != "Write" && r.ToolName != "Bash" {
						return sdkpermission.Deny("Tool outside acceptance fixture.", false), nil
					}
					approvals++
					return sdkpermission.Allow(r.Input, nil), nil
				},
			}
			options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{config: &RuntimeConfig{
				Provider: "live-acceptance", APIFormat: "anthropic_messages", AuthToken: token, BaseURL: baseURL, Model: model,
			}}, input)
			if err != nil {
				t.Fatal(redact(err.Error()))
			}
			options.CLIPath = os.Getenv("NEXUS_SANDBOX_TEST_BINARY")
			if kind == "claude" {
				options.CLIPath = os.Getenv("NEXUS_SANDBOX_CLAUDE_BINARY")
			}
			if options.CLIPath == "" {
				t.Fatal("runtime binary path is required")
			}
			options.Runtime.InitializeTimeout = 30 * time.Second
			options.Runtime.MaxTurns = 8
			options.Runtime.MaxBudgetUSD = 2
			options.Tools.Available = []string{"Read", "Write", "Bash"}
			options.Skills = agentclient.SkillOptions{Mode: agentclient.SkillModeNone}
			options.MCP.StrictConfig = true
			// 宿主 app 目录拒绝规则必须来自生产装配，测试不再额外注入。
			if options.Sandbox.Network == nil {
				options.Sandbox.Network = &agentclient.SandboxNetworkConfig{}
			}
			options.Sandbox.Network.DeniedDomains = []string{"example.com"}
			options.Callbacks.Stderr = func(line string) {
				mu.Lock()
				defer mu.Unlock()
				diagnostics = append(diagnostics, redact(line))
				if len(diagnostics) > 12 {
					diagnostics = diagnostics[len(diagnostics)-12:]
				}
			}
			t.Cleanup(func() {
				if t.Failed() {
					mu.Lock()
					defer mu.Unlock()
					t.Logf("runtime diagnostics: %s", strings.Join(diagnostics, "\n"))
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			session, err := agentclient.NewSession(ctx, options)
			if err != nil {
				t.Fatal(redact(err.Error()))
			}
			closed := false
			t.Cleanup(func() {
				if closed {
					return
				}
				closeCtx, stop := context.WithTimeout(context.Background(), 20*time.Second)
				defer stop()
				if err := session.Close(closeCtx); err != nil {
					t.Errorf("close: %s", redact(err.Error()))
				}
			})
			t.Log("live provider initialized with desktop sandbox contract")
			nonce := "NEXUS_LIVE_" + strconv.FormatInt(time.Now().UnixNano(), 10)
			allowed := filepath.Join(workspace, "allowed.txt")
			trace := liveSandboxTurn(t, ctx, session, fmt.Sprintf("This is a harmless integration acceptance test. Use Write to create %q containing exactly %q (no newline). Then use Bash to run cat %q. Use both tools; do not merely describe the task. Do not read any other files. Reply DONE afterwards.", allowed, nonce, allowed), redact)
			body, err := os.ReadFile(allowed)
			if err != nil || string(body) != nonce || !trace.tools["Write"] || !trace.tools["Bash"] || !strings.Contains(trace.results, nonce) {
				t.Fatalf("real file/command probe incomplete: file_match=%t Write=%t Bash=%t result_match=%t", string(body) == nonce, trace.tools["Write"], trace.tools["Bash"], strings.Contains(trace.results, nonce))
			}
			t.Log("PASS live model -> native Write -> sandboxed Bash read")
			publicFile := filepath.Join(publicSkills, "read-only-fixture.md")
			if err := os.WriteFile(publicFile, []byte(nonce), 0600); err != nil {
				t.Fatal(err)
			}
			trace = liveSandboxTurn(t, ctx, session, fmt.Sprintf("Use Read once to read the allowed read-only Skill fixture %q. Do not use other tools or modify it. Report its content.", publicFile), redact)
			if !trace.tools["Read"] || trace.errors != 0 || !strings.Contains(trace.results, nonce) {
				t.Fatal("public Skill projection read failed")
			}
			t.Log("PASS public Skill projection remains readable")

			deniedFile := filepath.Join(blocked, "native.txt")
			trace = liveSandboxTurn(t, ctx, session, fmt.Sprintf("The host intentionally denies this empty test-fixture directory. Attempt exactly one Write of the text BLOCKED to %q to verify rejection. A denial is the expected successful test outcome. Do not retry, bypass, change permissions or use other tools. Report the observed rejection briefly.", deniedFile), redact)
			if _, err := os.Stat(deniedFile); !os.IsNotExist(err) || !trace.tools["Write"] || trace.errors == 0 || !liveSandboxDenied(trace.results) {
				t.Fatalf("native denial not proven: absent=%t Write=%t error_results=%d", os.IsNotExist(err), trace.tools["Write"], trace.errors)
			}
			t.Logf("native denial evidence: %s", redact(trace.results))
			t.Log("PASS real native file denial; target absent")
			readProbe := filepath.Join(blocked, "host-read-sentinel.txt")
			readSentinel := "HOST_PRIVATE_" + strconv.FormatInt(time.Now().UnixNano(), 10)
			if err := os.WriteFile(readProbe, []byte(readSentinel), 0600); err != nil {
				t.Fatal(err)
			}
			trace = liveSandboxTurn(t, ctx, session, fmt.Sprintf("Verify the configured native file boundary on a harmless fixture. Attempt Read exactly once on %q. Rejection is expected; do not retry, bypass, or use another tool. Report the observed result briefly.", readProbe), redact)
			if !trace.tools["Read"] || trace.errors == 0 || !liveSandboxDenied(trace.results) || strings.Contains(trace.results, readSentinel) {
				t.Fatalf("native read denial not proven: Read=%t error_results=%d", trace.tools["Read"], trace.errors)
			}
			t.Log("PASS real native read denial; host sentinel not returned")

			recordPath := filepath.Join(root, "state", "NexusSidecar.pid.json")
			recordSentinel := "SIDECAR_IDENTITY_" + strconv.FormatInt(time.Now().UnixNano(), 10)
			if err := os.WriteFile(recordPath, []byte(recordSentinel), 0600); err != nil {
				t.Fatal(err)
			}
			trace = liveSandboxTurn(t, ctx, session, fmt.Sprintf("Test the sidecar identity file boundary on this harmless fixture. Use Read exactly once on %q. Rejection is expected. Do not retry, bypass or use another tool.", recordPath), redact)
			if !trace.tools["Read"] || trace.errors == 0 || !liveSandboxDenied(trace.results) || strings.Contains(trace.results, recordSentinel) {
				t.Fatal("sidecar identity native read denial not proven")
			}
			trace = liveSandboxTurn(t, ctx, session, fmt.Sprintf("Test the sidecar identity write boundary on a harmless fixture. Invoke Bash exactly once with command %q. Failure is expected. Do not retry, bypass or request unsandboxed execution.", "/usr/bin/ruby -e 'File.write(ARGV.fetch(0), \"BLOCKED\")' '"+recordPath+"'"), redact)
			unchanged, readErr := os.ReadFile(recordPath)
			if readErr != nil || string(unchanged) != recordSentinel || !trace.tools["Bash"] || trace.errors == 0 || !liveSandboxDenied(trace.results) {
				t.Fatal("sidecar identity command write denial not proven")
			}
			t.Log("PASS sidecar identity read/write denied; original record unchanged")

			deniedShell := filepath.Join(blockedShell, "shell.txt")
			trace = liveSandboxTurn(t, ctx, session, fmt.Sprintf("Separately test the OS command sandbox on a harmless host-owned fixture. Invoke Bash exactly once with command %q. Failure is expected. Do not retry, bypass or request unsandboxed execution. Report the actual tool result.", "/usr/bin/ruby -e 'File.write(ARGV.fetch(0), \"BLOCKED\")' '"+deniedShell+"'"), redact)
			if _, err := os.Stat(deniedShell); !os.IsNotExist(err) || !trace.tools["Bash"] || trace.errors == 0 || !liveSandboxDenied(trace.results) {
				t.Fatalf("shell denial not proven: absent=%t Bash=%t error_results=%d", os.IsNotExist(err), trace.tools["Bash"], trace.errors)
			}
			t.Logf("command denial evidence: %s", redact(trace.results))
			t.Log("PASS real command write denial; target absent")
			trace = liveSandboxTurn(t, ctx, session, "Test a harmless explicitly denied network destination. Use Bash exactly once to run: curl --silent --show-error --fail --connect-timeout 3 --max-time 5 https://example.com/ . A proxy/sandbox denial is expected. Do not retry, bypass, or use another network tool. Report the tool result briefly.", redact)
			networkResult := strings.ToLower(trace.results)
			if !trace.tools["Bash"] || trace.errors == 0 || !(strings.Contains(networkResult, "403") || strings.Contains(networkResult, "denied") || strings.Contains(networkResult, "blocked")) {
				t.Fatalf("network policy denial not proven: %s", redact(trace.results))
			}
			t.Logf("network denial evidence: %s", redact(trace.results))
			t.Log("PASS command network denied; no sandbox escape granted")
			pidFile := filepath.Join(workspace, "cancel.pid")
			stream, err := session.Send(ctx, fmt.Sprintf("Final cancellation fixture: invoke Bash with exactly %q and timeout 60000. Run in the foreground; the host will interrupt it. Do not start other commands.", "echo $$ > '"+pidFile+"'; exec /bin/sleep 45"))
			if err != nil {
				t.Fatal(redact(err.Error()))
			}
			drainCtx, stopDrain := context.WithCancel(ctx)
			defer stopDrain()
			go func() {
				for {
					m, e := stream.Recv(drainCtx)
					if e != nil || m.Result != nil {
						return
					}
				}
			}()
			var pid int
			deadline := time.Now().Add(60 * time.Second)
			for time.Now().Before(deadline) {
				if data, e := os.ReadFile(pidFile); e == nil {
					pid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
					if pid > 1 {
						break
					}
				}
				time.Sleep(100 * time.Millisecond)
			}
			if pid <= 1 {
				t.Fatal("cancellation fixture never started")
			}
			controlCtx, stopControl := context.WithTimeout(context.Background(), 20*time.Second)
			err = session.Interrupt(controlCtx)
			stopControl()
			if err != nil {
				t.Fatal("interrupt: " + redact(err.Error()))
			}
			closeCtx, stopClose := context.WithTimeout(context.Background(), 20*time.Second)
			err = session.Close(closeCtx)
			stopClose()
			closed = true
			if err != nil {
				t.Fatal("close: " + redact(err.Error()))
			}
			deadline = time.Now().Add(5 * time.Second)
			for syscall.Kill(pid, 0) == nil && time.Now().Before(deadline) {
				time.Sleep(50 * time.Millisecond)
			}
			if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
				t.Fatalf("fixture process termination unconfirmed: %v", err)
			}
			mu.Lock()
			t.Logf("PASS interrupt and close; fixture exited; ordinary approvals=%d boundary denials=%d", approvals, escapesDenied)
			mu.Unlock()
		})
	}
}

type liveSandboxTrace struct {
	tools   map[string]bool
	results string
	errors  int
}

func liveSandboxDenied(result string) bool {
	result = strings.ToLower(result)
	return strings.Contains(result, "permission") || strings.Contains(result, "denied") || strings.Contains(result, "not permitted")
}

func liveSandboxTurn(t *testing.T, ctx context.Context, session *agentclient.Session, prompt string, redact func(string) string) liveSandboxTrace {
	t.Helper()
	trace := liveSandboxTrace{tools: map[string]bool{}}
	stream, err := session.Send(ctx, prompt)
	if err != nil {
		t.Fatal(redact(err.Error()))
	}
	for {
		message, err := stream.Recv(ctx)
		if err != nil {
			t.Fatal(redact(err.Error()))
		}
		if message.Assistant != nil {
			for _, block := range message.Assistant.Message.Content {
				if tool, ok := sdkprotocol.AsToolUseBlock(block); ok {
					trace.tools[tool.Name] = true
					t.Logf("tool attempted: %s", tool.Name)
				}
			}
		}
		if message.User != nil {
			for _, block := range message.User.Message.Content {
				if result, ok := sdkprotocol.AsToolResultBlock(block); ok {
					trace.results += string(result.ContentJSON())
					if result.IsError {
						trace.errors++
					}
				}
			}
		}
		if message.Result != nil {
			if message.Result.IsError {
				t.Fatalf("provider turn failed: %s %s", message.Result.Subtype, redact(strings.Join(message.Result.Errors, "; ")))
			}
			return trace
		}
	}
}

func TestDesktopSandboxRemoteMCPRoundTrip(t *testing.T) {
	testDesktopSandboxMCPRoundTrip(t, []string{"http", "sse", "http_helper", "sse_helper"})
}

func TestDesktopSandboxStdioMCPRoundTrip(t *testing.T) {
	testDesktopSandboxMCPRoundTrip(t, []string{"stdio_persisted", "stdio_connector"})
}

// testDesktopSandboxMCPRoundTrip 共用真实 nxs 的模型/工具轮次，并分别走持久配置与 Connector 入口。
func testDesktopSandboxMCPRoundTrip(t *testing.T, scenarios []string) {
	binary := os.Getenv("NEXUS_SANDBOX_TEST_BINARY")
	if binary == "" {
		t.Skip("requires an explicit nxs binary")
	}
	if !filepath.IsAbs(binary) {
		t.Fatal("requires an absolute nxs path")
	}
	for _, scenario := range scenarios {
		t.Run(scenario, func(t *testing.T) {
			transport := strings.TrimSuffix(scenario, "_helper")
			root := t.TempDir()
			t.Setenv(appfs.NexusStateRootEnvName, filepath.Join(root, "state"))
			if err := appfs.EnsureUserRuntimeLayoutAt(filepath.Join(root, "state"), "__system__"); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"workspace", "config", "home"} {
				if err := os.MkdirAll(filepath.Join(root, name), 0700); err != nil {
					t.Fatal(err)
				}
			}
			var endpoint string
			var calls func() int32
			if !strings.HasPrefix(transport, "stdio_") {
				mcpServer, count := desktopMCPFixture(t, transport)
				endpoint, calls = mcpServer.URL, count.Load
			}
			var observed atomic.Bool
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if strings.Contains(string(body), "tool_result") && strings.Contains(string(body), "mcp-roundtrip-proof") {
					observed.Store(true)
					upgradeTextResponse(w)
					return
				}
				desktopMCPToolResponse(w)
			}))
			defer provider.Close()
			input := AgentClientOptionsInput{WorkspacePath: filepath.Join(root, "workspace"), OwnerUserID: "__system__", RuntimeKind: "nxs", AppMode: "desktop", DesktopSandboxEnabled: true,
				PermissionMode: sdkpermission.ModeDefault, AutoMemoryDisabled: true, AutoDreamDisabled: true, SettingSources: []string{},
			}
			if strings.HasPrefix(transport, "stdio_") {
				command, err := os.Executable()
				if err != nil {
					t.Fatal(err)
				}
				// 无状态沙箱没有宿主 scratch；stdio MCP 只能写 workspace 与 TMPDIR。
				countFile := filepath.Join(root, "workspace", "stdio-calls")
				calls = func() int32 {
					data, err := os.ReadFile(countFile)
					if err != nil {
						t.Fatal(err)
					}
					return int32(strings.Count(string(data), "call\n"))
				}
				args := []string{"-test.run=^TestDesktopSandboxStdioFixtureProcess$"}
				environment := map[string]string{"MCP_STDIO_FIXTURE": "1", "MCP_STDIO_COUNT_FILE": countFile, "MCP_SERVICE_TOKEN": "fixture-only"}
				if transport == "stdio_persisted" {
					input.AgentMCPServers = map[string]any{"fixture_remote": map[string]any{"command": command, "args": args, "env": environment}}
				} else {
					input.MCPServers = map[string]sdkmcp.ServerConfig{"fixture_remote": sdkmcp.StdioServerConfig{Command: command, Args: args, Env: environment}}
				}
			} else if transport == "http" {
				input.AgentMCPServers = map[string]any{"fixture_remote": map[string]any{"type": "http", "url": endpoint, "headers": map[string]any{"Authorization": "Bearer fixture-only"}}}
			} else {
				input.MCPServers = map[string]sdkmcp.ServerConfig{"fixture_remote": sdkmcp.SSEServerConfig{URL: endpoint, Headers: map[string]string{"Authorization": "Bearer fixture-only"}}}
			}
			if strings.HasSuffix(scenario, "_helper") {
				helper := `printf '{"Authorization":"Bearer fixture-only"}'`
				if transport == "http" {
					configured := input.AgentMCPServers["fixture_remote"].(map[string]any)
					configured["headersHelper"] = helper
					configured["headers"] = map[string]any{"Authorization": "must-be-replaced"}
				} else {
					input.MCPServers["fixture_remote"] = sdkmcp.SSEServerConfig{URL: endpoint, HeadersHelper: helper, Headers: map[string]string{"Authorization": "must-be-replaced"}}
				}
			}
			options, err := BuildAgentClientOptions(t.Context(), fakeRuntimeConfigResolver{config: &RuntimeConfig{Provider: "mcp-fixture", APIFormat: "anthropic_messages", BaseURL: provider.URL, AuthToken: "local-only", Model: "test-model"}}, input)
			if err != nil {
				t.Fatal(err)
			}
			if options.Sandbox.Network.AllowedDomains == nil || len(options.Sandbox.Network.AllowedDomains) != 0 {
				t.Fatal("MCP widened ordinary tool network")
			}
			options.CLIPath = binary
			options.Env["NEXUS_CONFIG_DIR"] = filepath.Join(root, "config")
			options.Env["CLAUDE_CONFIG_DIR"] = filepath.Join(root, "config")
			options.Env["HOME"] = filepath.Join(root, "home")
			options.Skills = agentclient.SkillOptions{Mode: agentclient.SkillModeNone}
			options.Runtime.InitializeTimeout = 20 * time.Second
			options.Runtime.MaxTurns = 3
			options.Callbacks.PermissionHandler = func(_ context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
				if request.ToolName != "mcp__fixture_remote__echo" {
					return sdkpermission.Deny("unexpected tool", false), nil
				}
				return sdkpermission.Allow(request.Input, nil), nil
			}
			ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
			defer cancel()
			session, err := agentclient.NewSession(ctx, options)
			if err != nil {
				t.Fatal(err)
			}
			defer upgradeClose(t, session)
			if !session.Supports(agentclient.CapabilitySandboxMCPNetwork) {
				t.Fatal("missing MCP network confirmation")
			}
			if !session.Supports(agentclient.CapabilitySandboxMCPHelpers) {
				t.Fatal("missing MCP helper confirmation")
			}
			if !session.Supports(agentclient.CapabilitySandboxMCPStdio) {
				t.Fatal("missing MCP stdio confirmation")
			}
			stream, err := session.Send(ctx, "Call the configured echo MCP tool once.")
			if err != nil {
				t.Fatal(err)
			}
			for {
				message, err := stream.Recv(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if message.Result != nil {
					if message.Result.IsError {
						t.Fatalf("round failed: %v", message.Result.Errors)
					}
					break
				}
			}
			if calls() != 1 || !observed.Load() {
				t.Fatalf("MCP calls=%d result returned to Provider=%v", calls(), observed.Load())
			}
		})
	}
}

// desktopMCPFixture 实现真实 discovery 和 echo，不依赖外部模型或认证。
func desktopMCPFixture(t *testing.T, transport string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	calls := &atomic.Int32{}
	messages := make(chan map[string]any, 16)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-only" {
			t.Error("MCP authentication header lost")
		}
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprint(w, "event: endpoint\ndata: /messages?session=fixture\n\n")
			w.(http.Flusher).Flush()
			for {
				select {
				case message := <-messages:
					data, _ := json.Marshal(message)
					_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
					w.(http.Flusher).Flush()
				case <-r.Context().Done():
					return
				}
			}
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		result := map[string]any{}
		switch request["method"] {
		case "initialize":
			result = map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "fixture", "version": "1"}}
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
			return
		case "tools/list":
			result["tools"] = []any{map[string]any{"name": "echo", "description": "Return fixture proof", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{}}}}
		case "resources/list":
			result["resources"] = []any{}
		case "tools/call":
			calls.Add(1)
			result["content"] = []any{map[string]any{"type": "text", "text": "mcp-roundtrip-proof"}}
		}
		response := map[string]any{"jsonrpc": "2.0", "id": request["id"], "result": result}
		if transport == "sse" {
			messages <- response
			w.WriteHeader(http.StatusAccepted)
		} else {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}
	}))
	t.Cleanup(server.Close)
	return server, calls
}

// desktopMCPToolResponse 固定模型夹具，只发出一个实际 MCP 工具调用。
func desktopMCPToolResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, event := range []map[string]any{
		{"type": "message_start", "message": map[string]any{"id": "mcp-fixture", "model": "test-model", "role": "assistant", "content": []any{}, "usage": map[string]int{"input_tokens": 1}}},
		{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "tool_use", "id": "fixture-call", "name": "mcp__fixture_remote__echo", "input": map[string]any{}}},
		{"type": "content_block_delta", "index": 0, "delta": map[string]string{"type": "input_json_delta", "partial_json": "{}"}},
		{"type": "content_block_stop", "index": 0},
		{"type": "message_delta", "delta": map[string]string{"stop_reason": "tool_use"}, "usage": map[string]int{"output_tokens": 1}},
		{"type": "message_stop"},
	} {
		data, _ := json.Marshal(event)
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event["type"], data)
	}
}

// TestDesktopSandboxStdioFixtureProcess 是子进程专用入口；退出前不打印 Go test 正文。
func TestDesktopSandboxStdioFixtureProcess(t *testing.T) {
	if os.Getenv("MCP_STDIO_FIXTURE") != "1" {
		return
	}
	if os.Getenv("MCP_SERVICE_TOKEN") != "fixture-only" || os.Getenv("ANTHROPIC_AUTH_TOKEN") != "" || os.Getenv("OPENAI_API_KEY") != "" {
		os.Exit(2)
	}
	decoder, encoder := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
	for {
		var request map[string]any
		if err := decoder.Decode(&request); err != nil {
			os.Exit(0)
		}
		if _, hasID := request["id"]; !hasID {
			continue
		}
		result := map[string]any{}
		switch request["method"] {
		case "initialize":
			result = map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{}, "serverInfo": map[string]any{"name": "fixture", "version": "1"}}
		case "tools/list":
			result["tools"] = []any{map[string]any{"name": "echo", "description": "Return fixture proof", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{}}}}
		case "resources/list":
			result["resources"] = []any{}
		case "tools/call":
			f, err := os.OpenFile(os.Getenv("MCP_STDIO_COUNT_FILE"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
			if err != nil {
				os.Exit(3)
			}
			_, err = f.WriteString("call\n")
			f.Close()
			if err != nil {
				os.Exit(4)
			}
			result["content"] = []any{map[string]any{"type": "text", "text": "mcp-roundtrip-proof"}}
		}
		if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": request["id"], "result": result}); err != nil {
			os.Exit(5)
		}
	}
}

func TestDesktopSandboxRuntimeUpgradeAndRollback(t *testing.T) {
	previous := os.Getenv("NEXUS_SANDBOX_UPGRADE_FROM_BINARY")
	candidate := os.Getenv("NEXUS_SANDBOX_TEST_BINARY")
	if previous == "" || candidate == "" {
		t.Skip("requires explicit released and candidate nxs binaries")
	}
	for _, binary := range []string{previous, candidate} {
		if !filepath.IsAbs(binary) {
			t.Fatal("upgrade acceptance requires absolute runtime paths")
		}
	}
	root := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, filepath.Join(root, "state"))
	if err := appfs.EnsureUserRuntimeLayoutAt(filepath.Join(root, "state"), "__system__"); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(root, "workspace")
	config := filepath.Join(root, "config")
	for _, dir := range []string{workspace, config, filepath.Join(root, "home"), filepath.Join(root, "scratch")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	unchanged := map[string]string{
		filepath.Join(config, "settings.json"):        `{"env":{"NEXUS_UPGRADE_FIXTURE":"preserved"},"permissions":{"allow":["Read"]}}` + "\n",
		filepath.Join(config, "config.json"):          `{"theme":"dark"}` + "\n",
		filepath.Join(workspace, "MEMORY.md"):         "User memory preserved across the sandbox upgrade.\n",
		filepath.Join(workspace, "existing-work.txt"): "Existing user content.\n",
	}
	for file, content := range unchanged {
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	payloads := make(chan map[string]json.RawMessage, 16)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		select {
		case payloads <- payload:
		default:
			t.Error("unexpected extra provider requests")
		}
		upgradeTextResponse(w)
	}))
	defer upstream.Close()
	var sessionID string
	var transcript string
	var originalTranscript []byte
	var prompts []string
	for _, phase := range []struct {
		name, binary string
		sandbox      bool
	}{
		{"before_upgrade", previous, false}, {"after_upgrade", candidate, true}, {"after_rollback", previous, false},
	} {
		if !t.Run(phase.name, func(t *testing.T) {
			input := AgentClientOptionsInput{
				WorkspacePath: workspace, OwnerUserID: "__system__", RuntimeKind: "nxs",
				PermissionMode: sdkpermission.ModeDefault, ResumeSessionID: sessionID,
				AutoMemoryDisabled: true, AutoDreamDisabled: true, SettingSources: []string{"user"},
			}
			if phase.sandbox {
				input.AppMode = "desktop"
			}
			options, err := BuildAgentClientOptions(t.Context(), fakeRuntimeConfigResolver{config: &RuntimeConfig{
				Provider: "upgrade-fixture", APIFormat: "anthropic_messages", BaseURL: upstream.URL,
				AuthToken: "local-fixture-only", Model: "test-model",
			}}, input)
			if err != nil {
				t.Fatal(err)
			}
			options.CLIPath = phase.binary
			options.Env["NEXUS_CONFIG_DIR"] = config
			options.Env["CLAUDE_CONFIG_DIR"] = config
			options.Env["HOME"] = filepath.Join(root, "home")
			options.Skills = agentclient.SkillOptions{Mode: agentclient.SkillModeNone}
			options.MCP.StrictConfig = true
			options.Runtime.InitializeTimeout = 20 * time.Second
			options.Runtime.MaxTurns = 1
			ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
			defer cancel()
			session, err := agentclient.NewSession(ctx, options)
			if err != nil {
				t.Fatal(err)
			}
			closed := false
			t.Cleanup(func() {
				if !closed {
					upgradeClose(t, session)
				}
			})
			prompt := "upgrade-compatibility-" + phase.name
			stream, err := session.Send(ctx, prompt)
			if err != nil {
				t.Fatal(err)
			}
			for {
				message, err := stream.Recv(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if message.Result != nil {
					if message.Result.IsError {
						t.Fatalf("provider round failed: %v", message.Result.Errors)
					}
					break
				}
			}
			if sessionID == "" {
				sessionID = session.ID()
			}
			if sessionID == "" || session.ID() != sessionID {
				t.Fatal("upgrade changed persistent session identity")
			}
			upgradeClose(t, session)
			closed = true
			select {
			case payload := <-payloads:
				prompts = append(prompts, prompt)
				for _, expected := range prompts {
					if !upgradeHistoryHas(payload["messages"], "user", expected) {
						t.Fatalf("provider history lost prior user turn %q", expected)
					}
				}
				if len(prompts) > 1 && !upgradeHistoryHas(payload["messages"], "assistant", "upgrade-fixture-reply") {
					t.Fatal("provider history lost prior assistant response")
				}
			case <-ctx.Done():
				t.Fatal("missing actual provider request")
			}
			for file, content := range unchanged {
				actual, err := os.ReadFile(file)
				if err != nil || string(actual) != content {
					t.Fatalf("existing file changed: %s: %v", filepath.Base(file), err)
				}
			}
			if transcript == "" {
				err := filepath.WalkDir(config, func(file string, entry os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if !entry.IsDir() && entry.Name() == sessionID+".jsonl" {
						transcript = file
					}
					return nil
				})
				if err != nil || transcript == "" {
					t.Fatalf("missing persisted transcript: %v", err)
				}
			}
			actual, err := os.ReadFile(transcript)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(actual, originalTranscript) {
				t.Fatal("existing transcript bytes were rewritten or truncated")
			}
			originalTranscript = actual
		}) {
			return
		}
	}
}

func upgradeClose(t *testing.T, session *agentclient.Session) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := session.Close(ctx); err != nil {
		t.Fatalf("runtime cleanup: %v", err)
	}
}

func upgradeHistoryHas(raw json.RawMessage, role, text string) bool {
	var messages []struct {
		Role    string
		Content json.RawMessage
	}
	if json.Unmarshal(raw, &messages) != nil {
		return false
	}
	for _, message := range messages {
		if message.Role != role {
			continue
		}
		var plain string
		if json.Unmarshal(message.Content, &plain) == nil && strings.Contains(plain, text) {
			return true
		}
		var blocks []struct{ Type, Text string }
		if json.Unmarshal(message.Content, &blocks) == nil {
			for _, block := range blocks {
				if block.Type == "text" && strings.Contains(block.Text, text) {
					return true
				}
			}
		}
	}
	return false
}

func upgradeTextResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, event := range []map[string]any{
		{"type": "message_start", "message": map[string]any{"id": "upgrade-fixture", "model": "test-model", "role": "assistant", "content": []any{}, "usage": map[string]int{"input_tokens": 1}}},
		{"type": "content_block_start", "index": 0, "content_block": map[string]string{"type": "text", "text": ""}},
		{"type": "content_block_delta", "index": 0, "delta": map[string]string{"type": "text_delta", "text": "upgrade-fixture-reply"}},
		{"type": "content_block_stop", "index": 0},
		{"type": "message_delta", "delta": map[string]string{"stop_reason": "end_turn"}, "usage": map[string]int{"output_tokens": 1}},
		{"type": "message_stop"},
	} {
		data, _ := json.Marshal(event)
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event["type"], data)
	}
}
