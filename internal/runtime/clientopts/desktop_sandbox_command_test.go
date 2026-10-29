//go:build darwin || windows

// INPUT: 生产桌面沙箱选项、固定 Bridge、真实 nxs 与脚本化模型夹具。
// OUTPUT: 每轮一个真实 shell 调用的沙箱内结果：临时目录与 workspace 可写，宿主与凭据位置被拒绝。
// POS: 无模型、无密钥的 macOS/Windows 命令边界验收；替代已移除的 scratch 租约断言，不代表 App UI 验收。
package clientopts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

func TestDesktopSandboxScriptedCommandBoundaries(t *testing.T) {
	binary := os.Getenv("NEXUS_SANDBOX_TEST_BINARY")
	if binary == "" {
		t.Skip("requires an explicit nxs binary")
	}
	if !filepath.IsAbs(binary) {
		t.Fatal("requires an absolute nxs path")
	}
	root := t.TempDir()
	if runtime.GOOS == "windows" {
		// Keep PSEC fixture paths below legacy Windows path limits.
		short, err := os.MkdirTemp("", "nxs-host-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(short) })
		root = short
	}
	// 凭据拒绝规则来自宿主进程 HOME；夹具 HOME 避免读取 runner 的真实凭据。
	home := filepath.Join(root, "home")
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	secretSentinel := "SECRET_" + fmt.Sprint(time.Now().UnixNano())
	secretFile := filepath.Join(home, ".ssh", "id_fixture")
	t.Setenv(appfs.NexusStateRootEnvName, filepath.Join(root, "state"))
	if err := appfs.EnsureUserRuntimeLayoutAt(filepath.Join(root, "state"), "__system__"); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(root, "workspace")
	appData := filepath.Join(appfs.AppDir(), "data")
	for _, dir := range []string{workspace, appData, filepath.Dir(secretFile)} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(secretFile, []byte(secretSentinel), 0o600); err != nil {
		t.Fatal(err)
	}
	appProbe := filepath.Join(appData, "probe")
	lateSecret := filepath.Join(appfs.AppDir(), "new-private", "secret")
	npmSecret := filepath.Join(home, ".npmrc")
	publicSkill := filepath.Join(appfs.PlatformSkillRoot(), "fixture.txt")
	if err := os.MkdirAll(filepath.Dir(publicSkill), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(publicSkill, []byte("PUBLIC_SKILL"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(npmSecret, []byte(secretSentinel), 0600); err != nil {
		t.Fatal(err)
	}
	probes := []struct {
		name    string
		command string
		want    string
	}{
		{"tmpdir_write", `echo ok > "$TMPDIR/probe" && cat "$TMPDIR/probe" && echo TMPDIR_WRITE_OK`, "TMPDIR_WRITE_OK"},
		{"python_tempfile", `/usr/bin/python3 -c 'import tempfile;tempfile.mkstemp();print("PY_TMP_OK")' 2>/dev/null`, "PY_TMP_OK"},
		{"workspace_write", `echo ok > ws_probe && echo WS_WRITE_OK`, "WS_WRITE_OK"},
		{"secret_read", fmt.Sprintf(`cat '%s' 2>&1; echo SECRET_RC=$?`, secretFile), "SECRET_RC=1"},
		{"app_write", fmt.Sprintf(`echo x > '%s' 2>&1; echo APP_RC=$?`, appProbe), "APP_RC=1"},
		{"private_boundary", fmt.Sprintf(`cat '%s' >/dev/null 2>&1; a=$?; cat '%s' >/dev/null 2>&1; b=$?; p=$(cat '%s'); (echo overwrite > '%s') 2>/dev/null; c=$?; echo "LATE=$a NPM=$b SKILL=$p WRITE=$c"`, lateSecret, npmSecret, publicSkill, publicSkill), "LATE=1 NPM=1 SKILL=PUBLIC_SKILL WRITE=1"},
	}

	tool := "Bash"
	if runtime.GOOS == "windows" {
		tool = "PowerShell"
		probes[0].command = `$p=Join-Path $env:TEMP 'probe'; Set-Content $p ok; Get-Content $p; 'TMPDIR_WRITE_OK'`
		probes[1].command = `$p=[IO.Path]::GetTempFileName(); Remove-Item $p; 'PY_TMP_OK'`
		probes[2].command = `Set-Content ws_probe ok; 'WS_WRITE_OK'`
		probes[3].command = fmt.Sprintf(`try { Get-Content '%s' -ErrorAction Stop; 'SECRET_RC=0' } catch { 'SECRET_RC=1' }`, secretFile)
		probes[4].command = fmt.Sprintf(`try { Set-Content '%s' x -ErrorAction Stop; 'APP_RC=0' } catch { 'APP_RC=1' }`, appProbe)
		probes[5].command = fmt.Sprintf(`$a=0; $b=0; try { Get-Content '%s' -ErrorAction Stop | Out-Null } catch { $a=1 }; try { Get-Content '%s' -ErrorAction Stop | Out-Null } catch { $b=1 }; $p=Get-Content '%s' -ErrorAction Stop; $c=0; try { Set-Content '%s' overwrite -ErrorAction Stop } catch { $c=1 }; "LATE=$a NPM=$b SKILL=$p WRITE=$c"`, lateSecret, npmSecret, publicSkill, publicSkill)
	}

	var mu sync.Mutex
	next := 0
	results := map[string]string{}
	escapes := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var request struct {
			Tools    []json.RawMessage `json:"tools"`
			Messages []struct {
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(body, &request)
		mu.Lock()
		defer mu.Unlock()
		if n := len(request.Messages); n > 0 {
			var blocks []map[string]any
			if json.Unmarshal(request.Messages[n-1].Content, &blocks) == nil {
				for _, block := range blocks {
					if block["type"] == "tool_result" && next > 0 {
						raw, _ := json.Marshal(block["content"])
						results[probes[next-1].name] = string(raw)
						t.Logf("%s result: %s", probes[next-1].name, raw)
						desktopCommandTextResponse(w)
						return
					}
				}
			}
		}
		if len(request.Tools) == 0 || next >= len(probes) {
			desktopCommandTextResponse(w)
			return
		}
		desktopShellToolResponse(w, tool, fmt.Sprintf("probe-%d", next), probes[next].command)
		next++
	}))
	defer provider.Close()

	input := AgentClientOptionsInput{
		SkillDirectories: []string{appfs.PlatformSkillRoot()}, WorkspacePath: workspace, OwnerUserID: "__system__", RuntimeKind: "nxs", AppMode: "desktop", DesktopSandboxEnabled: true,
		PermissionMode: sdkpermission.ModeDefault, AutoMemoryDisabled: true, AutoDreamDisabled: true, SettingSources: []string{},
		PermissionHandler: func(_ context.Context, r sdkpermission.Request) (sdkpermission.Decision, error) {
			if r.Boundary == sdkpermission.BoundarySandboxEscape || r.Boundary == sdkpermission.BoundarySandboxNetwork {
				mu.Lock()
				escapes++
				mu.Unlock()
				return sdkpermission.Deny("fixture never grants sandbox escape", false), nil
			}
			return sdkpermission.Allow(r.Input, nil), nil
		},
	}
	options, err := BuildAgentClientOptions(t.Context(), fakeRuntimeConfigResolver{config: &RuntimeConfig{Provider: "command-fixture", APIFormat: "anthropic_messages", BaseURL: provider.URL, AuthToken: "local-only", Model: "test-model"}}, input)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "darwin" && options.Sandbox.Resources != nil {
		t.Fatal("stateless desktop sandbox must not carry host scratch resources")
	}

	// Model a host creating a new private directory after the policy was built.
	if err := os.MkdirAll(filepath.Dir(lateSecret), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lateSecret, []byte(secretSentinel), 0600); err != nil {
		t.Fatal(err)
	}
	options.CLIPath = binary
	options.Runtime.InitializeTimeout = 90 * time.Second
	options.Runtime.MaxTurns = 4
	options.Tools.Available = []string{tool}
	options.Skills = agentclient.SkillOptions{Mode: agentclient.SkillModeNone}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	session, err := agentclient.NewSession(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("real nxs session initialized")
	defer func() {
		closeCtx, stop := context.WithTimeout(context.Background(), 20*time.Second)
		defer stop()
		if err := session.Close(closeCtx); err != nil {
			t.Errorf("runtime cleanup failed: %v", err)
		}
	}()
	for _, probe := range probes {
		t.Logf("running %s", probe.name)
		stream, err := session.Send(ctx, "run the next fixture command")
		if err != nil {
			t.Fatal(err)
		}
		for {
			message, err := stream.Recv(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if message.Result != nil {
				break
			}
		}
	}

	mu.Lock()
	defer mu.Unlock()
	for _, probe := range probes {
		result, ok := results[probe.name]
		if !ok {
			t.Errorf("%s: no tool result", probe.name)
			continue
		}
		if !strings.Contains(result, probe.want) {
			t.Errorf("%s: want %q in %s", probe.name, probe.want, result)
		}
	}
	if strings.Contains(results["secret_read"], secretSentinel) {
		t.Error("credential fixture content reached the model")
	}
	if _, err := os.Stat(appProbe); !os.IsNotExist(err) {
		t.Errorf("host app write landed: %v", err)
	}
	if escapes != 0 {
		t.Errorf("fixture commands requested sandbox escape %d times", escapes)
	}
}

// desktopShellToolResponse emits one platform-native shell command.
func desktopShellToolResponse(w http.ResponseWriter, tool, id string, command string) {
	input, _ := json.Marshal(map[string]string{"command": command, "description": "fixture"})
	w.Header().Set("Content-Type", "text/event-stream")
	for _, event := range []map[string]any{
		{"type": "message_start", "message": map[string]any{"id": id, "model": "test-model", "role": "assistant", "content": []any{}, "usage": map[string]int{"input_tokens": 1}}},
		{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "tool_use", "id": id, "name": tool, "input": map[string]any{}}},
		{"type": "content_block_delta", "index": 0, "delta": map[string]string{"type": "input_json_delta", "partial_json": string(input)}},
		{"type": "content_block_stop", "index": 0},
		{"type": "message_delta", "delta": map[string]string{"stop_reason": "tool_use"}, "usage": map[string]int{"output_tokens": 1}},
		{"type": "message_stop"},
	} {
		data, _ := json.Marshal(event)
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event["type"], data)
	}
}

func desktopCommandTextResponse(w http.ResponseWriter) {
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
