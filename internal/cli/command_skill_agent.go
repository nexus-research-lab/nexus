package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
)

func resolveSkillInstallAgentID(
	cmd *cobra.Command,
	appServices *serverapp.AppServices,
	agentID string,
) (string, error) {
	if trimmed := strings.TrimSpace(agentID); trimmed != "" {
		return trimmed, nil
	}
	if inferred := inferCLIWorkspaceAgentID(cmd, appServices); inferred != "" {
		return inferred, nil
	}
	return "", usageErrorf("必须提供 --agent-id，或在 Agent runtime 中通过 %s 推断", nexusctlWorkspacePathEnvName)
}

func inferCLIWorkspaceAgentID(
	cmd *cobra.Command,
	appServices *serverapp.AppServices,
) string {
	if appServices == nil || appServices.Core == nil || appServices.Core.Agent == nil {
		return ""
	}
	workspacePath := filepath.Clean(strings.TrimSpace(os.Getenv(nexusctlWorkspacePathEnvName)))
	if workspacePath == "." {
		return ""
	}
	agents, err := appServices.Core.Agent.ListAgentRecords(commandContext(cmd))
	if err != nil {
		return ""
	}
	for _, agentValue := range agents {
		agentWorkspace := filepath.Clean(strings.TrimSpace(agentValue.WorkspacePath))
		if agentWorkspace == "." || agentWorkspace == "" {
			continue
		}
		if workspacePath == agentWorkspace || strings.HasPrefix(workspacePath, agentWorkspace+string(os.PathSeparator)) {
			return agentValue.AgentID
		}
	}
	return ""
}
