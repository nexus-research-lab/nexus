package tool

import (
	"context"
	"strings"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/protocol"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/render"
)

func list(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "list_scheduled_tasks",
		Description: "列出定时任务。普通 agent 只能看到自己 agent_id 名下的任务；主智能体可传 agent_id 过滤或不传以列全部。当前会话是飞书/IM 群且未指定 agent_id/query 时，默认只列当前群相关任务。用户按名称、任务内容、投递通道/目标、执行会话、来源或状态描述任务时，可传 query 找候选；DM/Room/IM 群里的 query 会优先匹配当前会话相关任务，显式写“这里/当前会话/这个群/当前频道”会强制限定到当前会话；enabled 可筛选启用或停用任务。",
		SearchHint:  searchHintListScheduledTasks,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"agent_id": map[string]any{"type": "string"},
				"query":    map[string]any{"type": "string", "description": "可选。按 job_id/name/instruction/agent_id/投递通道/投递目标/执行会话/来源/状态模糊过滤，用于根据用户口头描述定位候选；当前外部 IM 群里不传 query 时默认只列当前群相关任务；DM/Room/IM 群里传 query 时优先当前会话匹配。"},
				"enabled":  map[string]any{"type": "boolean", "description": "可选。true 只看启用任务，false 只看停用任务。"},
			},
		},
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			filterAgentID, err := resolveListAgentID(sctx, argx.String(args, "agent_id"))
			if err != nil {
				return render.Error(err), nil
			}
			jobs, err := svc.ListTasks(scopedToolContext(ctx, sctx), filterAgentID)
			if err != nil {
				return render.Error(err), nil
			}
			jobs = filterListedTasks(jobs, args, sctx)
			return render.JSON(render.DecorateTimes(jobs, "")), nil
		},
	}
}

func filterListedTasks(jobs []automationdomain.CronJob, args map[string]any, sctx contract.ServerContext) []automationdomain.CronJob {
	query := strings.TrimSpace(argx.String(args, "query"))
	var enabledFilter *bool
	if args != nil {
		if raw, ok := args["enabled"]; ok {
			enabled := argx.ParseBool(raw)
			enabledFilter = &enabled
		}
	}
	if shouldDefaultListToCurrentExternal(args, sctx) {
		current, _ := currentExternalTaskContextFromServerContext(sctx)
		jobs = filterCronJobsByCurrentExternalContext(jobs, current)
	}
	if query != "" {
		jobs = filterCronJobsByToolQuery(jobs, query, sctx)
	}
	if enabledFilter == nil {
		return jobs
	}
	result := make([]automationdomain.CronJob, 0, len(jobs))
	for _, job := range jobs {
		if enabledFilter != nil && job.Enabled != *enabledFilter {
			continue
		}
		result = append(result, job)
	}
	return result
}

func shouldDefaultListToCurrentExternal(args map[string]any, sctx contract.ServerContext) bool {
	if strings.TrimSpace(argx.String(args, "agent_id")) != "" ||
		strings.TrimSpace(argx.String(args, "query")) != "" {
		return false
	}
	_, ok := currentExternalTaskContextFromServerContext(sctx)
	return ok
}
