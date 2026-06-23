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
