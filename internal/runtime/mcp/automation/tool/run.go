package tool

import (
	"context"
	"errors"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/internal/render"
)

func runNow(svc contract.Service, sctx contract.ServerContext) agentclient.MCPTool {
	return agentclient.MCPTool{
		Name:        "run_scheduled_task",
		Description: "立即触发一次执行（不影响后续排程），用于验证或紧急补跑。普通 agent 只能触发自己名下的任务。",
		InputSchema: jobIDSchema(),
		Handler: func(ctx context.Context, args map[string]any) (agentclient.MCPToolResult, error) {
			jobID := argx.String(args, "job_id")
			if jobID == "" {
				return render.Error(errors.New("job_id is required")), nil
			}
			if err := ensureJobOwnedByCaller(ctx, svc, sctx, jobID); err != nil {
				return render.Error(err), nil
			}
			result, err := svc.RunTaskNow(ctx, jobID)
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(render.DecorateTimes(result, "")), nil
		},
	}
}
