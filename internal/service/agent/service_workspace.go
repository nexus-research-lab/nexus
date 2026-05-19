package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxAgentIDWorkspaceAttempts = 8

func (s *Service) createAgentWorkspacePath(ownerUserID string) (string, string, error) {
	for range maxAgentIDWorkspaceAttempts {
		agentID := NewAgentID()
		workspacePath := ResolveWorkspacePath(s.config, ownerUserID, agentID)
		if err := os.Mkdir(workspacePath, 0o755); err == nil {
			return agentID, workspacePath, nil
		} else if err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return "", "", err
		}
	}
	return "", "", fmt.Errorf("无法生成可用的 agent 工作区目录")
}

func syncWorkspaceAgentIdentity(workspacePath string, agentID string, oldName string, newName string) (bool, error) {
	if NormalizeName(oldName) == NormalizeName(newName) {
		return false, nil
	}
	workspacePath = strings.TrimSpace(workspacePath)
	if workspacePath == "" || strings.TrimSpace(agentID) == "" {
		return false, nil
	}
	agentsPath := filepath.Join(workspacePath, "AGENTS.md")
	content, err := os.ReadFile(agentsPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	nextContent, changed := replaceWorkspaceAgentIdentityLine(string(content), agentID, oldName, newName)
	if !changed {
		return false, nil
	}
	info, err := os.Stat(agentsPath)
	if err != nil {
		return false, err
	}
	return true, os.WriteFile(agentsPath, []byte(nextContent), info.Mode())
}

func rollbackWorkspaceAgentIdentity(workspacePath string, agentID string, oldName string, newName string) error {
	_, err := syncWorkspaceAgentIdentity(workspacePath, agentID, oldName, newName)
	return err
}

func replaceWorkspaceAgentIdentityLine(content string, agentID string, oldName string, newName string) (string, bool) {
	oldLine := workspaceAgentIdentityLine(oldName, agentID)
	newLine := workspaceAgentIdentityLine(newName, agentID)
	lines := strings.SplitAfter(content, "\n")
	for index, line := range lines {
		body := strings.TrimRight(line, "\r\n")
		ending := strings.TrimPrefix(line, body)
		if body == oldLine || isWorkspaceAgentIdentityLine(body, agentID) {
			lines[index] = newLine + ending
			return strings.Join(lines, ""), true
		}
	}
	return content, false
}

func workspaceAgentIdentityLine(agentName string, agentID string) string {
	return fmt.Sprintf("当前 Agent 标识：%s（%s）", NormalizeName(agentName), strings.TrimSpace(agentID))
}

func isWorkspaceAgentIdentityLine(line string, agentID string) bool {
	return strings.HasPrefix(line, "当前 Agent 标识：") &&
		strings.Contains(line, "（"+strings.TrimSpace(agentID)+"）")
}
