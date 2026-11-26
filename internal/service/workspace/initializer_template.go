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
