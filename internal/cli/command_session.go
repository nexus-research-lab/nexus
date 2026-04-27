package cli

import (
	"github.com/spf13/cobra"

	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
)

func newSessionCommand(services *cliServiceProvider) *cobra.Command {
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
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Core.Session
				var (
					items any
				)
				if agentID != "" {
					items, err = service.ListAgentSessions(commandContext(cmd), agentID)
				} else {
					items, err = service.ListSessions(commandContext(cmd))
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
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Core.Session
				item, err := service.GetSession(commandContext(cmd), sessionKey)
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
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Core.Session
				item, err := service.CreateSession(commandContext(cmd), sessionsvc.CreateRequest{
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
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Core.Session
				item, err := service.UpdateSession(commandContext(cmd), sessionKey, sessionsvc.UpdateRequest{
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
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Core.Session
				items, err := service.GetSessionMessages(commandContext(cmd), sessionKey)
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
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Core.Session
				if err := service.DeleteSession(commandContext(cmd), sessionKey); err != nil {
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
