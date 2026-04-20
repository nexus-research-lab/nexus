package tool

import (
	"context"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/automation/mcp/contract"
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/internal/render"
	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
)

func runs(svc contract.Service) agentclient.MCPTool {
	return agentclient.MCPTool{
		Name:        "get_scheduled_task_runs",
		Description: "按 job_id 列出最近的运行记录，用于排查失败或验证执行。",
		InputSchema: jobIDSchema(),
		Annotations: &agentclient.MCPToolAnnotations{ReadOnly: true},
		Handler: func(ctx context.Context, args map[string]any) (agentclient.MCPToolResult, error) {
			jobID := argx.String(args, "job_id")
			if jobID == "" {
				return render.Error(errors.New("job_id is required")), nil
			}
			runs, err := svc.ListTaskRuns(ctx, jobID)
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(render.DecorateTimes(runs, "")), nil
		},
	}
}
