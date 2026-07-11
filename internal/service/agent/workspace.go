package agent

import (
	"errors"
	"fmt"
	"os"
)

const maxAgentIDWorkspaceAttempts = 8

func (s *Service) createAgentWorkspacePath(ownerUserID string) (string, string, error) {
	for range maxAgentIDWorkspaceAttempts {
		agentID := NewAgentID()
		workspacePath := ResolveWorkspacePath(s.config, ownerUserID, agentID)
		if err := os.Mkdir(workspacePath, 0o755); err == nil {
			return agentID, workspacePath, nil
		} else {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return "", "", err
		}
	}
	return "", "", fmt.Errorf("无法生成可用的 agent 工作区目录")
}
