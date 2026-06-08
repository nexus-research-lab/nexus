package tool

import (
	"context"

	sdktool "github.com/nexus-research-lab/nexus/internal/runtime/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/internal/render"
)

func taskEvents(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "get_scheduled_task_events",
		Description: "按 job_id 或 query 查看定时任务管理动作审计记录，包括创建、修改、启停、删除、立即运行、恢复、手动重投递和自动投递重试。query 可定位当前或已删除任务，唯一命中后再读取审计；当前会话是 DM/Room/IM 群时优先当前会话匹配，写“这里/当前会话/这个群/当前频道”会强制限定到当前会话。普通 agent 只能查看自己名下任务。",
		SearchHint:  searchHintGetTaskEvents,
		InputSchema: taskEventsSchema(),
		Annotations: &sdktool.ToolAnnotations{
			ReadOnly: true,
		},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			scope, err := requireOwnedTaskHistoryScope(ctx, svc, sctx, args)
			if err != nil {
				return render.Error(err), nil
			}
			items, err := svc.ListTaskEvents(scope.Context, scope.JobID, argx.Int(args["limit"]))
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(render.DecorateTimes(items, "")), nil
		},
	}
}
