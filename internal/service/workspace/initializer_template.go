package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

var workspaceFiles = map[string]string{
	"agents": "AGENTS.md",
	"user":   "USER.md",
	"soul":   "SOUL.md",
	"tools":  "TOOLS.md",
}

func ensureWorkspaceTemplateFile(root *confinedfs.Root, targetPath string, content string) error {
	rendered := strings.TrimSpace(content)
	if rendered == "" {
		return nil
	}
	if _, err := root.Lstat(targetPath); err != nil {
		if os.IsNotExist(err) {
			return root.WriteFileAtomic(targetPath, []byte(rendered+"\n"), workspaceFileMode())
		}
		return err
	}
	return nil
}

func removeGeneratedMainAgentsPrompt(root *confinedfs.Root, targetPath string) error {
	contentBytes, err := root.ReadFile(targetPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	content := string(contentBytes)
	if !looksLikeGeneratedMainAgentsPrompt(content) {
		return nil
	}
	return root.Remove(targetPath)
}

func removeGeneratedMainWorkspaceFile(root *confinedfs.Root, targetPath string) error {
	contentBytes, err := root.ReadFile(targetPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	content := string(contentBytes)
	if !looksLikeGeneratedMainWorkspaceFile(filepath.Base(targetPath), content) {
		return nil
	}
	return root.Remove(targetPath)
}

func looksLikeGeneratedMainAgentsPrompt(content string) bool {
	return strings.Contains(content, "## Main Agent Profile") &&
		(strings.Contains(content, "system-level collaboration organizer") || strings.Contains(content, "系统级组织代理"))
}

func looksLikeGeneratedMainWorkspaceFile(fileName string, content string) bool {
	switch fileName {
	case "SOUL.md":
		return strings.Contains(content, "## Personality") && strings.Contains(content, "## Emotion")
	case "TOOLS.md":
		return strings.Contains(content, "## Tool Notes") && strings.Contains(content, "## Skill Notes")
	default:
		return false
	}
}

func buildTemplateContext(agentID string, agentName string, workspacePath string, createdAt time.Time) map[string]string {
	timestamp := createdAt
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	return map[string]string{
		"agent_id":     agentID,
		"agent_name":   agentName,
		"created_at":   timestamp.Format("2006-01-02 15:04:05"),
		"project_root": projectRoot(),
		"workspace":    filepath.Clean(workspacePath),
	}
}

func renderTemplate(raw string, context map[string]string) string {
	replacerArgs := make([]string, 0, len(context)*2)
	for key, value := range context {
		replacerArgs = append(replacerArgs, "{"+key+"}", value)
	}
	return strings.NewReplacer(replacerArgs...).Replace(raw)
}

func workspaceTemplate(key string, isMainAgent bool) string {
	if isMainAgent {
		return mainAgentWorkspaceTemplates[key]
	}
	return defaultWorkspaceTemplates[key]
}
