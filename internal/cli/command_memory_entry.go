package cli

import (
	"context"
	"strings"

	memorysvc "github.com/nexus-research-lab/nexus/internal/workspace/memory"

	"github.com/spf13/cobra"
)

func newMemoryAddCommand(services memoryCommandServices) *cobra.Command {
	var input memorysvc.MemoryWriteInput
	var scope memoryScopeFlags
	command := &cobra.Command{
		Use:   "add",
		Short: "手动新增候选记忆",
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := services.engine().Add(context.Background(), scope.toMemoryScope(), input)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "memory",
				"action": "add",
				"item":   item,
			})
		},
	}
	command.Flags().StringVar(&input.Kind, "kind", "LRN", "LRN | ERR | FEAT")
	command.Flags().StringVar(&input.Category, "category", "preference", "optional category")
	command.Flags().StringVar(&input.Title, "title", "", "entry title")
	command.Flags().StringVar(&input.Content, "content", "", "entry content")
	command.Flags().StringVar(&input.Status, "status", "candidate", "entry status")
	command.Flags().StringVar(&input.Priority, "priority", "medium", "entry priority")
	command.Flags().StringVar(&input.Source, "source", "manual", "entry source")
	addMemoryScopeFlags(command, &scope)
	_ = command.MarkFlagRequired("content")
	return command
}

func newMemoryUpdateCommand(services memoryCommandServices) *cobra.Command {
	var entryID string
	var input memorysvc.MemoryWriteInput
	command := &cobra.Command{
		Use:   "update",
		Short: "更新结构化记忆",
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := services.engine().Update(context.Background(), entryID, input)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "memory",
				"action": "update",
				"item":   item,
			})
		},
	}
	command.Flags().StringVar(&entryID, "entry-id", "", "entry id")
	command.Flags().StringVar(&input.Title, "title", "", "entry title")
	command.Flags().StringVar(&input.Content, "content", "", "entry content")
	command.Flags().StringVar(&input.Status, "status", "", "entry status")
	command.Flags().StringVar(&input.Priority, "priority", "", "entry priority")
	command.Flags().StringVar(&input.Source, "source", "", "entry source")
	command.Flags().StringVar(&input.Scope, "scope", "", "scope key")
	_ = command.MarkFlagRequired("entry-id")
	return command
}

func newMemoryDeleteCommand(services memoryCommandServices) *cobra.Command {
	var entryID string
	command := &cobra.Command{
		Use:   "delete",
		Short: "删除结构化记忆",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := services.engine().Delete(context.Background(), entryID); err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain":  "memory",
				"action":  "delete",
				"deleted": true,
			})
		},
	}
	command.Flags().StringVar(&entryID, "entry-id", "", "entry id")
	_ = command.MarkFlagRequired("entry-id")
	return command
}

func newMemoryIgnoreCommand(services memoryCommandServices) *cobra.Command {
	var entryID string
	var note string
	command := &cobra.Command{
		Use:   "ignore",
		Short: "忽略候选记忆",
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := services.engine().Ignore(context.Background(), entryID, note)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "memory",
				"action": "ignore",
				"item":   item,
			})
		},
	}
	command.Flags().StringVar(&entryID, "entry-id", "", "entry id")
	command.Flags().StringVar(&note, "note", "", "optional note")
	_ = command.MarkFlagRequired("entry-id")
	return command
}

func newMemoryLogCommand(services memoryCommandServices) *cobra.Command {
	var (
		kind          string
		title         string
		category      string
		promoteTarget string
		fields        []string
	)
	command := &cobra.Command{
		Use:   "log",
		Short: "向今日日记追加条目",
		RunE: func(cmd *cobra.Command, args []string) error {
			parsedFields, err := parseMemoryFields(fields)
			if err != nil {
				return err
			}
			item, err := services.service().Log(kind, title, category, parsedFields, promoteTarget)
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
	command.Flags().StringVar(&kind, "kind", "", "LRN | ERR | FEAT")
	command.Flags().StringVar(&title, "title", "", "entry title")
	command.Flags().StringVar(&category, "category", "", "optional category")
	command.Flags().StringVar(&promoteTarget, "promote-target", "", "memory|soul|tools|agents")
	command.Flags().StringSliceVar(&fields, "field", nil, "key=value")
	_ = command.MarkFlagRequired("kind")
	_ = command.MarkFlagRequired("title")
	return command
}

func newMemoryPromoteCommand(services memoryCommandServices) *cobra.Command {
	var target string
	var title string
	var content string
	var entryID string
	command := &cobra.Command{
		Use:   "promote",
		Short: "提升为长期规则",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(entryID) != "" && strings.TrimSpace(content) == "" {
				item, err := services.engine().Promote(context.Background(), entryID, target)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "memory",
					"action": "promote",
					"item":   item,
				})
			}
			if strings.TrimSpace(content) == "" {
				return usageErrorf("content 不能为空；或者提供 --entry-id 直接提升已有条目")
			}
			item, err := services.service().Promote(target, content, title, entryID)
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
	command.Flags().StringVar(&target, "target", "", "memory|soul|tools|agents")
	command.Flags().StringVar(&title, "title", "", "optional title")
	command.Flags().StringVar(&content, "content", "", "promotion content")
	command.Flags().StringVar(&entryID, "entry-id", "", "optional entry id")
	_ = command.MarkFlagRequired("target")
	return command
}

func newMemoryResolveCommand(services memoryCommandServices) *cobra.Command {
	var entryID string
	var note string
	command := &cobra.Command{
		Use:   "resolve",
		Short: "把条目标记为已解决",
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := services.service().ResolveEntry(entryID, note)
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
	command.Flags().StringVar(&entryID, "entry-id", "", "entry id")
	command.Flags().StringVar(&note, "note", "", "resolve note")
	_ = command.MarkFlagRequired("entry-id")
	_ = command.MarkFlagRequired("note")
	return command
}

func newMemorySetStatusCommand(services memoryCommandServices) *cobra.Command {
	var entryID string
	var status string
	var note string
	command := &cobra.Command{
		Use:   "set-status",
		Short: "更新条目状态",
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := services.service().SetEntryStatus(entryID, status, note)
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
	command.Flags().StringVar(&entryID, "entry-id", "", "entry id")
	command.Flags().StringVar(&status, "status", "", "target status")
	command.Flags().StringVar(&note, "note", "", "optional note")
	_ = command.MarkFlagRequired("entry-id")
	_ = command.MarkFlagRequired("status")
	return command
}
