package cli

import (
	"github.com/spf13/cobra"

	skillsvc "github.com/nexus-research-lab/nexus/internal/service/skills"
)

func newSkillCommand(services *cliServiceProvider) *cobra.Command {
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
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Skills
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
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Skills
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
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Skills
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
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Skills
				targetAgentID, err := resolveSkillInstallAgentID(cmd, appServices, agentID)
				if err != nil {
					return err
				}
				item, err := service.InstallSkill(commandContext(cmd), targetAgentID, skillName)
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
		installCommand.Flags().StringVar(&agentID, "agent-id", "", "agent id，未传时尝试从 Agent workspace 推断")
		installCommand.Flags().StringVar(&skillName, "skill-name", "", "skill name")
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
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Skills
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
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Skills
				item, err := service.ImportLocalPath(commandContext(cmd), path)
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

	command.AddCommand(func() *cobra.Command {
		var query string
		var includeReadme bool
		searchCommand := &cobra.Command{
			Use:   "search-external [query]",
			Short: "搜索外部技能来源",
			RunE: func(cmd *cobra.Command, args []string) error {
				if len(args) > 1 {
					return usageErrorf("最多只能提供一个 query")
				}
				if query == "" && len(args) == 1 {
					query = args[0]
				}
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				item, err := appServices.Skills.SearchExternalSkills(commandContext(cmd), query, includeReadme)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "skill",
					"action": "search_external",
					"item":   item,
				})
			},
		}
		searchCommand.Flags().StringVar(&query, "query", "", "search query")
		searchCommand.Flags().BoolVar(&includeReadme, "include-readme", false, "include readme preview")
		return searchCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var repositoryURL string
		var branch string
		var skillPath string
		importCommand := &cobra.Command{
			Use:   "import-git",
			Short: "从 Git 仓库导入技能",
			RunE: func(cmd *cobra.Command, args []string) error {
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				item, err := appServices.Skills.ImportGitPath(commandContext(cmd), repositoryURL, branch, skillPath)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "skill",
					"action": "import_git",
					"item":   item,
				})
			},
		}
		importCommand.Flags().StringVar(&repositoryURL, "url", "", "https git repository url")
		importCommand.Flags().StringVar(&branch, "branch", "", "git branch")
		importCommand.Flags().StringVar(&skillPath, "path", "", "skill sub path")
		_ = importCommand.MarkFlagRequired("url")
		return importCommand
	}())

	command.AddCommand(newExternalSkillImportCommand(services, false))
	command.AddCommand(newExternalSkillImportCommand(services, true))

	command.AddCommand(func() *cobra.Command {
		var all bool
		updateCommand := &cobra.Command{
			Use:   "update [skill_name]",
			Short: "更新已导入技能",
			RunE: func(cmd *cobra.Command, args []string) error {
				if all && len(args) > 0 {
					return usageErrorf("--all 与 skill_name 不能同时使用")
				}
				if !all && len(args) != 1 {
					return usageErrorf("必须提供 skill_name，或使用 --all")
				}
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				if all {
					item, err := appServices.Skills.UpdateImportedSkills(commandContext(cmd))
					if err != nil {
						return err
					}
					return emitJSON(map[string]any{
						"domain": "skill",
						"action": "update_all",
						"item":   item,
					})
				}
				item, err := appServices.Skills.UpdateSingleSkill(commandContext(cmd), args[0])
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "skill",
					"action": "update",
					"item":   item,
				})
			},
		}
		updateCommand.Flags().BoolVar(&all, "all", false, "update all imported skills")
		return updateCommand
	}())

	return command
}
