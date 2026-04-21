package tool

import (
	"context"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/automation/mcp/contract"
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/automation/mcp/internal/render"
	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
)

// status 同时生成 enable / disable 两个工具，仅 enabled 取值不同。
func status(svc contract.Service, name string, enabled bool) agentclient.MCPTool {
	description := "启用定时任务。"
	if !enabled {
		description = "停用定时任务。停用后不会触发，但保留配置。"
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
			job, err := svc.UpdateTaskStatus(ctx, jobID, enabled)
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(render.DecorateTimes(job, job.Schedule.Timezone)), nil
		},
	}
}
