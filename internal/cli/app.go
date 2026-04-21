// =====================================================
// @File   ：app.go
// @Date   ：2026/04/10 21:22:41
// @Author ：leemysw
// 2026/04/10 21:22:41   Create
// =====================================================

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	agent2 "github.com/nexus-research-lab/nexus/internal/agent"
	"github.com/nexus-research-lab/nexus/internal/bootstrap"
	"github.com/nexus-research-lab/nexus/internal/config"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/connectors"
	"github.com/nexus-research-lab/nexus/internal/launcher"
	roomsvc "github.com/nexus-research-lab/nexus/internal/room"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/session"
	skillsvc "github.com/nexus-research-lab/nexus/internal/skills"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/workspace"

	"github.com/spf13/cobra"
)

// New 创建 CLI 应用。
func New(cfg config.Config) (*cobra.Command, error) {
	appServices, err := bootstrap.NewAppServices(cfg, nil)
	if err != nil {
		return nil, err
	}
	agentService := appServices.Core.Agent
	roomService := appServices.Core.Room
	sessionService := appServices.Core.Session
	authService := appServices.Auth
	workspaceService := appServices.Workspace
	skillService := appServices.Skills
	connectorService := appServices.Connectors
	launcherService := appServices.Launcher
	ingressService := appServices.Ingress
	automationService := appServices.Automation

	root := &cobra.Command{
		Use:   "nexusctl",
		Short: "Nexus 主智能体操作系统 CLI",
	}

	root.AddCommand(newAgentCommand(agentService))
	root.AddCommand(newAuthCommand(authService))
	root.AddCommand(newUserCommand(authService))
	root.AddCommand(newRoomCommand(roomService))
	root.AddCommand(newConversationCommand(roomService, sessionService))
	root.AddCommand(newSessionCommand(sessionService))
	root.AddCommand(newWorkspaceCommand(workspaceService))
	root.AddCommand(newSkillCommand(skillService))
	root.AddCommand(newConnectorCommand(connectorService))
	root.AddCommand(newLauncherCommand(launcherService))
	root.AddCommand(newChannelCommand(ingressService))
	root.AddCommand(newAutomationCommand(automationService))
	root.AddCommand(newMemoryCommand())

	return root, nil
}

func newAgentCommand(service *agent2.Service) *cobra.Command {
	command := &cobra.Command{
		Use:   "agent",
		Short: "agent 领域命令",
	}

	command.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "列出全部 Agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := service.ListAgents(context.Background())
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
				item, err := service.CreateAgent(context.Background(), agent2.CreateRequest{
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
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := service.GetAgent(context.Background(), args[0])
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

func newRoomCommand(service *roomsvc.Service) *cobra.Command {
	command := &cobra.Command{
		Use:   "room",
		Short: "room 领域命令",
	}

	command.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "列出全部 Room",
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := service.ListRooms(context.Background(), 200)
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
				item, err := service.CreateRoom(context.Background(), roomsvc.CreateRoomRequest{
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
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := service.GetRoom(context.Background(), args[0])
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
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := service.GetRoomContexts(context.Background(), args[0])
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
				item, err := service.EnsureDirectRoom(context.Background(), agentID)
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
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.UpdateRoom(context.Background(), args[0], roomsvc.UpdateRoomRequest{
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
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := service.DeleteRoom(context.Background(), args[0]); err != nil {
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
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.AddRoomMember(context.Background(), args[0], roomsvc.AddRoomMemberRequest{
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
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.RemoveRoomMember(context.Background(), args[0], agentID)
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
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.CreateConversation(context.Background(), args[0], roomsvc.CreateConversationRequest{
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
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.UpdateConversation(context.Background(), args[0], args[1], roomsvc.UpdateConversationRequest{
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
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := service.DeleteConversation(context.Background(), args[0], args[1])
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

func newLauncherCommand(service *launcher.Service) *cobra.Command {
	command := &cobra.Command{
		Use:   "launcher",
		Short: "launcher 领域命令",
	}

	command.AddCommand(func() *cobra.Command {
		var query string
		queryCommand := &cobra.Command{
			Use:   "query",
			Short: "解析 Launcher 查询",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.Query(context.Background(), query)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "launcher",
					"action": "query",
					"item":   item,
				})
			},
		}
		queryCommand.Flags().StringVar(&query, "query", "", "launcher query")
		_ = queryCommand.MarkFlagRequired("query")
		return queryCommand
	}())

	command.AddCommand(&cobra.Command{
		Use:   "suggestions",
		Short: "读取 Launcher 推荐列表",
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := service.Suggestions(context.Background())
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "launcher",
				"action": "suggestions",
				"item":   item,
			})
		},
	})

	return command
}

func newSessionCommand(service *sessionsvc.Service) *cobra.Command {
	command := &cobra.Command{
		Use:   "session",
		Short: "session 领域命令",
	}

	command.AddCommand(func() *cobra.Command {
		var agentID string
		listCommand := &cobra.Command{
			Use:   "list",
			Short: "列出会话",
			RunE: func(cmd *cobra.Command, args []string) error {
				var (
					items any
					err   error
				)
				if agentID != "" {
					items, err = service.ListAgentSessions(context.Background(), agentID)
				} else {
					items, err = service.ListSessions(context.Background())
				}
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "session",
					"action": "list",
					"items":  items,
				})
			},
		}
		listCommand.Flags().StringVar(&agentID, "agent-id", "", "filter by agent id")
		return listCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var sessionKey string
		getCommand := &cobra.Command{
			Use:   "get",
			Short: "读取单个会话",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.GetSession(context.Background(), sessionKey)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "session",
					"action": "get",
					"item":   item,
				})
			},
		}
		getCommand.Flags().StringVar(&sessionKey, "session-key", "", "structured session key")
		_ = getCommand.MarkFlagRequired("session-key")
		return getCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var (
			sessionKey string
			agentID    string
			title      string
		)
		createCommand := &cobra.Command{
			Use:   "create",
			Short: "创建会话",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.CreateSession(context.Background(), sessionsvc.CreateRequest{
					SessionKey: sessionKey,
					AgentID:    agentID,
					Title:      title,
				})
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "session",
					"action": "create",
					"item":   item,
				})
			},
		}
		createCommand.Flags().StringVar(&sessionKey, "session-key", "", "structured session key")
		createCommand.Flags().StringVar(&agentID, "agent-id", "", "agent id override")
		createCommand.Flags().StringVar(&title, "title", "", "session title")
		_ = createCommand.MarkFlagRequired("session-key")
		return createCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var (
			sessionKey string
			title      string
		)
		updateCommand := &cobra.Command{
			Use:   "update",
			Short: "更新会话标题",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.UpdateSession(context.Background(), sessionKey, sessionsvc.UpdateRequest{
					Title: &title,
				})
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "session",
					"action": "update",
					"item":   item,
				})
			},
		}
		updateCommand.Flags().StringVar(&sessionKey, "session-key", "", "structured session key")
		updateCommand.Flags().StringVar(&title, "title", "", "session title")
		_ = updateCommand.MarkFlagRequired("session-key")
		return updateCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var sessionKey string
		messagesCommand := &cobra.Command{
			Use:   "messages",
			Short: "读取会话消息",
			RunE: func(cmd *cobra.Command, args []string) error {
				items, err := service.GetSessionMessages(context.Background(), sessionKey)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "session",
					"action": "messages",
					"items":  items,
				})
			},
		}
		messagesCommand.Flags().StringVar(&sessionKey, "session-key", "", "structured session key")
		_ = messagesCommand.MarkFlagRequired("session-key")
		return messagesCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var sessionKey string
		deleteCommand := &cobra.Command{
			Use:   "delete",
			Short: "删除会话",
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := service.DeleteSession(context.Background(), sessionKey); err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "session",
					"action": "delete",
					"item": map[string]any{
						"success": true,
					},
				})
			},
		}
		deleteCommand.Flags().StringVar(&sessionKey, "session-key", "", "structured session key")
		_ = deleteCommand.MarkFlagRequired("session-key")
		return deleteCommand
	}())

	return command
}

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
				items, err := roomService.GetRoomContexts(context.Background(), roomID)
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
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := roomService.GetConversationContext(context.Background(), args[0])
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
				item, err := roomService.CreateConversation(context.Background(), roomID, roomsvc.CreateConversationRequest{
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
				item, err := roomService.UpdateConversation(context.Background(), roomID, conversationID, roomsvc.UpdateConversationRequest{
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
				item, err := roomService.DeleteConversation(context.Background(), roomID, conversationID)
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
				items, err := sessionService.GetSessionMessages(context.Background(), fmt.Sprintf("room:group:%s", conversationID))
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

func newWorkspaceCommand(service *workspacepkg.Service) *cobra.Command {
	command := &cobra.Command{
		Use:   "workspace",
		Short: "workspace 领域命令",
	}

	command.AddCommand(func() *cobra.Command {
		var agentID string
		listCommand := &cobra.Command{
			Use:   "list",
			Short: "列出工作区文件",
			RunE: func(cmd *cobra.Command, args []string) error {
				items, err := service.ListFiles(context.Background(), agentID)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "workspace",
					"action": "list",
					"items":  items,
				})
			},
		}
		listCommand.Flags().StringVar(&agentID, "agent-id", "", "agent id")
		_ = listCommand.MarkFlagRequired("agent-id")
		return listCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var agentID string
		var path string
		getCommand := &cobra.Command{
			Use:   "get",
			Short: "读取工作区文件",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.GetFile(context.Background(), agentID, path)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "workspace",
					"action": "get",
					"item":   item,
				})
			},
		}
		getCommand.Flags().StringVar(&agentID, "agent-id", "", "agent id")
		getCommand.Flags().StringVar(&path, "path", "", "relative path")
		_ = getCommand.MarkFlagRequired("agent-id")
		_ = getCommand.MarkFlagRequired("path")
		return getCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var agentID string
		var path string
		var content string
		updateCommand := &cobra.Command{
			Use:   "update",
			Short: "更新工作区文件",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.UpdateFile(context.Background(), agentID, path, content)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "workspace",
					"action": "update",
					"item":   item,
				})
			},
		}
		updateCommand.Flags().StringVar(&agentID, "agent-id", "", "agent id")
		updateCommand.Flags().StringVar(&path, "path", "", "relative path")
		updateCommand.Flags().StringVar(&content, "content", "", "file content")
		_ = updateCommand.MarkFlagRequired("agent-id")
		_ = updateCommand.MarkFlagRequired("path")
		return updateCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var agentID string
		var path string
		var entryType string
		var content string
		createCommand := &cobra.Command{
			Use:   "create",
			Short: "创建工作区条目",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.CreateEntry(context.Background(), agentID, path, entryType, content)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "workspace",
					"action": "create",
					"item":   item,
				})
			},
		}
		createCommand.Flags().StringVar(&agentID, "agent-id", "", "agent id")
		createCommand.Flags().StringVar(&path, "path", "", "relative path")
		createCommand.Flags().StringVar(&entryType, "type", "file", "file or directory")
		createCommand.Flags().StringVar(&content, "content", "", "file content")
		_ = createCommand.MarkFlagRequired("agent-id")
		_ = createCommand.MarkFlagRequired("path")
		return createCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var agentID string
		var path string
		var newPath string
		renameCommand := &cobra.Command{
			Use:   "rename",
			Short: "重命名工作区条目",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.RenameEntry(context.Background(), agentID, path, newPath)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "workspace",
					"action": "rename",
					"item":   item,
				})
			},
		}
		renameCommand.Flags().StringVar(&agentID, "agent-id", "", "agent id")
		renameCommand.Flags().StringVar(&path, "path", "", "relative path")
		renameCommand.Flags().StringVar(&newPath, "new-path", "", "new relative path")
		_ = renameCommand.MarkFlagRequired("agent-id")
		_ = renameCommand.MarkFlagRequired("path")
		_ = renameCommand.MarkFlagRequired("new-path")
		return renameCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var agentID string
		var path string
		deleteCommand := &cobra.Command{
			Use:   "delete",
			Short: "删除工作区条目",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.DeleteEntry(context.Background(), agentID, path)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "workspace",
					"action": "delete",
					"item":   item,
				})
			},
		}
		deleteCommand.Flags().StringVar(&agentID, "agent-id", "", "agent id")
		deleteCommand.Flags().StringVar(&path, "path", "", "relative path")
		_ = deleteCommand.MarkFlagRequired("agent-id")
		_ = deleteCommand.MarkFlagRequired("path")
		return deleteCommand
	}())

	return command
}

func newSkillCommand(service *skillsvc.Service) *cobra.Command {
	command := &cobra.Command{
		Use:   "skill",
		Short: "skill 领域命令",
	}

	command.AddCommand(func() *cobra.Command {
		var agentID string
		var categoryKey string
		var sourceType string
		var query string
		listCommand := &cobra.Command{
			Use:   "list",
			Short: "列出技能目录",
			RunE: func(cmd *cobra.Command, args []string) error {
				items, err := service.ListSkills(context.Background(), skillsvc.Query{
					AgentID:     agentID,
					CategoryKey: categoryKey,
					SourceType:  sourceType,
					Q:           query,
				})
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "skill",
					"action": "list",
					"items":  items,
				})
			},
		}
		listCommand.Flags().StringVar(&agentID, "agent-id", "", "agent id")
		listCommand.Flags().StringVar(&categoryKey, "category", "", "category key")
		listCommand.Flags().StringVar(&sourceType, "source-type", "", "source type")
		listCommand.Flags().StringVar(&query, "query", "", "search query")
		return listCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var agentID string
		getCommand := &cobra.Command{
			Use:   "get [skill_name]",
			Short: "读取单个技能详情",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.GetSkillDetail(context.Background(), args[0], agentID)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "skill",
					"action": "get",
					"item":   item,
				})
			},
		}
		getCommand.Flags().StringVar(&agentID, "agent-id", "", "agent id")
		return getCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var agentID string
		listCommand := &cobra.Command{
			Use:   "agent-list",
			Short: "列出 Agent 已可见技能",
			RunE: func(cmd *cobra.Command, args []string) error {
				items, err := service.GetAgentSkills(context.Background(), agentID)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "skill",
					"action": "agent_list",
					"items":  items,
				})
			},
		}
		listCommand.Flags().StringVar(&agentID, "agent-id", "", "agent id")
		_ = listCommand.MarkFlagRequired("agent-id")
		return listCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var agentID string
		var skillName string
		installCommand := &cobra.Command{
			Use:   "install",
			Short: "为 Agent 安装技能",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.InstallSkill(context.Background(), agentID, skillName)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "skill",
					"action": "install",
					"item":   item,
				})
			},
		}
		installCommand.Flags().StringVar(&agentID, "agent-id", "", "agent id")
		installCommand.Flags().StringVar(&skillName, "skill-name", "", "skill name")
		_ = installCommand.MarkFlagRequired("agent-id")
		_ = installCommand.MarkFlagRequired("skill-name")
		return installCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var agentID string
		var skillName string
		uninstallCommand := &cobra.Command{
			Use:   "uninstall",
			Short: "从 Agent 卸载技能",
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := service.UninstallSkill(context.Background(), agentID, skillName); err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "skill",
					"action": "uninstall",
					"item": map[string]any{
						"success": true,
					},
				})
			},
		}
		uninstallCommand.Flags().StringVar(&agentID, "agent-id", "", "agent id")
		uninstallCommand.Flags().StringVar(&skillName, "skill-name", "", "skill name")
		_ = uninstallCommand.MarkFlagRequired("agent-id")
		_ = uninstallCommand.MarkFlagRequired("skill-name")
		return uninstallCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var path string
		importCommand := &cobra.Command{
			Use:   "import-local",
			Short: "从本地目录导入技能",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.ImportLocalPath(path)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "skill",
					"action": "import_local",
					"item":   item,
				})
			},
		}
		importCommand.Flags().StringVar(&path, "path", "", "skill local path")
		_ = importCommand.MarkFlagRequired("path")
		return importCommand
	}())

	return command
}

func newConnectorCommand(service *connectorsvc.Service) *cobra.Command {
	command := &cobra.Command{
		Use:   "connector",
		Short: "connector 领域命令",
	}

	command.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "列出连接器目录",
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := service.ListConnectors(context.Background(), "", "", "")
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "connector",
				"action": "list",
				"items":  items,
			})
		},
	})

	command.AddCommand(&cobra.Command{
		Use:   "get [connector_id]",
		Short: "读取单个连接器详情",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := service.GetConnectorDetail(context.Background(), args[0])
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "connector",
				"action": "get",
				"item":   item,
			})
		},
	})

	command.AddCommand(func() *cobra.Command {
		var redirectURI string
		authURLCommand := &cobra.Command{
			Use:   "auth-url [connector_id]",
			Short: "生成 OAuth 授权地址",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.GetAuthURL(context.Background(), args[0], redirectURI)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "connector",
					"action": "auth_url",
					"item":   item,
				})
			},
		}
		authURLCommand.Flags().StringVar(&redirectURI, "redirect-uri", "", "oauth redirect uri")
		return authURLCommand
	}())

	command.AddCommand(&cobra.Command{
		Use:   "disconnect [connector_id]",
		Short: "断开连接器",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := service.Disconnect(context.Background(), args[0])
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "connector",
				"action": "disconnect",
				"item":   item,
			})
		},
	})

	return command
}

func emitJSON(payload map[string]any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}
