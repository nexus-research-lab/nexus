package cli

import (
	skillsvc "github.com/nexus-research-lab/nexus/internal/skills"
	"github.com/spf13/cobra"
)

func newSkillCommand(service *skillsvc.Service) *cobra.Command {
	command := &cobra.Command{
		Use:   "skill",
		Short: "skill 领域命令",
	}

	command.AddCommand(func() *cobra.Command {
		var agentID string
		var categoryKey string
		var sourceType string
		var query string
		listCommand := &cobra.Command{
			Use:   "list",
			Short: "列出技能目录",
			RunE: func(cmd *cobra.Command, args []string) error {
				items, err := service.ListSkills(commandContext(cmd), skillsvc.Query{
					AgentID:     agentID,
					CategoryKey: categoryKey,
					SourceType:  sourceType,
					Q:           query,
				})
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "skill",
					"action": "list",
					"items":  items,
				})
			},
		}
		listCommand.Flags().StringVar(&agentID, "agent-id", "", "agent id")
		listCommand.Flags().StringVar(&categoryKey, "category", "", "category key")
		listCommand.Flags().StringVar(&sourceType, "source-type", "", "source type")
		listCommand.Flags().StringVar(&query, "query", "", "search query")
		return listCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var agentID string
		getCommand := &cobra.Command{
			Use:   "get [skill_name]",
			Short: "读取单个技能详情",
			Args:  exactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.GetSkillDetail(commandContext(cmd), args[0], agentID)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "skill",
					"action": "get",
					"item":   item,
				})
			},
		}
		getCommand.Flags().StringVar(&agentID, "agent-id", "", "agent id")
		return getCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var agentID string
		listCommand := &cobra.Command{
			Use:   "agent-list",
			Short: "列出 Agent 已可见技能",
			RunE: func(cmd *cobra.Command, args []string) error {
				items, err := service.GetAgentSkills(commandContext(cmd), agentID)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "skill",
					"action": "agent_list",
					"items":  items,
				})
			},
		}
		listCommand.Flags().StringVar(&agentID, "agent-id", "", "agent id")
		_ = listCommand.MarkFlagRequired("agent-id")
		return listCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var agentID string
		var skillName string
		installCommand := &cobra.Command{
			Use:   "install",
			Short: "为 Agent 安装技能",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.InstallSkill(commandContext(cmd), agentID, skillName)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "skill",
					"action": "install",
					"item":   item,
				})
			},
		}
		installCommand.Flags().StringVar(&agentID, "agent-id", "", "agent id")
		installCommand.Flags().StringVar(&skillName, "skill-name", "", "skill name")
		_ = installCommand.MarkFlagRequired("agent-id")
		_ = installCommand.MarkFlagRequired("skill-name")
		return installCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var agentID string
		var skillName string
		uninstallCommand := &cobra.Command{
			Use:   "uninstall",
			Short: "从 Agent 卸载技能",
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := service.UninstallSkill(commandContext(cmd), agentID, skillName); err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "skill",
					"action": "uninstall",
					"item": map[string]any{
						"success": true,
					},
				})
			},
		}
		uninstallCommand.Flags().StringVar(&agentID, "agent-id", "", "agent id")
		uninstallCommand.Flags().StringVar(&skillName, "skill-name", "", "skill name")
		_ = uninstallCommand.MarkFlagRequired("agent-id")
		_ = uninstallCommand.MarkFlagRequired("skill-name")
		return uninstallCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var path string
		importCommand := &cobra.Command{
			Use:   "import-local",
			Short: "从本地目录导入技能",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.ImportLocalPath(path)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "skill",
					"action": "import_local",
					"item":   item,
				})
			},
		}
		importCommand.Flags().StringVar(&path, "path", "", "skill local path")
		_ = importCommand.MarkFlagRequired("path")
		return importCommand
	}())

	return command
}
