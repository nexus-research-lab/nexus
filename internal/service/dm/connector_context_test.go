package dm

import (
	"context"
	"errors"
	"strings"
	"testing"
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
