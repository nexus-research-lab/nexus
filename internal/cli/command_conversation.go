package cli

import (
	"fmt"
	roomsvc "github.com/nexus-research-lab/nexus/internal/room"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/session"
	"github.com/spf13/cobra"
)

func newConversationCommand(roomService *roomsvc.Service, sessionService *sessionsvc.Service) *cobra.Command {
	command := &cobra.Command{
		Use:   "conversation",
		Short: "conversation 领域命令",
	}

	command.AddCommand(func() *cobra.Command {
		var roomID string
		listCommand := &cobra.Command{
			Use:   "list",
			Short: "列出 Room 下的全部话题",
			RunE: func(cmd *cobra.Command, args []string) error {
				items, err := roomService.GetRoomContexts(commandContext(cmd), roomID)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "conversation",
					"action": "list",
					"items":  items,
				})
			},
		}
		listCommand.Flags().StringVar(&roomID, "room-id", "", "room id")
		_ = listCommand.MarkFlagRequired("room-id")
		return listCommand
	}())

	command.AddCommand(&cobra.Command{
		Use:   "get [conversation_id]",
		Short: "读取单个话题上下文",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := roomService.GetConversationContext(commandContext(cmd), args[0])
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "conversation",
				"action": "get",
				"item":   item,
			})
		},
	})

	command.AddCommand(func() *cobra.Command {
		var roomID string
		var title string
		createCommand := &cobra.Command{
			Use:   "create",
			Short: "创建 Room 话题",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := roomService.CreateConversation(commandContext(cmd), roomID, roomsvc.CreateConversationRequest{
					Title: title,
				})
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "conversation",
					"action": "create",
					"item":   item,
				})
			},
		}
		createCommand.Flags().StringVar(&roomID, "room-id", "", "room id")
		createCommand.Flags().StringVar(&title, "title", "", "conversation title")
		_ = createCommand.MarkFlagRequired("room-id")
		return createCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var roomID string
		var conversationID string
		var title string
		updateCommand := &cobra.Command{
			Use:   "update",
			Short: "更新 Room 话题",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := roomService.UpdateConversation(commandContext(cmd), roomID, conversationID, roomsvc.UpdateConversationRequest{
					Title: title,
				})
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "conversation",
					"action": "update",
					"item":   item,
				})
			},
		}
		updateCommand.Flags().StringVar(&roomID, "room-id", "", "room id")
		updateCommand.Flags().StringVar(&conversationID, "conversation-id", "", "conversation id")
		updateCommand.Flags().StringVar(&title, "title", "", "conversation title")
		_ = updateCommand.MarkFlagRequired("room-id")
		_ = updateCommand.MarkFlagRequired("conversation-id")
		_ = updateCommand.MarkFlagRequired("title")
		return updateCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var roomID string
		var conversationID string
		deleteCommand := &cobra.Command{
			Use:   "delete",
			Short: "删除 Room 话题",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := roomService.DeleteConversation(commandContext(cmd), roomID, conversationID)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "conversation",
					"action": "delete",
					"item":   item,
				})
			},
		}
		deleteCommand.Flags().StringVar(&roomID, "room-id", "", "room id")
		deleteCommand.Flags().StringVar(&conversationID, "conversation-id", "", "conversation id")
		_ = deleteCommand.MarkFlagRequired("room-id")
		_ = deleteCommand.MarkFlagRequired("conversation-id")
		return deleteCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var conversationID string
		messagesCommand := &cobra.Command{
			Use:   "messages",
			Short: "读取共享对话消息",
			RunE: func(cmd *cobra.Command, args []string) error {
				items, err := sessionService.GetSessionMessages(commandContext(cmd), fmt.Sprintf("room:group:%s", conversationID))
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "conversation",
					"action": "messages",
					"items":  items,
				})
			},
		}
		messagesCommand.Flags().StringVar(&conversationID, "conversation-id", "", "conversation id")
		_ = messagesCommand.MarkFlagRequired("conversation-id")
		return messagesCommand
	}())

	return command
}
