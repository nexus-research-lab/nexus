package agent

import (
	"strings"
	"testing"
)

func TestDefaultPromptsSeparateAgentControlSurfaces(t *testing.T) {
	if strings.Contains(defaultBaseSystemPrompt, "nexusctl") ||
		strings.Contains(defaultBaseSystemPrompt, "NEXUSCTL_COMMAND_PATH") {
		t.Fatal("普通 Agent 基础提示词不应暴露 owner 控制面 CLI")
	}
	for _, required := range []string{"NEXUSCFG_COMMAND_PATH", "nexus.command"} {
		if !strings.Contains(defaultBaseSystemPrompt, required) {
			t.Fatalf("普通 Agent 基础提示词缺少受限入口 %q", required)
		}
	}
	if !strings.Contains(defaultMainAgentSystemPrompt, "NEXUSCTL_COMMAND_PATH") {
		t.Fatal("主智能体提示词应保留 owner 控制面 CLI")
	}
}
