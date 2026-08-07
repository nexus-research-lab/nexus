// INPUT: scheduled task 写入返回值与读取阶段的 configuration_version。
// OUTPUT: 从持久化服务重读后的配置，或明确的写后核验错误。
// POS: 对话 task create/update/delete 的 write-after-read 稳定边界。
package tool

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
)

func verifyScheduledTaskPresent(
	ctx context.Context,
	svc contract.Service,
	sctx contract.ServerContext,
	expected automationdomain.ScheduledTask,
) (*automationdomain.ScheduledTask, error) {
	if expected.ConfigurationVersion < 1 {
		return nil, errors.New("scheduled task write returned an invalid configuration_version")
	}
	persisted, err := svc.GetTask(scopedToolContext(ctx, sctx), strings.TrimSpace(expected.JobID))
	if err != nil {
		return nil, err
	}
	if persisted == nil {
		return nil, errors.New("scheduled task write verification failed: task is missing")
	}
	if persisted.ConfigurationVersion != expected.ConfigurationVersion ||
		!sameScheduledTaskConfiguration(*persisted, expected) {
		return nil, errors.New("scheduled task write verification failed: persisted configuration differs")
	}
	return persisted, nil
}

func verifyScheduledTaskVersionedWrite(
	ctx context.Context,
	svc contract.Service,
	expected automationdomain.ScheduledTask,
	previousVersion int64,
) (*automationdomain.ScheduledTask, error) {
	if previousVersion < 1 || expected.ConfigurationVersion != previousVersion+1 {
		return nil, fmt.Errorf(
			"scheduled task version did not advance exactly once: previous=%d current=%d",
			previousVersion,
			expected.ConfigurationVersion,
		)
	}
	persisted, err := svc.GetTask(ctx, strings.TrimSpace(expected.JobID))
	if err != nil {
		return nil, err
	}
	if persisted == nil {
		return nil, errors.New("scheduled task update verification failed: task is missing")
	}
	if persisted.ConfigurationVersion != expected.ConfigurationVersion ||
		!sameScheduledTaskConfiguration(*persisted, expected) {
		return nil, errors.New("scheduled task update verification failed: persisted configuration differs")
	}
	return persisted, nil
}

func verifyScheduledTaskDeleted(ctx context.Context, svc contract.Service, jobID string) error {
	persisted, err := svc.GetTask(ctx, strings.TrimSpace(jobID))
	if err != nil {
		return err
	}
	if persisted != nil {
		return errors.New("scheduled task delete verification failed: task still exists")
	}
	return nil
}

func sameScheduledTaskConfiguration(left automationdomain.ScheduledTask, right automationdomain.ScheduledTask) bool {
	clearScheduledTaskRuntime := func(task *automationdomain.ScheduledTask) {
		task.ConfigurationVersion = 0
		task.NextRunAt = nil
		task.Running = false
		task.RunningRunID = ""
		task.RunningStartedAt = nil
		task.LastRunAt = nil
		task.LastRunStatus = ""
		task.FailureStreak = 0
		task.LastError = nil
		task.LastDeliveryStatus = ""
	}
	clearScheduledTaskRuntime(&left)
	clearScheduledTaskRuntime(&right)
	return reflect.DeepEqual(left, right)
}
