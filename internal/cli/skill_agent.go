package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
)

func addSkillAgentCommands(parent *cobra.Command, services *cliServiceProvider) {
	parent.AddCommand(
		newSkillAgentListCommand(services),
		newSkillInstallCommand(services),
		newSkillUninstallCommand(services),
	)
}

func newSkillAgentListCommand(services *cliServiceProvider) *cobra.Command {
	var agentID string
	command := &cobra.Command{
		Use:   "agent-list",
		Short: "列出 Agent 已可见技能",
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := skillService(services)
			if err != nil {
				return err
			}
			items, err := service.GetAgentSkills(commandContext(cmd), agentID)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{"domain": "skill", "action": "agent_list", "items": items})
		},
	}
	command.Flags().StringVar(&agentID, "agent-id", "", "agent id")
	_ = command.MarkFlagRequired("agent-id")
	return command
}

func newSkillInstallCommand(services *cliServiceProvider) *cobra.Command {
	var agentID string
	var skillName string
	command := &cobra.Command{
		Use:   "install",
		Short: "为 Agent 安装技能",
		RunE: func(cmd *cobra.Command, _ []string) error {
			appServices, err := services.AppServices()
			if err != nil {
				return err
			}
			targetAgentID, err := resolveSkillInstallAgentID(cmd, appServices, agentID)
			if err != nil {
				return err
			}
			item, err := appServices.Skills.InstallSkill(commandContext(cmd), targetAgentID, skillName)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{"domain": "skill", "action": "install", "item": item})
		},
	}
	command.Flags().StringVar(&agentID, "agent-id", "", "agent id，未传时尝试从 Agent workspace 推断")
	command.Flags().StringVar(&skillName, "skill-name", "", "skill name")
	_ = command.MarkFlagRequired("skill-name")
	return command
}

func newSkillUninstallCommand(services *cliServiceProvider) *cobra.Command {
	var agentID string
	var skillName string
	command := &cobra.Command{
		Use:   "uninstall",
		Short: "从 Agent 卸载技能",
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := skillService(services)
			if err != nil {
				return err
			}
			if err = service.UninstallSkill(commandContext(cmd), agentID, skillName); err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "skill",
				"action": "uninstall",
				"item":   map[string]any{"success": true},
			})
		},
	}
	command.Flags().StringVar(&agentID, "agent-id", "", "agent id")
	command.Flags().StringVar(&skillName, "skill-name", "", "skill name")
	_ = command.MarkFlagRequired("agent-id")
	_ = command.MarkFlagRequired("skill-name")
	return command
}

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
