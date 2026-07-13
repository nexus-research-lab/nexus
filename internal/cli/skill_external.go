package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	skillsvc "github.com/nexus-research-lab/nexus/internal/service/skills"
)

func addSkillSourceCommands(parent *cobra.Command, services *cliServiceProvider) {
	parent.AddCommand(
		newSkillImportLocalCommand(services),
		newSkillSearchExternalCommand(services),
		newSkillImportGitCommand(services),
		newExternalSkillImportCommand(services, false),
		newExternalSkillImportCommand(services, true),
		newSkillUpdateCommand(services),
	)
}

func newSkillImportLocalCommand(services *cliServiceProvider) *cobra.Command {
	var path string
	command := &cobra.Command{
		Use:   "import-local",
		Short: "从本地目录导入技能",
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := skillService(services)
			if err != nil {
				return err
			}
			item, err := service.ImportLocalPath(commandContext(cmd), path)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{"domain": "skill", "action": "import_local", "item": item})
		},
	}
	command.Flags().StringVar(&path, "path", "", "skill local path")
	_ = command.MarkFlagRequired("path")
	return command
}

func newSkillSearchExternalCommand(services *cliServiceProvider) *cobra.Command {
	var query string
	var includeReadme bool
	command := &cobra.Command{
		Use:   "search-external [query]",
		Short: "搜索外部技能来源",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedQuery, err := resolveSkillSearchQuery(query, args)
			if err != nil {
				return err
			}
			service, err := skillService(services)
			if err != nil {
				return err
			}
			item, err := service.SearchExternalSkills(commandContext(cmd), resolvedQuery, includeReadme)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{"domain": "skill", "action": "search_external", "item": item})
		},
	}
	command.Flags().StringVar(&query, "query", "", "search query")
	command.Flags().BoolVar(&includeReadme, "include-readme", false, "include readme preview")
	return command
}

func resolveSkillSearchQuery(flagValue string, args []string) (string, error) {
	if len(args) > 1 {
		return "", usageErrorf("最多只能提供一个 query")
	}
	if flagValue != "" || len(args) == 0 {
		return flagValue, nil
	}
	return args[0], nil
}

func newSkillImportGitCommand(services *cliServiceProvider) *cobra.Command {
	var repositoryURL string
	var branch string
	var skillPath string
	command := &cobra.Command{
		Use:   "import-git",
		Short: "从 Git 仓库导入技能",
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := skillService(services)
			if err != nil {
				return err
			}
			item, err := service.ImportGitPath(commandContext(cmd), repositoryURL, branch, skillPath)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{"domain": "skill", "action": "import_git", "item": item})
		},
	}
	command.Flags().StringVar(&repositoryURL, "url", "", "https git repository url")
	command.Flags().StringVar(&branch, "branch", "", "git branch")
	command.Flags().StringVar(&skillPath, "path", "", "skill sub path")
	_ = command.MarkFlagRequired("url")
	return command
}

func newSkillUpdateCommand(services *cliServiceProvider) *cobra.Command {
	var all bool
	command := &cobra.Command{
		Use:   "update [skill_name]",
		Short: "更新已导入技能",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := resolveSkillUpdateTarget(all, args)
			if err != nil {
				return err
			}
			service, err := skillService(services)
			if err != nil {
				return err
			}
			if all {
				item, updateErr := service.UpdateImportedSkills(commandContext(cmd))
				return emitSkillUpdate(item, "update_all", updateErr)
			}
			item, updateErr := service.UpdateSingleSkill(commandContext(cmd), name)
			return emitSkillUpdate(item, "update", updateErr)
		},
	}
	command.Flags().BoolVar(&all, "all", false, "update all imported skills")
	return command
}

func resolveSkillUpdateTarget(all bool, args []string) (string, error) {
	if all && len(args) > 0 {
		return "", usageErrorf("--all 与 skill_name 不能同时使用")
	}
	if !all && len(args) != 1 {
		return "", usageErrorf("必须提供 skill_name，或使用 --all")
	}
	if all {
		return "", nil
	}
	return args[0], nil
}

func emitSkillUpdate(item any, action string, err error) error {
	if err != nil {
		return err
	}
	return emitJSON(map[string]any{"domain": "skill", "action": action, "item": item})
}

type externalSkillImportFlags struct {
	agentID     string
	itemJSON    string
	itemFile    string
	sourceKind  string
	importMode  string
	packageSpec string
	skillSlug   string
	gitURL      string
	gitBranch   string
	gitPath     string
	rawURL      string
	detailURL   string
	title       string
	description string
}

func newExternalSkillImportCommand(services *cliServiceProvider, install bool) *cobra.Command {
	flags := &externalSkillImportFlags{}
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			appServices, err := services.AppServices()
			if err != nil {
				return err
			}
			item, err := flags.item()
			if err != nil {
				return err
			}
			detail, err := appServices.Skills.ImportExternalSkill(commandContext(cmd), item)
			if err != nil {
				return err
			}
			var result any = detail
			if install {
				targetAgentID, resolveErr := resolveSkillInstallAgentID(cmd, appServices, flags.agentID)
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
	flags.bind(externalCommand, install)
	return externalCommand
}

func (f *externalSkillImportFlags) bind(command *cobra.Command, install bool) {
	if install {
		command.Flags().StringVar(&f.agentID, "agent-id", "", "agent id，未传时尝试从 Agent workspace 推断")
	}
	command.Flags().StringVar(&f.itemJSON, "item-json", "", "external search item JSON")
	command.Flags().StringVar(&f.itemFile, "item-file", "", "external search item JSON file, or - for stdin")
	command.Flags().StringVar(&f.sourceKind, "source-kind", "", "source kind")
	command.Flags().StringVar(&f.importMode, "import-mode", "", "import mode: skills_sh, git, url")
	command.Flags().StringVar(&f.packageSpec, "package-spec", "", "package spec")
	command.Flags().StringVar(&f.skillSlug, "skill-slug", "", "skill slug")
	command.Flags().StringVar(&f.gitURL, "git-url", "", "git repository url")
	command.Flags().StringVar(&f.gitBranch, "git-branch", "", "git branch")
	command.Flags().StringVar(&f.gitPath, "git-path", "", "git skill path")
	command.Flags().StringVar(&f.rawURL, "raw-url", "", "raw SKILL.md or zip url")
	command.Flags().StringVar(&f.detailURL, "detail-url", "", "detail url")
	command.Flags().StringVar(&f.title, "title", "", "skill title")
	command.Flags().StringVar(&f.description, "description", "", "skill description")
}

func (f externalSkillImportFlags) item() (skillsvc.ExternalSkillSearchItem, error) {
	item := skillsvc.ExternalSkillSearchItem{}
	payload, err := readOptionalJSONPayload(f.itemJSON, f.itemFile)
	if err != nil {
		return item, err
	}
	if len(payload) > 0 {
		if err = json.Unmarshal(payload, &item); err != nil {
			return item, usageErrorf("external skill item JSON 格式错误: %v", err)
		}
	}
	applyTrimmedString(&item.SourceKind, f.sourceKind)
	applyTrimmedString(&item.ImportMode, f.importMode)
	applyTrimmedString(&item.PackageSpec, f.packageSpec)
	applyTrimmedString(&item.SkillSlug, f.skillSlug)
	applyTrimmedString(&item.GitURL, f.gitURL)
	applyTrimmedString(&item.GitBranch, f.gitBranch)
	applyTrimmedString(&item.GitPath, f.gitPath)
	applyTrimmedString(&item.RawURL, f.rawURL)
	applyTrimmedString(&item.DetailURL, f.detailURL)
	applyTrimmedString(&item.Title, f.title)
	applyTrimmedString(&item.Description, f.description)
	return normalizeExternalSkillItem(item)
}

func applyTrimmedString(target *string, value string) {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		*target = trimmed
	}
}

func normalizeExternalSkillItem(item skillsvc.ExternalSkillSearchItem) (skillsvc.ExternalSkillSearchItem, error) {
	fillExternalSkillIdentity(&item)
	item.ImportMode = resolveExternalSkillImportMode(item)
	if strings.TrimSpace(item.ImportMode) == "" {
		return item, usageErrorf("必须提供 --item-json/--item-file，或指定 --import-mode 与对应来源参数")
	}
	return item, nil
}

func fillExternalSkillIdentity(item *skillsvc.ExternalSkillSearchItem) {
	if strings.TrimSpace(item.Name) == "" {
		item.Name = firstNonEmptyCLI(item.SkillSlug, item.Title, filepath.Base(item.GitPath), filepath.Base(item.RawURL))
	}
	if strings.TrimSpace(item.SkillSlug) == "" {
		item.SkillSlug = firstNonEmptyCLI(item.Name, item.Title)
	}
}

func resolveExternalSkillImportMode(item skillsvc.ExternalSkillSearchItem) string {
	if mode := strings.TrimSpace(item.ImportMode); mode != "" {
		return mode
	}
	candidates := []struct {
		value string
		mode  string
	}{
		{value: item.GitURL, mode: "git"},
		{value: item.RawURL, mode: "url"},
		{value: item.PackageSpec, mode: "skills_sh"},
		{value: item.SourceKind, mode: strings.TrimSpace(item.SourceKind)},
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.value) != "" {
			return candidate.mode
		}
	}
	return ""
}

func readOptionalJSONPayload(itemJSON string, itemFile string) ([]byte, error) {
	itemJSON = strings.TrimSpace(itemJSON)
	itemFile = strings.TrimSpace(itemFile)
	if itemJSON != "" && itemFile != "" {
		return nil, usageErrorf("--item-json 与 --item-file 不能同时使用")
	}
	if itemJSON != "" {
		return []byte(itemJSON), nil
	}
	if itemFile == "" {
		return nil, nil
	}
	if itemFile == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(itemFile)
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
