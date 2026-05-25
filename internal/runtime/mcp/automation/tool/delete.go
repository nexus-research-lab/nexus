package tool

import (
	"context"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/internal/render"
)

func del(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
		Name:        "delete_scheduled_task",
		Description: "按 job_id 或 query 删除定时任务。query 只在当前权限范围内唯一命中当前未删除任务时才会执行，多候选会要求用户确认。普通 agent 只能删除自己名下的任务。若删除时任务仍有 active run，服务会尝试把该 run 标记为 cancelled，返回值会包含 active_run_id / cancelled_run_id / cancelled_active_run。",
		InputSchema: jobIDSchema(),
		Annotations: &sdkmcp.ToolAnnotations{Destructive: true},
		Handler: func(ctx context.Context, args map[string]any) (sdkmcp.ToolResult, error) {
			scope, err := requireOwnedTaskScope(ctx, svc, sctx, args)
			if err != nil {
				return render.Error(err), nil
			}
			result, err := svc.DeleteTask(scope.Context, scope.JobID)
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(result), nil
		},
	}
}
