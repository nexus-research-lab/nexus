package dm

import (
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	"strings"
	"testing"
)

func TestConnectorRuntimeToolPromptGitHubMount(t *testing.T) {
	for _, mounted := range []bool{false, true} {
		servers := map[string]sdkmcp.ServerConfig{}
		if mounted {
			servers["github"] = sdkmcp.HTTPServerConfig{URL: "https://api.githubcopilot.com/mcp/", Headers: map[string]string{"Authorization": "Bearer secret-token"}}
		}
		prompt := connectorRuntimeToolPrompt([]string{"github"}, servers)
		if strings.Contains(prompt, `"connector_id":"github","server_alias":"github"`) != mounted {
			t.Fatalf("GitHub mount projection mismatch: %s", prompt)
		}
		if strings.Contains(prompt, "secret-token") {
			t.Fatal("credential leaked to model context")
		}
	}
}

func TestConnectorRuntimeToolPromptAllBuiltInAliases(t *testing.T) {
	for id, alias := range map[string]string{
		"github": "github", "richmail": "richmail", "amap": "amap_maps", "didi": "didi_ride",
		"dingtalk-ai-table": "dingtalk_ai_table", "tencent-docs": "tencent_docs", "yuque": "yuque", "feishu-docx": "nexus_feishu_docx",
	} {
		prompt := connectorRuntimeToolPrompt([]string{id}, map[string]sdkmcp.ServerConfig{alias: sdkmcp.HTTPServerConfig{URL: "https://example.invalid/mcp"}})
		if !strings.Contains(prompt, `"connector_id":"`+id+`","server_alias":"`+alias+`"`) {
			t.Fatalf("missing mapping for %s", id)
		}
	}
	prompt := connectorRuntimeToolPrompt([]string{"custom-mcp:test"}, map[string]sdkmcp.ServerConfig{"my_tools": sdkmcp.HTTPServerConfig{URL: "https://example.invalid/mcp"}})
	if !strings.Contains(prompt, `"attached_mcp_server_aliases":["my_tools"]`) || !strings.Contains(prompt, "unmapped custom Connector alias do not prove that tools are missing") {
		t.Fatal("custom MCP must preserve actual server facts without declaring missing tools")
	}
}
