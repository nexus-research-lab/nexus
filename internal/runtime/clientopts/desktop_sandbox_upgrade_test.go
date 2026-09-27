//go:build darwin

// INPUT: 已发布 nxs、候选 nxs 和隔离的本地 Anthropic 协议夹具。
// OUTPUT: 旧内核创建、升级续用、旧内核回退续用的同一持久会话与原配置证据。
// POS: macOS 数据/协议兼容性验收；不是签名 App、数据库降级或真实 Provider 验收。
package clientopts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

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
				input.SandboxResources = &agentclient.SandboxResourcePolicy{Version: 1,
					WriteScope: agentclient.SandboxWriteScopeWorkspaceWrite, ScratchRoot: filepath.Join(root, "scratch")}
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
