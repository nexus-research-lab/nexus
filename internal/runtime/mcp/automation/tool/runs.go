package tool

import (
	"context"
	"errors"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/internal/render"
)

func runs(svc contract.Service, sctx contract.ServerContext) sdkmcp.Tool {
	return sdkmcp.Tool{
		Name:        "get_scheduled_task_runs",
		Description: "按 job_id 列出最近的运行记录。普通 agent 只能查看自己名下任务的记录。",
		SearchHint:  searchHintGetScheduledTaskRuns,
		InputSchema: jobIDSchema(),
		Annotations: &sdkmcp.ToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (sdkmcp.ToolResult, error) {
			jobID := argx.String(args, "job_id")
			if jobID == "" {
				return render.Error(errors.New("job_id is required")), nil
			}
			if err := ensureJobOwnedByCaller(ctx, svc, sctx, jobID); err != nil {
				return render.Error(err), nil
			}
			runs, err := svc.ListTaskRuns(scopedToolContext(ctx, sctx), jobID)
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(render.DecorateTimes(runs, "")), nil
		},
	}
}
