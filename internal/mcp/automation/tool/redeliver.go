package tool

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/argx"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/render"
)

func redeliver(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "retry_scheduled_task_delivery",
		Description: "按 job_id 或 query 只重试某次 run 的结果投递，不重新执行任务本身。适合处理“任务已跑完但飞书/IM 发送失败”。run_id 可显式传入；没传时会从任务健康摘要里自动选择唯一可手动补投递的失败 run，多候选会要求用户确认。query 只在当前权限范围内唯一命中当前未删除任务时才会执行。普通 agent 只能操作自己名下任务。",
		SearchHint:  searchHintRetryDelivery,
		InputSchema: runIDSchema(),
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			scope, err := requireOwnedTaskScope(ctx, svc, sctx, args)
			if err != nil {
				return render.Error(err), nil
			}
			runID, err := resolveRetryDeliveryRunID(scope.Context, svc, scope, argx.String(args, "run_id"))
			if err != nil {
				return render.Error(err), nil
			}
			run, err := svc.RetryRunDelivery(scope.Context, scope.JobID, runID)
			if err != nil {
				return render.Error(err), nil
			}
			return render.JSON(render.DecorateTimes(run, "")), nil
		},
	}
}

func resolveRetryDeliveryRunID(
	ctx context.Context,
	svc contract.Service,
	scope ownedTaskScope,
	requested string,
) (string, error) {
	runID := strings.TrimSpace(requested)
	if runID != "" {
		return runID, nil
	}
	status, err := svc.GetTaskStatus(ctx, scope.JobID, 20, 0)
	if err != nil {
		return "", err
	}
	candidates := retryableDeliveryRunIDs(status)
	switch len(candidates) {
	case 0:
		return "", errors.New("run_id is required because no failed delivery run is currently retryable for this scheduled task")
	case 1:
		return candidates[0], nil
	default:
		return "", fmt.Errorf("multiple failed delivery runs are retryable; ask the user to choose one run_id: %s", strings.Join(candidates, ", "))
	}
}

func retryableDeliveryRunIDs(status *automationdomain.CronTaskStatus) []string {
	if status == nil {
		return nil
	}
	runIDs := make([]string, 0, len(status.Health.ManualRedeliveryRunIDs)+len(status.Health.DeliveryDeadLetterRunIDs))
	seen := map[string]bool{}
	appendUniqueRunIDs(&runIDs, seen, status.Health.ManualRedeliveryRunIDs)
	appendUniqueRunIDs(&runIDs, seen, status.Health.DeliveryDeadLetterRunIDs)
	for _, run := range status.RecentRuns {
		if strings.TrimSpace(run.DeliveryStatus) != automationdomain.DeliveryStatusFailed {
			continue
		}
		appendUniqueRunIDs(&runIDs, seen, []string{run.RunID})
	}
	return runIDs
}

func appendUniqueRunIDs(target *[]string, seen map[string]bool, values []string) {
	for _, value := range values {
		runID := strings.TrimSpace(value)
		if runID == "" || seen[runID] {
			continue
		}
		seen[runID] = true
		*target = append(*target, runID)
	}
}
