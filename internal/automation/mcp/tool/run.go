package tool

import (
	"context"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/automation/mcp/contract"
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/internal/render"
	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
)

func runNow(svc contract.Service) agentclient.MCPTool {
	return agentclient.MCPTool{
		Name:        "run_scheduled_task",
		Description: "立即触发一次执行（不影响后续排程），用于验证或紧急补跑。",
		InputSchema: jobIDSchema(),
		Handler: func(ctx context.Context, args map[string]any) (agentclient.MCPToolResult, error) {
			jobID := argx.String(args, "job_id")
			if jobID == "" {
				return render.Error(errors.New("job_id is required")), nil
			}
			result, err := svc.RunTaskNow(ctx, jobID)
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(render.DecorateTimes(result, "")), nil
		},
	}
}
