package cli

import (
	"github.com/spf13/cobra"

	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
)

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
				items, err := service.ListFiles(commandContext(cmd), agentID)
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
				item, err := service.GetFile(commandContext(cmd), agentID, path)
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
				item, err := service.UpdateFile(commandContext(cmd), agentID, path, content)
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
				item, err := service.CreateEntry(commandContext(cmd), agentID, path, entryType, content)
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
				item, err := service.RenameEntry(commandContext(cmd), agentID, path, newPath)
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
				item, err := service.DeleteEntry(commandContext(cmd), agentID, path)
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
