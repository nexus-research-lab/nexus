package tool

import (
	"context"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/render"
)

func recover(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "recover_scheduled_task",
		Description: "按 job_id 或 query 释放卡住的定时任务运行占用：先中断真实执行会话，再把当前未完成 run 标记为 cancelled，并恢复后续调度。query 只在当前权限范围内唯一命中当前未删除任务时才会执行。普通 agent 只能恢复自己名下任务。",
		SearchHint:  searchHintRecoverTask,
		InputSchema: recoverSchema(),
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			scope, err := requireOwnedTaskScope(ctx, svc, sctx, args)
			if err != nil {
				return render.Error(err), nil
			}
			job, err := svc.RecoverTaskRunningRun(scope.Context, scope.JobID, argx.String(args, "run_id"))
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(render.DecorateTimes(job, job.Schedule.Timezone)), nil
		},
	}
}
