package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
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

func newExternalSkillImportCommand(services *cliServiceProvider, install bool) *cobra.Command {
	var agentID string
	var itemJSON string
	var itemFile string
	var sourceKind string
	var importMode string
	var packageSpec string
	var skillSlug string
	var gitURL string
	var gitBranch string
	var gitPath string
	var rawURL string
	var detailURL string
	var title string
	var description string

	use := "import-external"
	short := "按外部搜索结果导入技能"
	action := "import_external"
	if install {
		use = "install-external"
		short = "按外部搜索结果导入并安装到 Agent"
		action = "install_external"
	}
	externalCommand := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			appServices, err := services.AppServices()
			if err != nil {
				return err
			}
			item, err := externalSkillItemFromCLI(
				itemJSON,
				itemFile,
				sourceKind,
				importMode,
				packageSpec,
				skillSlug,
				gitURL,
				gitBranch,
				gitPath,
				rawURL,
				detailURL,
				title,
				description,
			)
			if err != nil {
				return err
			}
			detail, err := appServices.Skills.ImportExternalSkill(commandContext(cmd), item)
			if err != nil {
				return err
			}
			var result any = detail
			if install {
				targetAgentID, resolveErr := resolveSkillInstallAgentID(cmd, appServices, agentID)
				if resolveErr != nil {
					return resolveErr
				}
				result, err = appServices.Skills.InstallSkill(commandContext(cmd), targetAgentID, detail.Name)
				if err != nil {
					return err
				}
			}
			return emitJSON(map[string]any{
				"domain": "skill",
				"action": action,
				"item":   result,
			})
		},
	}
	if install {
		externalCommand.Flags().StringVar(&agentID, "agent-id", "", "agent id，未传时尝试从 Agent workspace 推断")
	}
	externalCommand.Flags().StringVar(&itemJSON, "item-json", "", "external search item JSON")
	externalCommand.Flags().StringVar(&itemFile, "item-file", "", "external search item JSON file, or - for stdin")
	externalCommand.Flags().StringVar(&sourceKind, "source-kind", "", "source kind")
	externalCommand.Flags().StringVar(&importMode, "import-mode", "", "import mode: skills_sh, git, url")
	externalCommand.Flags().StringVar(&packageSpec, "package-spec", "", "package spec")
	externalCommand.Flags().StringVar(&skillSlug, "skill-slug", "", "skill slug")
	externalCommand.Flags().StringVar(&gitURL, "git-url", "", "git repository url")
	externalCommand.Flags().StringVar(&gitBranch, "git-branch", "", "git branch")
	externalCommand.Flags().StringVar(&gitPath, "git-path", "", "git skill path")
	externalCommand.Flags().StringVar(&rawURL, "raw-url", "", "raw SKILL.md or zip url")
	externalCommand.Flags().StringVar(&detailURL, "detail-url", "", "detail url")
	externalCommand.Flags().StringVar(&title, "title", "", "skill title")
	externalCommand.Flags().StringVar(&description, "description", "", "skill description")
	return externalCommand
}

func externalSkillItemFromCLI(
	itemJSON string,
	itemFile string,
	sourceKind string,
	importMode string,
	packageSpec string,
	skillSlug string,
	gitURL string,
	gitBranch string,
	gitPath string,
	rawURL string,
	detailURL string,
	title string,
	description string,
) (skillsvc.ExternalSkillSearchItem, error) {
	item := skillsvc.ExternalSkillSearchItem{}
	payload, err := readOptionalJSONPayload(itemJSON, itemFile)
	if err != nil {
		return item, err
	}
	if len(payload) > 0 {
		if err = json.Unmarshal(payload, &item); err != nil {
			return item, usageErrorf("external skill item JSON 格式错误: %v", err)
		}
	}
	applyString := func(target *string, value string) {
		if strings.TrimSpace(value) != "" {
			*target = strings.TrimSpace(value)
		}
	}
	applyString(&item.SourceKind, sourceKind)
	applyString(&item.ImportMode, importMode)
	applyString(&item.PackageSpec, packageSpec)
	applyString(&item.SkillSlug, skillSlug)
	applyString(&item.GitURL, gitURL)
	applyString(&item.GitBranch, gitBranch)
	applyString(&item.GitPath, gitPath)
	applyString(&item.RawURL, rawURL)
	applyString(&item.DetailURL, detailURL)
	applyString(&item.Title, title)
	applyString(&item.Description, description)
	if strings.TrimSpace(item.Name) == "" {
		item.Name = firstNonEmptyCLI(item.SkillSlug, item.Title, filepath.Base(item.GitPath), filepath.Base(item.RawURL))
	}
	if strings.TrimSpace(item.SkillSlug) == "" {
		item.SkillSlug = firstNonEmptyCLI(item.Name, item.Title)
	}
	if strings.TrimSpace(item.ImportMode) == "" && strings.TrimSpace(item.GitURL) != "" {
		item.ImportMode = "git"
	}
	if strings.TrimSpace(item.ImportMode) == "" && strings.TrimSpace(item.RawURL) != "" {
		item.ImportMode = "url"
	}
	if strings.TrimSpace(item.ImportMode) == "" && strings.TrimSpace(item.PackageSpec) != "" {
		item.ImportMode = "skills_sh"
	}
	if strings.TrimSpace(item.ImportMode) == "" && strings.TrimSpace(item.SourceKind) != "" {
		item.ImportMode = strings.TrimSpace(item.SourceKind)
	}
	if strings.TrimSpace(item.ImportMode) == "" {
		return item, usageErrorf("必须提供 --item-json/--item-file，或指定 --import-mode 与对应来源参数")
	}
	return item, nil
}

func readOptionalJSONPayload(itemJSON string, itemFile string) ([]byte, error) {
	if strings.TrimSpace(itemJSON) != "" && strings.TrimSpace(itemFile) != "" {
		return nil, usageErrorf("--item-json 与 --item-file 不能同时使用")
	}
	if strings.TrimSpace(itemJSON) != "" {
		return []byte(strings.TrimSpace(itemJSON)), nil
	}
	if strings.TrimSpace(itemFile) == "" {
		return nil, nil
	}
	if strings.TrimSpace(itemFile) == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(strings.TrimSpace(itemFile))
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

func firstNonEmptyCLI(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" && trimmed != "." {
			return trimmed
		}
	}
	return ""
}
