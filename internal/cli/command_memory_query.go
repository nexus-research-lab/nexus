package cli

import (
	"context"

	memorysvc "github.com/nexus-research-lab/nexus/internal/workspace/memory"

	"github.com/spf13/cobra"
)

func newMemorySearchCommand(services memoryCommandServices) *cobra.Command {
	var query string
	var limit int
	command := &cobra.Command{
		Use:   "search",
		Short: "搜索记忆内容",
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := services.service().Search(query, limit)
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
	command.Flags().StringVar(&query, "query", "", "search query")
	command.Flags().IntVar(&limit, "limit", 20, "result limit")
	_ = command.MarkFlagRequired("query")
	return command
}

func newMemoryGetCommand(services memoryCommandServices) *cobra.Command {
	var path string
	var fromLine int
	var lines int
	command := &cobra.Command{
		Use:   "get",
		Short: "读取记忆文件片段",
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := services.service().Get(path, fromLine, lines)
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
	command.Flags().StringVar(&path, "path", "", "relative path")
	command.Flags().IntVar(&fromLine, "from_line", 1, "start line")
	command.Flags().IntVar(&lines, "lines", 50, "line count")
	_ = command.MarkFlagRequired("path")
	return command
}

func newMemoryListCommand(services memoryCommandServices) *cobra.Command {
	var limit int
	var statuses []string
	var scope string
	command := &cobra.Command{
		Use:   "list",
		Short: "列出结构化记忆条目",
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := services.engine().List(context.Background(), memorysvc.MemoryListOptions{
				Limit:    limit,
				Statuses: statuses,
				Scope:    scope,
			})
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "memory",
				"action": "list",
				"items":  items,
			})
		},
	}
	command.Flags().IntVar(&limit, "limit", 200, "result limit")
	command.Flags().StringSliceVar(&statuses, "status", nil, "status filter")
	command.Flags().StringVar(&scope, "scope", "", "scope key filter")
	return command
}

func newMemoryRecallCommand(services memoryCommandServices) *cobra.Command {
	var query string
	var limit int
	var scope memoryScopeFlags
	command := &cobra.Command{
		Use:   "recall",
		Short: "按运行时作用域召回动态记忆",
		RunE: func(cmd *cobra.Command, args []string) error {
			injection, err := services.engine().BeforeRecall(context.Background(), scope.toMemoryScope(), memorysvc.RecallRequest{
				Query:      query,
				MaxResults: limit,
			})
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain":    "memory",
				"action":    "recall",
				"injection": injection,
			})
		},
	}
	command.Flags().StringVar(&query, "query", "", "recall query")
	command.Flags().IntVar(&limit, "limit", 5, "result limit")
	addMemoryScopeFlags(command, &scope)
	_ = command.MarkFlagRequired("query")
	return command
}

func newMemoryStatsCommand(services memoryCommandServices) *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "查看记忆统计",
		RunE: func(cmd *cobra.Command, args []string) error {
			stats, err := services.engine().Stats(context.Background())
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "memory",
				"action": "stats",
				"stats":  stats,
			})
		},
	}
}

func newMemoryCleanupCommand(services memoryCommandServices) *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup",
		Short: "清理孤立的 session 摘要和 checkpoint",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := services.engine().Cleanup(context.Background())
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "memory",
				"action": "cleanup",
				"result": result,
			})
		},
	}
}

func newMemorySessionSummaryCommand(services memoryCommandServices) *cobra.Command {
	var sessionKey string
	command := &cobra.Command{
		Use:   "session-summary",
		Short: "读取会话记忆摘要",
		RunE: func(cmd *cobra.Command, args []string) error {
			summary, err := services.engine().SessionSummary(context.Background(), sessionKey)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain":  "memory",
				"action":  "session_summary",
				"summary": summary,
			})
		},
	}
	command.Flags().StringVar(&sessionKey, "session-key", "", "session key")
	_ = command.MarkFlagRequired("session-key")
	return command
}

func newMemoryReviewCommand(services memoryCommandServices) *cobra.Command {
	var days int
	var limit int
	command := &cobra.Command{
		Use:   "review",
		Short: "回顾近期记忆条目",
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := services.service().ReviewRecentEntries(days, limit)
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
	command.Flags().IntVar(&days, "days", 3, "recent days")
	command.Flags().IntVar(&limit, "limit", 8, "result limit")
	return command
}
