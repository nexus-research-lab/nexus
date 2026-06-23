package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

var workspaceFiles = map[string]string{
	"agents": "AGENTS.md",
	"user":   "USER.md",
	"memory": "MEMORY.md",
	"soul":   "SOUL.md",
	"tools":  "TOOLS.md",
}

func ensureWorkspaceTemplateFile(targetPath string, key string, content string) error {
	rendered := strings.TrimSpace(content)
	if rendered == "" {
		return nil
	}
	if _, err := os.Stat(targetPath); err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(targetPath, []byte(rendered+"\n"), 0o644)
		}
		return err
	}
	if key != "agents" {
		return nil
	}
	return repairAgentsScheduleGuidance(targetPath)
}

func repairAgentsScheduleGuidance(targetPath string) error {
	// TODO: 迁移期清理旧 AGENTS.md 里的 ScheduleWakeup 说明；确认旧 workspace 已覆盖后删除。
	currentBytes, err := os.ReadFile(targetPath)
	if err != nil {
		return err
	}
	current := string(currentBytes)
	if !strings.Contains(current, "ScheduleWakeup / Cron*（harness 内置）= 会话内自我提醒") {
		return nil
	}
	repaired, ok := removeMarkdownSection(current, []string{"## 定时任务路由", "## 定时任务", "## Scheduled Task Routing", "## Scheduled Tasks"})
	if !ok || repaired == current {
		return nil
	}
	return os.WriteFile(targetPath, []byte(strings.TrimRight(repaired, "\n")+"\n"), 0o644)
}

func removeGeneratedMainAgentsPrompt(targetPath string) error {
	contentBytes, err := os.ReadFile(targetPath)
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
	return os.Remove(targetPath)
}

func removeGeneratedMainWorkspaceFile(targetPath string) error {
	contentBytes, err := os.ReadFile(targetPath)
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
	return os.Remove(targetPath)
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

func removeMarkdownSection(current string, headings []string) (string, bool) {
	for _, heading := range headings {
		currentStart, currentEnd, currentOK := markdownSectionBounds(current, heading)
		if !currentOK {
			continue
		}
		return current[:currentStart] + current[currentEnd:], true
	}
	return "", false
}

func markdownSectionBounds(content string, heading string) (int, int, bool) {
	start := -1
	if strings.HasPrefix(content, heading+"\n") {
		start = 0
	} else if index := strings.Index(content, "\n"+heading+"\n"); index >= 0 {
		start = index + 1
	}
	if start < 0 {
		return 0, 0, false
	}
	searchFrom := start + len(heading) + 1
	if next := strings.Index(content[searchFrom:], "\n## "); next >= 0 {
		return start, searchFrom + next + 1, true
	}
	return start, len(content), true
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
