package tool

import (
	"context"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/render"
)

func searchHistory(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "search_scheduled_task_history",
		Description: "按名称、job_id、任务内容、投递目标、来源、动作或审计 detail 搜索当前和已删除的定时任务候选，返回可继续传给 get_scheduled_task_runs / get_scheduled_task_daily_report / get_scheduled_task_events 的 job_id。当前会话是 DM/Room/IM 群时，query 会优先匹配当前会话任务；显式写“这里/当前会话/这个群/当前频道”会强制限定到当前会话。适合用户只说“那个新闻日报/删掉的任务/发到飞书群的任务/之前的发送记录”时先定位任务。",
		SearchHint:  searchHintSearchTaskHistory,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":           map[string]any{"type": "string", "description": "按 job_id/name/instruction/delivery/source/action/detail 模糊搜索；可填用户口头提到的任务名、内容或投递目标关键词；当前会话是 DM/Room/IM 群时优先当前会话匹配，“这里/当前会话/这个群/当前频道”会强制限定到当前会话"},
				"agent_id":        map[string]any{"type": "string", "description": "主智能体可填：只搜索某个智能体的任务历史；普通 agent 会被强制限定为自己"},
				"include_active":  map[string]any{"type": "boolean", "description": "是否包含当前仍存在的任务；缺省 true"},
				"include_deleted": map[string]any{"type": "boolean", "description": "是否包含已删除任务的历史候选；缺省 true"},
				"limit":           map[string]any{"type": "integer", "description": "返回候选数，缺省 20，最大 50"},
			},
		},
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			agentID, err := resolveListAgentID(sctx, argx.String(args, "agent_id"))
			if err != nil {
				return render.Error(err), nil
			}
			input := automationdomain.CronTaskHistorySearchInput{
				Query:          argx.String(args, "query"),
				AgentID:        agentID,
				IncludeActive:  optionalBoolDefault(args, "include_active", true),
				IncludeDeleted: optionalBoolDefault(args, "include_deleted", true),
				Limit:          argx.Int(args["limit"]),
			}
			items, err := searchTaskHistoryForToolQuery(scopedToolContext(ctx, sctx), svc, sctx, input)
			if err != nil {
				return render.Error(err), nil
			}
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
