package tool

import (
	"context"
	"errors"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"

	"github.com/nexus-research-lab/nexus/internal/service/automation/mcp/contract"
	"github.com/nexus-research-lab/nexus/internal/service/automation/mcp/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/service/automation/mcp/internal/render"
)

func runs(svc contract.Service, sctx contract.ServerContext) agentclient.MCPTool {
	return agentclient.MCPTool{
		Name:        "get_scheduled_task_runs",
		Description: "按 job_id 列出最近的运行记录。普通 agent 只能查看自己名下任务的记录。",
		InputSchema: jobIDSchema(),
		Annotations: &agentclient.MCPToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (agentclient.MCPToolResult, error) {
			jobID := argx.String(args, "job_id")
			if jobID == "" {
				return render.Error(errors.New("job_id is required")), nil
			}
			if err := ensureJobOwnedByCaller(ctx, svc, sctx, jobID); err != nil {
				return render.Error(err), nil
			}
			runs, err := svc.ListTaskRuns(ctx, jobID)
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(render.DecorateTimes(runs, "")), nil
		},
	}
}
