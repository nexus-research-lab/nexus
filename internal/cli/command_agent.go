package cli

import (
	"github.com/spf13/cobra"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	agent2 "github.com/nexus-research-lab/nexus/internal/service/agent"
)

func newAgentCommand(service *agent2.Service) *cobra.Command {
	command := &cobra.Command{
		Use:   "agent",
		Short: "agent 领域命令",
	}

	command.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "列出全部 Agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := service.ListAgents(commandContext(cmd))
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "agent",
				"action": "list",
				"items":  items,
			})
		},
	})

	command.AddCommand(func() *cobra.Command {
		var (
			name        string
			avatar      string
			description string
		)

		create := &cobra.Command{
			Use:   "create",
			Short: "创建 Agent",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.CreateAgent(commandContext(cmd), protocol.CreateRequest{
					Name:        name,
					Avatar:      avatar,
					Description: description,
				})
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "agent",
					"action": "create",
					"item":   item,
				})
			},
		}
		create.Flags().StringVar(&name, "name", "", "agent name")
		create.Flags().StringVar(&avatar, "avatar", "", "agent avatar")
		create.Flags().StringVar(&description, "description", "", "agent description")
		_ = create.MarkFlagRequired("name")
		return create
	}())

	command.AddCommand(&cobra.Command{
		Use:   "get [agent_id]",
		Short: "获取指定 Agent",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := service.GetAgent(commandContext(cmd), args[0])
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "agent",
				"action": "get",
				"item":   item,
			})
		},
	})

	return command
}
