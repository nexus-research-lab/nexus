package cli

import (
	"github.com/nexus-research-lab/nexus/internal/launcher"
	"github.com/spf13/cobra"
)

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
				item, err := service.Query(commandContext(cmd), query)
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
			item, err := service.Suggestions(commandContext(cmd))
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
