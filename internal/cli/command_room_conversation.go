package cli

import (
	"github.com/spf13/cobra"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func addRoomConversationCommands(command *cobra.Command, services *cliServiceProvider) {
	command.AddCommand(func() *cobra.Command {
		var title string
		createConversation := &cobra.Command{
			Use:   "create-conversation [room_id]",
			Short: "创建 Room 话题",
			Args:  exactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Core.Room
				item, err := service.CreateConversation(commandContext(cmd), args[0], protocol.CreateConversationRequest{
					Title: title,
				})
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "room",
					"action": "create_conversation",
					"item":   item,
				})
			},
		}
		createConversation.Flags().StringVar(&title, "title", "", "conversation title")
		return createConversation
	}())

	command.AddCommand(func() *cobra.Command {
		var title string
		updateConversation := &cobra.Command{
			Use:   "update-conversation [room_id] [conversation_id]",
			Short: "更新 Room 话题",
			Args:  exactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Core.Room
				item, err := service.UpdateConversation(commandContext(cmd), args[0], args[1], protocol.UpdateConversationRequest{
					Title: title,
				})
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "room",
					"action": "update_conversation",
					"item":   item,
				})
			},
		}
		updateConversation.Flags().StringVar(&title, "title", "", "conversation title")
		_ = updateConversation.MarkFlagRequired("title")
		return updateConversation
	}())

	command.AddCommand(&cobra.Command{
		Use:   "delete-conversation [room_id] [conversation_id]",
		Short: "删除 Room 话题",
		Args:  exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			appServices, err := services.AppServices()
			if err != nil {
				return err
			}
			service := appServices.Core.Room
			item, err := service.DeleteConversation(commandContext(cmd), args[0], args[1])
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "room",
				"action": "delete_conversation",
				"item":   item,
			})
		},
	})
}
