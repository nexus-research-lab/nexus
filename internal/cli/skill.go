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
	command.AddCommand(
		newSkillListCommand(services),
		newSkillGetCommand(services),
	)
	addSkillAgentCommands(command, services)
	addSkillSourceCommands(command, services)
	return command
}

func newSkillListCommand(services *cliServiceProvider) *cobra.Command {
	var query skillsvc.Query
	command := &cobra.Command{
		Use:   "list",
		Short: "列出技能目录",
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := skillService(services)
			if err != nil {
				return err
			}
			items, err := service.ListSkills(commandContext(cmd), query)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{"domain": "skill", "action": "list", "items": items})
		},
	}
	command.Flags().StringVar(&query.AgentID, "agent-id", "", "agent id")
	command.Flags().StringVar(&query.CategoryKey, "category", "", "category key")
	command.Flags().StringVar(&query.SourceType, "source-type", "", "source type")
	command.Flags().StringVar(&query.Q, "query", "", "search query")
	return command
}

func newSkillGetCommand(services *cliServiceProvider) *cobra.Command {
	var agentID string
	command := &cobra.Command{
		Use:   "get [skill_name]",
		Short: "读取单个技能详情",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := skillService(services)
			if err != nil {
				return err
			}
			item, err := service.GetSkillDetail(commandContext(cmd), args[0], agentID)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{"domain": "skill", "action": "get", "item": item})
		},
	}
	command.Flags().StringVar(&agentID, "agent-id", "", "agent id")
	return command
}

func skillService(services *cliServiceProvider) (*skillsvc.Service, error) {
	appServices, err := services.AppServices()
	if err != nil {
		return nil, err
	}
	return appServices.Skills, nil
}
