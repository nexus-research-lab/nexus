package cli

import (
	"github.com/spf13/cobra"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func addRoomMemberCommands(command *cobra.Command, services *cliServiceProvider) {
	command.AddCommand(func() *cobra.Command {
		var agentID string
		addMember := &cobra.Command{
			Use:   "add-member [room_id]",
			Short: "向 Room 添加成员",
			Args:  exactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Core.Room
				item, err := service.AddRoomMember(commandContext(cmd), args[0], protocol.AddRoomMemberRequest{
					AgentID: agentID,
				})
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "room",
					"action": "add_member",
					"item":   item,
				})
			},
		}
		addMember.Flags().StringVar(&agentID, "agent-id", "", "agent id")
		_ = addMember.MarkFlagRequired("agent-id")
		return addMember
	}())

	command.AddCommand(func() *cobra.Command {
		var agentID string
		removeMember := &cobra.Command{
			Use:   "remove-member [room_id]",
			Short: "从 Room 移除成员",
			Args:  exactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Core.Room
				item, err := service.RemoveRoomMember(commandContext(cmd), args[0], agentID)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "room",
					"action": "remove_member",
					"item":   item,
				})
			},
		}
		removeMember.Flags().StringVar(&agentID, "agent-id", "", "agent id")
		_ = removeMember.MarkFlagRequired("agent-id")
		return removeMember
	}())
}
