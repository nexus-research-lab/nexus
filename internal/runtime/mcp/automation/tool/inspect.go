package tool

import (
	"context"
	"errors"

	sdktool "github.com/nexus-research-lab/nexus/internal/runtime/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/internal/render"
)

func inspectTask(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "get_scheduled_task_status",
		Description: "按 job_id 或 query 查看单个定时任务的当前配置、健康状态、最近运行记录和最近管理事件。适合回答“这个任务现在怎么样/今天有没有发送失败/要不要修正执行失败、恢复或重投递”。query 只在当前权限范围内唯一命中当前未删除任务时才会查询。普通 agent 只能查看自己名下任务。",
		SearchHint:  searchHintGetTaskStatus,
		InputSchema: taskStatusSchema(),
		Annotations: &sdktool.ToolAnnotations{
			ReadOnly: true,
		},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			scope, err := requireOwnedTaskScope(ctx, svc, sctx, args)
			if err != nil {
				return render.Error(err), nil
			}
			payload, err := svc.GetTaskStatus(
				scope.Context,
				scope.JobID,
				argx.Int(args["run_limit"]),
				argx.Int(args["event_limit"]),
			)
			if err != nil {
				return render.Error(err), nil
			}
			if payload == nil {
				return render.Error(errors.New("scheduled task not found")), nil
			}
			return render.JSON(render.DecorateTimes(payload, payload.Job.Schedule.Timezone)), nil
		},
	}
}
