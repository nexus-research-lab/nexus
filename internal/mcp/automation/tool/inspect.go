package tool

import (
	"context"
	"errors"
	"strings"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/render"
)

func inspectTask(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "inspect_scheduled_task",
		Description: "检查单个定时任务。view=status 返回当前配置、健康摘要和最近观测；view=runs 返回运行历史；view=events 返回管理审计。runs/events 支持已删除任务。",
		SearchHint:  searchHintInspectTask,
		InputSchema: inspectSchema(),
		Annotations: &sdktool.ToolAnnotations{
			ReadOnly: true,
		},
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			switch strings.ToLower(strings.TrimSpace(argx.String(args, "view"))) {
			case "", "status":
				return inspectTaskStatus(ctx, svc, sctx, args)
			case "runs":
				return inspectTaskRuns(ctx, svc, sctx, args)
			case "events":
				return inspectTaskEvents(ctx, svc, sctx, args)
			default:
				return render.Error(errors.New("inspect_scheduled_task view must be one of status, runs, events")), nil
			}
		},
	}
}

func inspectTaskStatus(ctx context.Context, svc contract.Service, sctx contract.ServerContext, args map[string]any) (sdktool.ToolResult, error) {
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
}

func inspectTaskRuns(ctx context.Context, svc contract.Service, sctx contract.ServerContext, args map[string]any) (sdktool.ToolResult, error) {
	scope, err := requireOwnedTaskHistoryScope(ctx, svc, sctx, args)
	if err != nil {
		return render.Error(err), nil
	}
	runs, err := svc.ListTaskRuns(scope.Context, scope.JobID)
	if err != nil {
		return render.Error(err), nil
	}
	runs = limitSlice(runs, boundedInspectLimit(argx.Int(args["run_limit"])))
	return render.JSON(render.DecorateTimes(runs, "")), nil
}

func inspectTaskEvents(ctx context.Context, svc contract.Service, sctx contract.ServerContext, args map[string]any) (sdktool.ToolResult, error) {
	scope, err := requireOwnedTaskHistoryScope(ctx, svc, sctx, args)
	if err != nil {
		return render.Error(err), nil
	}
	events, err := svc.ListTaskEvents(scope.Context, scope.JobID, boundedInspectLimit(argx.Int(args["event_limit"])))
	if err != nil {
		return render.Error(err), nil
	}
	return render.JSON(render.DecorateTimes(events, "")), nil
}

func boundedInspectLimit(limit int) int {
	if limit <= 0 {
		return 10
	}
	return min(limit, 50)
}

func limitSlice[T any](items []T, limit int) []T {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}
