package tool

import (
	"context"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/render"
)

func runNow(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "run_scheduled_task",
		Description: "按 job_id 或 query 立即触发一次执行（不影响后续排程，也不会重新启用已暂停任务），用于验证或紧急补跑。query 只在当前权限范围内唯一命中当前未删除任务时才会执行。普通 agent 只能触发自己名下的任务。",
		SearchHint:  searchHintRunScheduledTask,
		InputSchema: jobIDSchema(),
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			scope, err := requireOwnedTaskScope(ctx, svc, sctx, args)
			if err != nil {
				return render.Error(err), nil
			}
			result, err := svc.RunTaskNow(scope.Context, scope.JobID)
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(render.DecorateTimes(result, "")), nil
		},
	}
}
