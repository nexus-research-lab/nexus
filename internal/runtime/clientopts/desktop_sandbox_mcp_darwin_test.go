//go:build darwin

// INPUT: Nexus 正常 MCP 配置入口、固定 Bridge、真实 nxs 与本机协议服务。
// OUTPUT: 持久化 HTTP 及 Connector SSE 工具经完整模型轮次返回，普通工具联网仍拒绝。
// POS: macOS 产品到内核的 MCP 集成验收；不代表 App UI 或真实第三方 MCP 验收。
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
	"strings"
	"sync/atomic"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

func TestDesktopSandboxRemoteMCPRoundTrip(t *testing.T) {
	binary := os.Getenv("NEXUS_SANDBOX_TEST_BINARY")
	if binary == "" {
		t.Skip("requires an explicit nxs binary")
	}
	if !filepath.IsAbs(binary) {
		t.Fatal("requires an absolute nxs path")
	}
	for _, transport := range []string{"http", "sse"} {
		t.Run(transport, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv(appfs.NexusStateRootEnvName, filepath.Join(root, "state"))
			if err := appfs.EnsureUserRuntimeLayoutAt(filepath.Join(root, "state"), "__system__"); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"workspace", "config", "home", "scratch"} {
				if err := os.MkdirAll(filepath.Join(root, name), 0700); err != nil {
					t.Fatal(err)
				}
			}
			mcpServer, calls := desktopMCPFixture(t, transport)
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
			input := AgentClientOptionsInput{WorkspacePath: filepath.Join(root, "workspace"), OwnerUserID: "__system__", RuntimeKind: "nxs", AppMode: "desktop",
				PermissionMode: sdkpermission.ModeDefault, AutoMemoryDisabled: true, AutoDreamDisabled: true, SettingSources: []string{},
				SandboxResources: &agentclient.SandboxResourcePolicy{Version: 1, WriteScope: agentclient.SandboxWriteScopeWorkspaceWrite, ScratchRoot: filepath.Join(root, "scratch")},
			}
			if transport == "http" {
				input.AgentMCPServers = map[string]any{"fixture_remote": map[string]any{"type": "http", "url": mcpServer.URL, "headers": map[string]any{"Authorization": "Bearer fixture-only"}}}
			} else {
				input.MCPServers = map[string]sdkmcp.ServerConfig{"fixture_remote": sdkmcp.SSEServerConfig{URL: mcpServer.URL, Headers: map[string]string{"Authorization": "Bearer fixture-only"}}}
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
			if calls.Load() != 1 || !observed.Load() {
				t.Fatalf("MCP calls=%d result returned to Provider=%v", calls.Load(), observed.Load())
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
