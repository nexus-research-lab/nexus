package tool

import (
	"context"
	"errors"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/internal/render"
)

// status 同时生成 enable / disable 两个工具，仅 enabled 取值不同。
func status(svc contract.Service, sctx contract.ServerContext, name string, enabled bool) agentclient.MCPTool {
	description := "启用定时任务。普通 agent 只能操作自己名下的任务。"
	if !enabled {
		description = "停用定时任务。停用后不会触发，但保留配置。普通 agent 只能操作自己名下的任务。"
	}
	return agentclient.MCPTool{
		Name:        name,
		Description: description,
		InputSchema: jobIDSchema(),
		Handler: func(ctx context.Context, args map[string]any) (agentclient.MCPToolResult, error) {
			jobID := argx.String(args, "job_id")
			if jobID == "" {
				return render.Error(errors.New("job_id is required")), nil
			}
			if err := ensureJobOwnedByCaller(ctx, svc, sctx, jobID); err != nil {
				return render.Error(err), nil
			}
			job, err := svc.UpdateTaskStatus(ctx, jobID, enabled)
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(render.DecorateTimes(job, job.Schedule.Timezone)), nil
		},
	}
}
