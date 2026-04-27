package cli

import (
	"github.com/spf13/cobra"

	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
)

func newConnectorCommand(service *connectorsvc.Service) *cobra.Command {
	command := &cobra.Command{
		Use:   "connector",
		Short: "connector 领域命令",
	}

	command.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "列出连接器目录",
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := service.ListConnectors(commandContext(cmd), currentCLIUserID(cmd), "", "", "")
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
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := service.GetConnectorDetail(commandContext(cmd), currentCLIUserID(cmd), args[0])
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
			Args:  exactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := service.GetAuthURL(commandContext(cmd), currentCLIUserID(cmd), args[0], redirectURI, nil)
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
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := service.Disconnect(commandContext(cmd), args[0])
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
