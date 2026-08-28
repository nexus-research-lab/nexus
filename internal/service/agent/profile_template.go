package agent

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

const agentProfileFilePath = "AGENTS.md"

const defaultAgentProfileTemplate = `## Role

- Purpose:
- Responsibilities:
- Out of scope:
- Preferred working style:

## Baseline Rules

- Follow the injected Agent Identity, Agent Profile, and this file first for role-specific behavior.
- Keep file and shell work inside WORKING DIRECTORY unless the user explicitly gives another safe path.
- Inspect the real source of truth before making claims about files, logs, databases, APIs, tools, or prior work.
- The owner control-plane CLI is reserved for Nexus main agent and unavailable here. Use the round-scoped nexuscfg and nexus.command capabilities instead; do not search for or reconstruct a host CLI.
- Do not invent facts, tool results, files, links, or completed actions.
- If a request is ambiguous but inspectable, inspect first. Ask only when acting would be risky.
`

// DefaultProfileTemplate 返回创建普通 Agent 时使用的行为模板。
func DefaultProfileTemplate() string {
	return normalizeProfileTemplate(defaultAgentProfileTemplate)
}

func writeProfileTemplate(workspacePath string, requested string) error {
	root, err := confinedfs.Open(workspacePath)
	if err != nil {
		return err
	}
	defer root.Close()
	return writeProfileTemplateAt(root, requested)
}

func writeProfileTemplateAt(root *confinedfs.Root, requested string) error {
	content := normalizeProfileTemplate(requested)
	if strings.TrimSpace(requested) == "" {
		content = DefaultProfileTemplate()
	}
	return root.WriteFileAtomic(
		agentProfileFilePath,
		[]byte(content),
		agentWorkspaceFileMode(0o644),
	)
}

func normalizeProfileTemplate(content string) string {
	return strings.TrimRight(content, "\r\n") + "\n"
}
