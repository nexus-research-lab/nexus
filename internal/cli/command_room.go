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
	command.AddCommand(newRoomMessageCommand(services))

	command.AddCommand(func() *cobra.Command {
		var (
			agentIDs    []string
			name        string
			description string
			title       string
			skillNames  []string
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
					SkillNames:  skillNames,
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
		create.Flags().StringSliceVar(&skillNames, "skill-name", nil, "room skill name")
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
			skillNames  []string
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
				request := protocol.UpdateRoomRequest{
					Name:        name,
					Description: description,
					Title:       title,
				}
				if cmd.Flags().Changed("skill-name") {
					request.SkillNames = &skillNames
				}
				item, err := service.UpdateRoom(commandContext(cmd), args[0], request)
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
		update.Flags().StringSliceVar(&skillNames, "skill-name", nil, "room skill name")
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

	addRoomMemberCommands(command, services)
	addRoomConversationCommands(command, services)

	return command
}
