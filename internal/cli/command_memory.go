package cli

import (
	"strings"

	memorysvc "github.com/nexus-research-lab/nexus/internal/memory"

	"github.com/spf13/cobra"
)

func newMemoryCommand() *cobra.Command {
	var workspacePath string
	command := &cobra.Command{
		Use:   "memory",
		Short: "memory 记忆系统命令",
	}
	command.PersistentFlags().StringVar(&workspacePath, "workspace", "", "workspace absolute path")
	_ = command.MarkPersistentFlagRequired("workspace")

	serviceFromFlags := func() *memorysvc.Service {
		return memorysvc.NewService(workspacePath)
	}

	command.AddCommand(func() *cobra.Command {
		var query string
		var limit int
		searchCommand := &cobra.Command{
			Use:   "search",
			Short: "搜索记忆内容",
			RunE: func(cmd *cobra.Command, args []string) error {
				items, err := serviceFromFlags().Search(query, limit)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "memory",
					"action": "search",
					"items":  items,
				})
			},
		}
		searchCommand.Flags().StringVar(&query, "query", "", "search query")
		searchCommand.Flags().IntVar(&limit, "limit", 20, "result limit")
		_ = searchCommand.MarkFlagRequired("query")
		return searchCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var path string
		var fromLine int
		var lines int
		getCommand := &cobra.Command{
			Use:   "get",
			Short: "读取记忆文件片段",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := serviceFromFlags().Get(path, fromLine, lines)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "memory",
					"action": "get",
					"item":   item,
				})
			},
		}
		getCommand.Flags().StringVar(&path, "path", "", "relative path")
		getCommand.Flags().IntVar(&fromLine, "from_line", 1, "start line")
		getCommand.Flags().IntVar(&lines, "lines", 50, "line count")
		_ = getCommand.MarkFlagRequired("path")
		return getCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var days int
		var limit int
		reviewCommand := &cobra.Command{
			Use:   "review",
			Short: "回顾近期记忆条目",
			RunE: func(cmd *cobra.Command, args []string) error {
				items, err := serviceFromFlags().ReviewRecentEntries(days, limit)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "memory",
					"action": "review",
					"items":  items,
				})
			},
		}
		reviewCommand.Flags().IntVar(&days, "days", 3, "recent days")
		reviewCommand.Flags().IntVar(&limit, "limit", 8, "result limit")
		return reviewCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var (
			kind          string
			title         string
			category      string
			promoteTarget string
			fields        []string
		)
		logCommand := &cobra.Command{
			Use:   "log",
			Short: "向今日日记追加条目",
			RunE: func(cmd *cobra.Command, args []string) error {
				parsedFields, err := parseMemoryFields(fields)
				if err != nil {
					return err
				}
				item, err := serviceFromFlags().Log(kind, title, category, parsedFields, promoteTarget)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "memory",
					"action": "log",
					"item":   item,
				})
			},
		}
		logCommand.Flags().StringVar(&kind, "kind", "", "LRN | ERR | FEAT | REF")
		logCommand.Flags().StringVar(&title, "title", "", "entry title")
		logCommand.Flags().StringVar(&category, "category", "", "optional category")
		logCommand.Flags().StringVar(&promoteTarget, "promote-target", "", "memory|soul|tools|agents")
		logCommand.Flags().StringSliceVar(&fields, "field", nil, "key=value")
		_ = logCommand.MarkFlagRequired("kind")
		_ = logCommand.MarkFlagRequired("title")
		return logCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var target string
		var title string
		var content string
		var entryID string
		promoteCommand := &cobra.Command{
			Use:   "promote",
			Short: "提升为长期规则",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := serviceFromFlags().Promote(target, content, title, entryID)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "memory",
					"action": "promote",
					"item":   item,
				})
			},
		}
		promoteCommand.Flags().StringVar(&target, "target", "", "memory|soul|tools|agents")
		promoteCommand.Flags().StringVar(&title, "title", "", "optional title")
		promoteCommand.Flags().StringVar(&content, "content", "", "promotion content")
		promoteCommand.Flags().StringVar(&entryID, "entry-id", "", "optional entry id")
		_ = promoteCommand.MarkFlagRequired("target")
		_ = promoteCommand.MarkFlagRequired("content")
		return promoteCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var entryID string
		var note string
		resolveCommand := &cobra.Command{
			Use:   "resolve",
			Short: "把条目标记为已解决",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := serviceFromFlags().ResolveEntry(entryID, note)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "memory",
					"action": "resolve",
					"item":   item,
				})
			},
		}
		resolveCommand.Flags().StringVar(&entryID, "entry-id", "", "entry id")
		resolveCommand.Flags().StringVar(&note, "note", "", "resolve note")
		_ = resolveCommand.MarkFlagRequired("entry-id")
		_ = resolveCommand.MarkFlagRequired("note")
		return resolveCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var entryID string
		var status string
		var note string
		statusCommand := &cobra.Command{
			Use:   "set-status",
			Short: "更新条目状态",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := serviceFromFlags().SetEntryStatus(entryID, status, note)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "memory",
					"action": "set_status",
					"item":   item,
				})
			},
		}
		statusCommand.Flags().StringVar(&entryID, "entry-id", "", "entry id")
		statusCommand.Flags().StringVar(&status, "status", "", "target status")
		statusCommand.Flags().StringVar(&note, "note", "", "optional note")
		_ = statusCommand.MarkFlagRequired("entry-id")
		_ = statusCommand.MarkFlagRequired("status")
		return statusCommand
	}())

	return command
}

func parseMemoryFields(values []string) ([]memorysvc.Field, error) {
	items := make([]memorysvc.Field, 0, len(values))
	for _, value := range values {
		parts := strings.SplitN(value, "=", 2)
		if len(parts) != 2 {
			return nil, usageErrorf("field 格式错误: %s", value)
		}
		items = append(items, memorysvc.Field{
			Key:   strings.TrimSpace(parts[0]),
			Value: strings.TrimSpace(parts[1]),
		})
	}
	return items, nil
}
