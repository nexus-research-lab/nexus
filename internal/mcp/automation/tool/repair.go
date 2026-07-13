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

func repair(svc contract.Service, sctx contract.ServerContext) sdktool.Tool {
	return sdktool.Tool{
		Name:        "repair_scheduled_task",
		Description: "修复定时任务运行或投递。action=recover 会中断并释放卡住的 active run；action=retry_delivery 只补发已完成 run 的失败投递，不重新执行任务。",
		SearchHint:  searchHintRepairTask,
		InputSchema: repairSchema(),
		Handler: func(ctx context.Context, args map[string]any) (sdktool.ToolResult, error) {
			scope, err := requireOwnedTaskScope(ctx, svc, sctx, args)
			if err != nil {
				return render.Error(err), nil
			}
			switch strings.ToLower(strings.TrimSpace(argx.String(args, "action"))) {
			case "recover":
				job, recoverErr := svc.RecoverTaskRunningRun(scope.Context, scope.JobID, argx.String(args, "run_id"))
				if recoverErr != nil {
					return render.Error(recoverErr), nil
				}
				return render.JSON(render.DecorateTimes(job, job.Schedule.Timezone)), nil
			case "retry_delivery":
				runID, resolveErr := resolveRetryDeliveryRunID(scope.Context, svc, scope, argx.String(args, "run_id"))
				if resolveErr != nil {
					return render.Error(resolveErr), nil
				}
				run, retryErr := svc.RetryRunDelivery(scope.Context, scope.JobID, runID)
				if retryErr != nil {
					return render.Error(retryErr), nil
				}
				return render.JSON(render.DecorateTimes(run, "")), nil
			default:
				return render.Error(errors.New("repair_scheduled_task action must be one of recover, retry_delivery")), nil
			}
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

func retryableDeliveryRunIDs(status *automationdomain.ScheduledTaskStatus) []string {
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
