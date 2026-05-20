package cli

import (
	"context"
	"strings"

	memorysvc "github.com/nexus-research-lab/nexus/internal/workspace/memory"

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
	engineFromFlags := func() *memorysvc.Engine {
		return memorysvc.NewEngine(workspacePath, memorysvc.DefaultOptions())
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
		var limit int
		var statuses []string
		var scope string
		listCommand := &cobra.Command{
			Use:   "list",
			Short: "列出结构化记忆条目",
			RunE: func(cmd *cobra.Command, args []string) error {
				items, err := engineFromFlags().List(context.Background(), memorysvc.MemoryListOptions{
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
		listCommand.Flags().IntVar(&limit, "limit", 200, "result limit")
		listCommand.Flags().StringSliceVar(&statuses, "status", nil, "status filter")
		listCommand.Flags().StringVar(&scope, "scope", "", "scope key filter")
		return listCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var query string
		var limit int
		var scope memoryScopeFlags
		recallCommand := &cobra.Command{
			Use:   "recall",
			Short: "按运行时作用域召回动态记忆",
			RunE: func(cmd *cobra.Command, args []string) error {
				injection, err := engineFromFlags().BeforeRecall(context.Background(), scope.toMemoryScope(), memorysvc.RecallRequest{
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
		recallCommand.Flags().StringVar(&query, "query", "", "recall query")
		recallCommand.Flags().IntVar(&limit, "limit", 5, "result limit")
		addMemoryScopeFlags(recallCommand, &scope)
		_ = recallCommand.MarkFlagRequired("query")
		return recallCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var input memorysvc.MemoryWriteInput
		var scope memoryScopeFlags
		addCommand := &cobra.Command{
			Use:   "add",
			Short: "手动新增候选记忆",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := engineFromFlags().Add(context.Background(), scope.toMemoryScope(), input)
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
		addCommand.Flags().StringVar(&input.Kind, "kind", "LRN", "LRN | ERR | FEAT | REF")
		addCommand.Flags().StringVar(&input.Category, "category", "preference", "optional category")
		addCommand.Flags().StringVar(&input.Title, "title", "", "entry title")
		addCommand.Flags().StringVar(&input.Content, "content", "", "entry content")
		addCommand.Flags().StringVar(&input.Status, "status", "candidate", "entry status")
		addCommand.Flags().StringVar(&input.Priority, "priority", "medium", "entry priority")
		addCommand.Flags().StringVar(&input.Source, "source", "manual", "entry source")
		addMemoryScopeFlags(addCommand, &scope)
		_ = addCommand.MarkFlagRequired("content")
		return addCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var entryID string
		var input memorysvc.MemoryWriteInput
		updateCommand := &cobra.Command{
			Use:   "update",
			Short: "更新结构化记忆",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := engineFromFlags().Update(context.Background(), entryID, input)
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
		updateCommand.Flags().StringVar(&entryID, "entry-id", "", "entry id")
		updateCommand.Flags().StringVar(&input.Title, "title", "", "entry title")
		updateCommand.Flags().StringVar(&input.Content, "content", "", "entry content")
		updateCommand.Flags().StringVar(&input.Status, "status", "", "entry status")
		updateCommand.Flags().StringVar(&input.Priority, "priority", "", "entry priority")
		updateCommand.Flags().StringVar(&input.Source, "source", "", "entry source")
		updateCommand.Flags().StringVar(&input.Scope, "scope", "", "scope key")
		_ = updateCommand.MarkFlagRequired("entry-id")
		return updateCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var entryID string
		deleteCommand := &cobra.Command{
			Use:   "delete",
			Short: "删除结构化记忆",
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := engineFromFlags().Delete(context.Background(), entryID); err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain":  "memory",
					"action":  "delete",
					"deleted": true,
				})
			},
		}
		deleteCommand.Flags().StringVar(&entryID, "entry-id", "", "entry id")
		_ = deleteCommand.MarkFlagRequired("entry-id")
		return deleteCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var entryID string
		var note string
		ignoreCommand := &cobra.Command{
			Use:   "ignore",
			Short: "忽略候选记忆",
			RunE: func(cmd *cobra.Command, args []string) error {
				item, err := engineFromFlags().Ignore(context.Background(), entryID, note)
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
		ignoreCommand.Flags().StringVar(&entryID, "entry-id", "", "entry id")
		ignoreCommand.Flags().StringVar(&note, "note", "", "optional note")
		_ = ignoreCommand.MarkFlagRequired("entry-id")
		return ignoreCommand
	}())

	command.AddCommand(func() *cobra.Command {
		statsCommand := &cobra.Command{
			Use:   "stats",
			Short: "查看记忆统计",
			RunE: func(cmd *cobra.Command, args []string) error {
				stats, err := engineFromFlags().Stats(context.Background())
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
		return statsCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var sessionKey string
		summaryCommand := &cobra.Command{
			Use:   "session-summary",
			Short: "读取会话记忆摘要",
			RunE: func(cmd *cobra.Command, args []string) error {
				summary, err := engineFromFlags().SessionSummary(context.Background(), sessionKey)
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
		summaryCommand.Flags().StringVar(&sessionKey, "session-key", "", "session key")
		_ = summaryCommand.MarkFlagRequired("session-key")
		return summaryCommand
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
				if strings.TrimSpace(entryID) != "" && strings.TrimSpace(content) == "" {
					item, err := engineFromFlags().Promote(context.Background(), entryID, target)
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

type memoryScopeFlags struct {
	Kind           string
	UserID         string
	AgentID        string
	SessionKey     string
	SessionID      string
	RoomID         string
	ConversationID string
}

func addMemoryScopeFlags(command *cobra.Command, scope *memoryScopeFlags) {
	command.Flags().StringVar(&scope.Kind, "scope-kind", string(memorysvc.ScopeKindAgent), "user|agent|dm_session|room_shared|room_agent_session")
	command.Flags().StringVar(&scope.UserID, "user-id", "", "owner user id")
	command.Flags().StringVar(&scope.AgentID, "agent-id", "", "agent id")
	command.Flags().StringVar(&scope.SessionKey, "session-key", "", "session key")
	command.Flags().StringVar(&scope.SessionID, "session-id", "", "runtime session id")
	command.Flags().StringVar(&scope.RoomID, "room-id", "", "room id")
	command.Flags().StringVar(&scope.ConversationID, "conversation-id", "", "conversation id")
}

func (s memoryScopeFlags) toMemoryScope() memorysvc.MemoryScope {
	return memorysvc.MemoryScope{
		Kind:           memorysvc.ScopeKind(strings.TrimSpace(s.Kind)),
		UserID:         strings.TrimSpace(s.UserID),
		AgentID:        strings.TrimSpace(s.AgentID),
		SessionKey:     strings.TrimSpace(s.SessionKey),
		SessionID:      strings.TrimSpace(s.SessionID),
		RoomID:         strings.TrimSpace(s.RoomID),
		ConversationID: strings.TrimSpace(s.ConversationID),
	}
}
