//go:build darwin

// INPUT: Explicit live-provider opt-in and host credentials supplied by the runner.
// OUTPUT: Real model/tool, denied-file, cancellation and close evidence for each backend.
// POS: macOS host integration acceptance; neither App UI nor arbitrary descendant isolation.
package clientopts

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
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
				AppMode: "desktop", PermissionMode: sdkpermission.ModeDefault,
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
			if kind == "nxs" {
				input.SandboxResources = &agentclient.SandboxResourcePolicy{Version: 1, WriteScope: agentclient.SandboxWriteScopeWorkspaceWrite, ScratchRoot: scratch}
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
					trace.results += string(result.Content)
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
