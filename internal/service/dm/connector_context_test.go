package dm

import (
	"context"
	"errors"
	"strings"
	"testing"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
)

func TestConnectorRuntimeStatePromptSeparatesConfigurationFromSessionSelection(t *testing.T) {
	service := &Service{
		connectorRuntimeStates: func(context.Context, string) ([]ConnectorRuntimeState, error) {
			return []ConnectorRuntimeState{
				{ConnectorID: "feishu-docx", Configured: true},
				{ConnectorID: "github", Configured: false},
			}, nil
		},
	}
	prompt := service.connectorRuntimeStatePrompt(context.Background(), "owner-a", []string{"github"})
	for _, want := range []string{
		`"connector_id":"feishu-docx","configuration_state":"configured","selected_in_current_session":false`,
		`"connector_id":"github","configuration_state":"not_configured","selected_in_current_session":true`,
		"do not start authorization again",
		`click the "+" button on the left side of the chat composer`,
		`Do not vaguely refer to "session settings"`,
		"Do not call nexusctl",
		"Do not let stale conclusions from earlier turns override the current schema",
		"runtime mount problem",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("Connector prompt 缺少 %q: %s", want, prompt)
		}
	}
}

func TestConnectorRuntimeStatePromptKeepsSelectedStateWhenConfigurationReadFails(t *testing.T) {
	service := &Service{
		connectorRuntimeStates: func(context.Context, string) ([]ConnectorRuntimeState, error) {
			return nil, errors.New("database unavailable")
		},
	}
	prompt := service.connectorRuntimeStatePrompt(context.Background(), "owner-a", []string{"feishu-docx"})
	if !strings.Contains(prompt, `"connector_id":"feishu-docx","configuration_state":"unknown","selected_in_current_session":true`) {
		t.Fatalf("配置读取失败时未保留 Session 选择事实: %s", prompt)
	}
}

func TestConnectorRuntimeToolPromptPublishesCurrentTurnQualifiedNames(t *testing.T) {
	prompt := connectorRuntimeToolPrompt(
		[]string{"feishu-docx"},
		map[string]sdkmcp.ServerConfig{
			"nexus_feishu_docx": sdkmcp.SDKServerConfig{Name: "nexus_feishu_docx"},
			"nexus":             sdkmcp.SDKServerConfig{Name: "nexus"},
		},
	)
	for _, want := range []string{
		`"connector_id":"feishu-docx","server_alias":"nexus_feishu_docx"`,
		`mcp__nexus_feishu_docx__read`,
		`supersedes stale tool-availability claims`,
		`Do not ask the user to open another Session`,
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("当前轮 Connector 工具事实缺少 %q: %s", want, prompt)
		}
	}
}

func TestConnectorRuntimeToolPromptDoesNotClaimDetachedServer(t *testing.T) {
	prompt := connectorRuntimeToolPrompt([]string{"feishu-docx"}, nil)
	if strings.Contains(prompt, `"server_alias":"nexus_feishu_docx"`) ||
		strings.Contains(prompt, `mcp__nexus_feishu_docx__read`) {
		t.Fatalf("未装配 server 时不应宣称工具可用: %s", prompt)
	}
}
