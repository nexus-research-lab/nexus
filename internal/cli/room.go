// INPUT: nexusctl Room 子命令参数与当前用户作用域。
// OUTPUT: Room 查询/变更结果；创建结果可携带仅用于首次进入的主持开场协议。
// POS: Nexus Agent 通过受控 CLI 管理 Room 的命令边界。
package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func newRoomCommand(services *cliServiceProvider) *cobra.Command {
	command := &cobra.Command{
		Use:   "room",
		Short: "room 领域命令",
	}
	command.AddCommand(
		newRoomListCommand(services),
		newRoomMessageCommand(services),
		newRoomCreateCommand(services),
		newRoomGetCommand(services),
		newRoomContextsCommand(services),
		newRoomEnsureDMCommand(services),
		newRoomUpdateCommand(services),
		newRoomDeleteCommand(services),
	)
	addRoomMemberCommands(command, services)
	addRoomConversationCommands(command, services)
	return command
}

func newRoomListCommand(services *cliServiceProvider) *cobra.Command {
	return &cobra.Command{
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
	}
}

func newRoomCreateCommand(services *cliServiceProvider) *cobra.Command {
	var (
		agentIDs              []string
		name                  string
		description           string
		title                 string
		avatar                string
		skillNames            []string
		hostAgentID           string
		hostAutoReplyEnabled  bool
		allowMainAgentHost    bool
		initialMessage        string
		initialTargetAgentIDs []string
	)

	create := &cobra.Command{
		Use:   "create",
		Short: "创建 Room",
		RunE: func(cmd *cobra.Command, args []string) error {
			initialMessage = strings.ReplaceAll(strings.TrimSpace(initialMessage), `\n`, "\n")
			if initialMessage != "" && hostAgentID == "" {
				return usageErrorf("--initial-message 需要同时提供 --host-agent-id")
			}
			if len(initialTargetAgentIDs) > 0 && initialMessage == "" {
				return usageErrorf("--initial-target-agent-id 需要同时提供 --initial-message")
			}
			if initialMessage != "" {
				members := make(map[string]struct{}, len(agentIDs))
				for _, agentID := range agentIDs {
					members[strings.TrimSpace(agentID)] = struct{}{}
				}
				for _, targetAgentID := range initialTargetAgentIDs {
					targetAgentID = strings.TrimSpace(targetAgentID)
					if targetAgentID == hostAgentID {
						return usageErrorf("首次主持消息不能把 Room 主持人设为执行目标")
					}
					if _, ok := members[targetAgentID]; !ok {
						return usageErrorf("首次主持消息目标必须是 Room 成员: %s", targetAgentID)
					}
				}
			}
			appServices, err := services.AppServices()
			if err != nil {
				return err
			}
			item, err := appServices.Core.Room.CreateRoom(commandContext(cmd), protocol.CreateRoomRequest{
				AgentIDs:             agentIDs,
				Name:                 name,
				Description:          description,
				Title:                title,
				Avatar:               avatar,
				SkillNames:           skillNames,
				HostAgentID:          hostAgentID,
				HostAutoReplyEnabled: hostAutoReplyEnabled,
				AllowMainAgentHost:   allowMainAgentHost,
			})
			if err != nil {
				return err
			}
			payload := map[string]any{"domain": "room", "action": "create", "item": item}
			if initialMessage != "" {
				payload["initial_message"] = initialMessage
			}
			if len(initialTargetAgentIDs) > 0 {
				payload["initial_target_agent_ids"] = initialTargetAgentIDs
			}
			return emitJSON(payload)
		},
	}
	create.Flags().StringSliceVar(&agentIDs, "agent-id", nil, "room agent ids")
	create.Flags().StringVar(&name, "name", "", "room name")
	create.Flags().StringVar(&description, "description", "", "room description")
	create.Flags().StringVar(&title, "title", "", "conversation title")
	create.Flags().StringVar(&avatar, "avatar", "", "room avatar")
	create.Flags().StringSliceVar(&skillNames, "skill-name", nil, "room skill name")
	create.Flags().StringVar(&hostAgentID, "host-agent-id", "", "room host agent id")
	create.Flags().BoolVar(&hostAutoReplyEnabled, "host-auto-reply-enabled", false, "enable room host automatic replies")
	create.Flags().BoolVar(&allowMainAgentHost, "allow-main-agent-host", false, "allow the Nexus main agent to act as room host")
	create.Flags().StringVar(&initialMessage, "initial-message", "", "scripted host message sent when the Room card is first opened")
	create.Flags().StringSliceVar(&initialTargetAgentIDs, "initial-target-agent-id", nil, "agent ids targeted by the initial host message")
	_ = create.MarkFlagRequired("agent-id")
	return create
}

func newRoomGetCommand(services *cliServiceProvider) *cobra.Command {
	return &cobra.Command{
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
	}
}

func newRoomContextsCommand(services *cliServiceProvider) *cobra.Command {
	return &cobra.Command{
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
	}
}

func newRoomEnsureDMCommand(services *cliServiceProvider) *cobra.Command {
	var agentID string
	command := &cobra.Command{
		Use:   "ensure-dm",
		Short: "获取或创建直聊 Room",
		RunE: func(cmd *cobra.Command, args []string) error {
			appServices, err := services.AppServices()
			if err != nil {
				return err
			}
			item, err := appServices.Core.Room.EnsureDirectRoom(commandContext(cmd), agentID)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{"domain": "room", "action": "ensure_dm", "item": item})
		},
	}
	command.Flags().StringVar(&agentID, "agent-id", "", "target agent id")
	_ = command.MarkFlagRequired("agent-id")
	return command
}

func newRoomUpdateCommand(services *cliServiceProvider) *cobra.Command {
	var name, description, title string
	var skillNames []string
	command := &cobra.Command{
		Use:   "update [room_id]",
		Short: "更新 Room",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appServices, err := services.AppServices()
			if err != nil {
				return err
			}
			request := protocol.UpdateRoomRequest{Name: name, Description: description, Title: title}
			if cmd.Flags().Changed("skill-name") {
				request.SkillNames = &skillNames
			}
			item, err := appServices.Core.Room.UpdateRoom(commandContext(cmd), args[0], request)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{"domain": "room", "action": "update", "item": item})
		},
	}
	command.Flags().StringVar(&name, "name", "", "room name")
	command.Flags().StringVar(&description, "description", "", "room description")
	command.Flags().StringVar(&title, "title", "", "conversation title")
	command.Flags().StringSliceVar(&skillNames, "skill-name", nil, "room skill name")
	return command
}

func newRoomDeleteCommand(services *cliServiceProvider) *cobra.Command {
	return &cobra.Command{
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
	}
}
