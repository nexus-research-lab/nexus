package tool

import (
	"context"
	"errors"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/render"
)

func find(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "find_scheduled_tasks",
		Description: "查找当前或已删除的定时任务，返回适合继续检查或管理的紧凑候选列表。缺省只查当前任务；需要追溯已删除任务时传 include_deleted=true。",
		SearchHint:  searchHintFindScheduledTasks,
		InputSchema: findSchema(),
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			includeActive := optionalBoolDefault(args, "include_active", true)
			includeDeleted := optionalBoolDefault(args, "include_deleted", false)
			if !includeActive && !includeDeleted {
				return render.Error(errors.New("find_scheduled_tasks requires include_active or include_deleted")), nil
			}
			agentID, err := resolveListAgentID(sctx, argx.String(args, "agent_id"))
			if err != nil {
				return render.Error(err), nil
			}
			requestedLimit := normalizedTaskHistoryToolLimit(argx.Int(args["limit"]))
			searchLimit := requestedLimit
			if _, filterEnabled := args["enabled"]; filterEnabled {
				// enabled 不是持久层检索条件，先取完整候选集再过滤，避免 limit 提前截断造成漏报。
				searchLimit = 50
			}
			items, err := searchTaskHistoryForToolQuery(scopedToolContext(ctx, sctx), svc, sctx, automationdomain.ScheduledTaskHistorySearchInput{
				Query:          argx.String(args, "query"),
				AgentID:        agentID,
				IncludeActive:  includeActive,
				IncludeDeleted: includeDeleted,
				Limit:          searchLimit,
			})
			if err != nil {
				return render.Error(err), nil
			}
			items = filterTaskHistoryItemsByEnabled(items, args)
			items = limitSlice(items, requestedLimit)
			return render.JSON(render.DecorateTimes(items, "")), nil
		},
	}
}

func optionalBoolDefault(args map[string]any, key string, defaultValue bool) bool {
	if args == nil {
		return defaultValue
	}
	raw, ok := args[key]
	if !ok {
		return defaultValue
	}
	return argx.ParseBool(raw)
}

func filterTaskHistoryItemsByEnabled(items []automationdomain.ScheduledTaskHistoryItem, args map[string]any) []automationdomain.ScheduledTaskHistoryItem {
	if args == nil {
		return items
	}
	raw, ok := args["enabled"]
	if !ok {
		return items
	}
	want := argx.ParseBool(raw)
	filtered := make([]automationdomain.ScheduledTaskHistoryItem, 0, len(items))
	for _, item := range items {
		if item.Enabled == nil || *item.Enabled != want {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}
