package cli

import (
	"github.com/spf13/cobra"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func newRoomCommand(services *cliServiceProvider) *cobra.Command {
	command := &cobra.Command{
		Use:   "room",
		Short: "room 领域命令",
	}

	command.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "列出全部 Room",
		RunE: func(cmd *cobra.Command, args []string) error {
			appServices, err := services.AppServices()
			if err != nil {
				return err
			}
			service := appServices.Core.Room
			items, err := service.ListRooms(commandContext(cmd), 200)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "room",
				"action": "list",
				"items":  items,
			})
		},
	})

	command.AddCommand(func() *cobra.Command {
		var (
			agentIDs    []string
			name        string
			description string
			title       string
		)

		create := &cobra.Command{
			Use:   "create",
			Short: "创建 Room",
			RunE: func(cmd *cobra.Command, args []string) error {
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Core.Room
				item, err := service.CreateRoom(commandContext(cmd), protocol.CreateRoomRequest{
					AgentIDs:    agentIDs,
					Name:        name,
					Description: description,
					Title:       title,
				})
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "room",
					"action": "create",
					"item":   item,
				})
			},
		}
		create.Flags().StringSliceVar(&agentIDs, "agent-id", nil, "room agent ids")
		create.Flags().StringVar(&name, "name", "", "room name")
		create.Flags().StringVar(&description, "description", "", "room description")
		create.Flags().StringVar(&title, "title", "", "conversation title")
		_ = create.MarkFlagRequired("agent-id")
		return create
	}())

	command.AddCommand(&cobra.Command{
		Use:   "get [room_id]",
		Short: "读取指定 Room",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appServices, err := services.AppServices()
			if err != nil {
				return err
			}
			service := appServices.Core.Room
			item, err := service.GetRoom(commandContext(cmd), args[0])
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "room",
				"action": "get",
				"item":   item,
			})
		},
	})

	command.AddCommand(&cobra.Command{
		Use:   "contexts [room_id]",
		Short: "读取 Room 上下文",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appServices, err := services.AppServices()
			if err != nil {
				return err
			}
			service := appServices.Core.Room
			items, err := service.GetRoomContexts(commandContext(cmd), args[0])
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "room",
				"action": "contexts",
				"items":  items,
			})
		},
	})

	command.AddCommand(func() *cobra.Command {
		var agentID string
		ensureDM := &cobra.Command{
			Use:   "ensure-dm",
			Short: "获取或创建直聊 Room",
			RunE: func(cmd *cobra.Command, args []string) error {
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Core.Room
				item, err := service.EnsureDirectRoom(commandContext(cmd), agentID)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "room",
					"action": "ensure_dm",
					"item":   item,
				})
			},
		}
		ensureDM.Flags().StringVar(&agentID, "agent-id", "", "target agent id")
		_ = ensureDM.MarkFlagRequired("agent-id")
		return ensureDM
	}())

	command.AddCommand(func() *cobra.Command {
		var (
			name        string
			description string
			title       string
		)
		update := &cobra.Command{
			Use:   "update [room_id]",
			Short: "更新 Room",
			Args:  exactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Core.Room
				item, err := service.UpdateRoom(commandContext(cmd), args[0], protocol.UpdateRoomRequest{
					Name:        name,
					Description: description,
					Title:       title,
				})
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "room",
					"action": "update",
					"item":   item,
				})
			},
		}
		update.Flags().StringVar(&name, "name", "", "room name")
		update.Flags().StringVar(&description, "description", "", "room description")
		update.Flags().StringVar(&title, "title", "", "conversation title")
		return update
	}())

	command.AddCommand(&cobra.Command{
		Use:   "delete [room_id]",
		Short: "删除 Room",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appServices, err := services.AppServices()
			if err != nil {
				return err
			}
			service := appServices.Core.Room
			if err := service.DeleteRoom(commandContext(cmd), args[0]); err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "room",
				"action": "delete",
				"item": map[string]any{
					"success": true,
				},
			})
		},
	})

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

	return command
}
