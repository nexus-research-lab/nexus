package tool

import (
	"context"
	"strings"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/internal/render"
)

// status 同时生成 enable / disable 两个工具，仅 enabled 取值不同。
func status(svc contract.Service, sctx contract.ServerContext, name string, enabled bool) sdkmcp.Tool {
	description := "按 job_id 或 query 启用已暂停的定时任务。query 只在当前权限范围内唯一命中当前未删除任务时才会执行。普通 agent 只能操作自己名下的任务。"
	inputSchema := jobIDSchema()
	if !enabled {
		description = "按 job_id 或 query 停用定时任务。query 只在当前权限范围内唯一命中当前未删除任务时才会执行，多候选会要求用户确认。缺省只阻止后续触发并保留当前 active run；如果用户明确要停止当前正在跑的这次，传 cancel_active_run=true，工具会停用后调用恢复路径中断真实执行会话并把 running run 标记为 cancelled。普通 agent 只能操作自己名下的任务。"
		inputSchema = disableSchema()
	}
	return sdkmcp.Tool{
		Name:        name,
		Description: description,
		SearchHint:  searchHintScheduledTaskStatus(enabled),
		InputSchema: inputSchema,
		Handler: func(ctx context.Context, args map[string]any) (sdkmcp.ToolResult, error) {
			scope, err := requireOwnedTaskScope(ctx, svc, sctx, args)
			if err != nil {
				return render.Error(err), nil
			}
			job, err := svc.UpdateTaskStatus(scope.Context, scope.JobID, enabled)
			if err != nil {
				return render.Error(err), nil
			}
			if !enabled && argx.Bool(args, "cancel_active_run", false) {
				runID := firstNonEmptyString(
					argx.String(args, "run_id"),
					strings.TrimSpace(job.RunningRunID),
					strings.TrimSpace(scope.Job.RunningRunID),
				)
				if runID != "" {
					job, err = svc.RecoverTaskRunningRun(scope.Context, scope.JobID, runID)
					if err != nil {
						return render.Error(err), nil
					}
				}
			}
			return render.JSON(render.DecorateTimes(job, job.Schedule.Timezone)), nil
		},
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
