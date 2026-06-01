package clientopts

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
)

type fakeRuntimeMCPServer struct{}

func (fakeRuntimeMCPServer) HandleMessage(context.Context, map[string]any) (map[string]any, error) {
	return map[string]any{"ok": true}, nil
}

func TestMaterializeProcessArgFilesForWindowsMovesAppendPromptToFile(t *testing.T) {
	restore := overrideRuntimeArgFilesRoot(t.TempDir())
	defer restore()

	options := agentclient.Options{}
	options.System.Append = "第一行\n第二行"

	if err := materializeProcessArgFilesForOS("windows", &options); err != nil {
		t.Fatalf("materializeProcessArgFilesForOS 失败: %v", err)
	}
	if options.System.Append != "" {
		t.Fatalf("Windows 下不应继续把 append system prompt 放进命令行: %q", options.System.Append)
	}
	path := options.ExtraArgs["append-system-prompt-file"]
	if path == "" {
		t.Fatalf("未生成 append-system-prompt-file 参数: %+v", options.ExtraArgs)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取提示词参数文件失败: %v", err)
	}
	if string(content) != "第一行\n第二行" {
		t.Fatalf("提示词参数文件内容不正确: %q", string(content))
	}
}

func TestMaterializeProcessArgFilesForWindowsUsesMCPConfigFile(t *testing.T) {
	restore := overrideRuntimeArgFilesRoot(t.TempDir())
	defer restore()

	options := agentclient.Options{}
	options.MCP.Servers = map[string]sdkmcp.ServerConfig{
		"nexus_room": sdkmcp.SDKServerConfig{
			Name:     "nexus_room",
			Instance: fakeRuntimeMCPServer{},
		},
		"amap_maps": sdkmcp.HTTPServerConfig{
			URL: "https://mcp.amap.com/mcp?key=test-key",
			Headers: map[string]string{
				"X-Test": "1",
			},
		},
	}

	if err := materializeProcessArgFilesForOS("windows", &options); err != nil {
		t.Fatalf("materializeProcessArgFilesForOS 失败: %v", err)
	}
	if options.MCP.Config == "" {
		t.Fatalf("Windows 下应把 MCP config 写入文件")
	}
	if len(options.MCP.Servers) != 2 {
		t.Fatalf("MCP server registry 应保留给 initialize/control 使用: %+v", options.MCP.Servers)
	}
	data, err := os.ReadFile(options.MCP.Config)
	if err != nil {
		t.Fatalf("读取 MCP 参数文件失败: %v", err)
	}
	var payload map[string]map[string]map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("MCP 参数文件不是合法 JSON: %v", err)
	}
	servers := payload["mcpServers"]
	if servers["nexus_room"]["type"] != "sdk" || servers["nexus_room"]["scope"] != "dynamic" {
		t.Fatalf("SDK MCP server 序列化不正确: %+v", servers["nexus_room"])
	}
	if servers["amap_maps"]["type"] != "http" || servers["amap_maps"]["url"] == "" {
		t.Fatalf("HTTP MCP server 序列化不正确: %+v", servers["amap_maps"])
	}
}

func TestMaterializeProcessArgFilesSkippedOutsideWindows(t *testing.T) {
	restore := overrideRuntimeArgFilesRoot(t.TempDir())
	defer restore()

	options := agentclient.Options{}
	options.System.Append = "保持原样"
	options.MCP.Servers = map[string]sdkmcp.ServerConfig{
		"amap_maps": sdkmcp.HTTPServerConfig{URL: "https://mcp.amap.com/mcp?key=test-key"},
	}

	if err := materializeProcessArgFilesForOS("darwin", &options); err != nil {
		t.Fatalf("materializeProcessArgFilesForOS 失败: %v", err)
	}
	if options.System.Append != "保持原样" {
		t.Fatalf("非 Windows 不应改写提示词参数: %q", options.System.Append)
	}
	if options.MCP.Config != "" {
		t.Fatalf("非 Windows 不应生成 MCP 参数文件: %q", options.MCP.Config)
	}
}

func overrideRuntimeArgFilesRoot(root string) func() {
	previous := runtimeArgFilesRoot
	runtimeArgFilesRoot = func() string {
		return filepath.Clean(root)
	}
	return func() {
		runtimeArgFilesRoot = previous
	}
}
