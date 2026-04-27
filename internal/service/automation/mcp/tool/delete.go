package tool

import (
	"context"
	"errors"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"

	"github.com/nexus-research-lab/nexus/internal/service/automation/mcp/contract"
	"github.com/nexus-research-lab/nexus/internal/service/automation/mcp/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/service/automation/mcp/internal/render"
)

func del(svc contract.Service, sctx contract.ServerContext) agentclient.MCPTool {
	return agentclient.MCPTool{
		Name:        "delete_scheduled_task",
		Description: "按 job_id 删除定时任务。普通 agent 只能删除自己名下的任务。",
		InputSchema: jobIDSchema(),
		Annotations: &agentclient.MCPToolAnnotations{Destructive: true},
		Handler: func(ctx context.Context, args map[string]any) (agentclient.MCPToolResult, error) {
			jobID := argx.String(args, "job_id")
			if jobID == "" {
				return render.Error(errors.New("job_id is required")), nil
			}
			if err := ensureJobOwnedByCaller(ctx, svc, sctx, jobID); err != nil {
				return render.Error(err), nil
			}
			if err := svc.DeleteTask(ctx, jobID); err != nil {
				return render.Error(err), nil
			}
			return render.JSON(map[string]any{"job_id": jobID, "deleted": true}), nil
		},
	}
}
