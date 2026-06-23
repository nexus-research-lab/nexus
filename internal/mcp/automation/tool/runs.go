package tool

import (
	"context"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/render"
)

func runs(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "get_scheduled_task_runs",
		Description: "按 job_id 或 query 列出最近的运行记录。query 可定位当前或已删除任务，唯一命中后再读取 run；当前会话是 DM/Room/IM 群时优先当前会话匹配，写“这里/当前会话/这个群/当前频道”会强制限定到当前会话。普通 agent 只能查看自己名下任务的记录。",
		SearchHint:  searchHintGetScheduledTaskRuns,
		InputSchema: taskHistoryJobIDSchema(),
		Annotations: &sdktool.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			scope, err := requireOwnedTaskHistoryScope(ctx, svc, sctx, args)
			if err != nil {
				return render.Error(err), nil
			}
			runs, err := svc.ListTaskRuns(scope.Context, scope.JobID)
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(render.DecorateTimes(runs, "")), nil
		},
	}
}
