// INPUT: 与 automation task/run 查询字段顺序一致的 SQL scanner。
// OUTPUT: 归一化的 ScheduledTask、ScheduledTaskRun 与 Heartbeat 领域快照。
// POS: Automation storage 的单一扫描投影；新增持久字段必须在此同步接入。
package automation

import (
	"database/sql"
	"encoding/json"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

func scanScheduledTask(scanner interface {
	Scan(dest ...any) error
}) (automationdomain.ScheduledTask, error) {
	var (
		item                       automationdomain.ScheduledTask
		runAt                      sql.NullString
		intervalSeconds            sql.NullInt64
		cronExpression             sql.NullString
		boundSessionKey            sql.NullString
		namedSessionKey            sql.NullString
		deliveryChannel            sql.NullString
		deliveryTo                 sql.NullString
		deliveryAccountID          sql.NullString
		deliveryThreadID           sql.NullString
		deliverySessionKey         sql.NullString
		deliveryAgentID            sql.NullString
		invalidatedSessionKeysJSON sql.NullString
		sourceKind                 sql.NullString
		sourceCreatorID            sql.NullString
		sourceContextType          sql.NullString
		sourceContextID            sql.NullString
		sourceContextLabel         sql.NullString
		sourceSessionKey           sql.NullString
		sourceSessionLabel         sql.NullString
		deliveryGrantJSON          sql.NullString
		expiresAt                  sql.NullTime
		nextRunAt                  sql.NullTime
		runningRunID               sql.NullString
		runningStartedAt           sql.NullTime
		lastRunAt                  sql.NullTime
		lastRunStatus              sql.NullString
		failureStreak              sql.NullInt64
		lastError                  sql.NullString
		lastDeliveryStatus         sql.NullString
		permissionPolicyJSON       sql.NullString
		pendingPermissionRequestID sql.NullString
	)
	err := scanner.Scan(
		&item.JobID,
		&item.OwnerUserID,
		&item.Name,
		&item.AgentID,
		&item.Schedule.Kind,
		&runAt,
		&intervalSeconds,
		&cronExpression,
		&item.Schedule.Timezone,
		&item.Instruction,
		&item.ExecutionKind,
		&item.PermissionMode,
		&item.SessionTarget.Kind,
		&boundSessionKey,
		&namedSessionKey,
		&item.SessionTarget.WakeMode,
		&item.Delivery.Mode,
		&deliveryChannel,
		&deliveryTo,
		&deliveryAccountID,
		&deliveryThreadID,
		&deliverySessionKey,
		&deliveryAgentID,
		&item.SessionBindingState,
		&invalidatedSessionKeysJSON,
		&sourceKind,
		&sourceCreatorID,
		&sourceContextType,
		&sourceContextID,
		&sourceContextLabel,
		&sourceSessionKey,
		&sourceSessionLabel,
		&deliveryGrantJSON,
		&item.OverlapPolicy,
		&expiresAt,
		&item.Enabled,
		&nextRunAt,
		&runningRunID,
		&runningStartedAt,
		&lastRunAt,
		&lastRunStatus,
		&failureStreak,
		&lastError,
		&lastDeliveryStatus,
		&item.ConfigurationVersion,
		&permissionPolicyJSON,
		&item.PermissionPolicy.Revision,
		&item.PermissionState,
		&pendingPermissionRequestID,
	)
	if err != nil {
		return automationdomain.ScheduledTask{}, err
	}
	item.Schedule.RunAt = nullStringToPointer(runAt)
	item.Schedule.IntervalSeconds = nullIntToPointer(intervalSeconds)
	item.Schedule.CronExpression = nullStringToPointer(cronExpression)
	item.ExecutionKind = automationdomain.NormalizeExecutionKind(item.ExecutionKind)
	item.PermissionMode = automationdomain.NormalizePermissionMode(item.PermissionMode)
	item.SessionTarget.BoundSessionKey = nullStringValue(boundSessionKey)
	item.SessionTarget.NamedSessionKey = nullStringValue(namedSessionKey)
	item.Delivery.Channel = nullStringValue(deliveryChannel)
	item.Delivery.To = nullStringValue(deliveryTo)
	item.Delivery.AccountID = nullStringValue(deliveryAccountID)
	item.Delivery.ThreadID = nullStringValue(deliveryThreadID)
	item.Delivery.SessionKey = nullStringValue(deliverySessionKey)
	item.Delivery.AgentID = nullStringValue(deliveryAgentID)
	if raw := strings.TrimSpace(nullStringValue(invalidatedSessionKeysJSON)); raw != "" && raw != "[]" {
		if decodeErr := json.Unmarshal([]byte(raw), &item.InvalidatedSessionKeys); decodeErr != nil {
			return automationdomain.ScheduledTask{}, decodeErr
		}
	}
	item.Source.Kind = nullStringValue(sourceKind)
	item.Source.CreatorAgentID = nullStringValue(sourceCreatorID)
	item.Source.ContextType = nullStringValue(sourceContextType)
	item.Source.ContextID = nullStringValue(sourceContextID)
	item.Source.ContextLabel = nullStringValue(sourceContextLabel)
	item.Source.SessionKey = nullStringValue(sourceSessionKey)
	item.Source.SessionLabel = nullStringValue(sourceSessionLabel)
	item.Source = item.Source.Normalized()
	if raw := strings.TrimSpace(nullStringValue(deliveryGrantJSON)); raw != "" && raw != "{}" {
		if decodeErr := json.Unmarshal([]byte(raw), &item.DeliveryGrant); decodeErr != nil {
			return automationdomain.ScheduledTask{}, decodeErr
		}
	}
	item.OverlapPolicy = automationdomain.NormalizeOverlapPolicy(item.OverlapPolicy)
	item.ExpiresAt = nullTimePointer(expiresAt)
	item.NextRunAt = nullTimePointer(nextRunAt)
	item.RunningRunID = nullStringValue(runningRunID)
	item.RunningStartedAt = nullTimePointer(runningStartedAt)
	item.Running = item.RunningRunID != ""
	item.LastRunAt = nullTimePointer(lastRunAt)
	item.LastRunStatus = nullStringValue(lastRunStatus)
	if failureStreak.Valid {
		item.FailureStreak = int(failureStreak.Int64)
	}
	item.LastError = nullStringToPointer(lastError)
	item.LastDeliveryStatus = nullStringValue(lastDeliveryStatus)
	if raw := strings.TrimSpace(nullStringValue(permissionPolicyJSON)); raw != "" && raw != "{}" {
		storedRevision := item.PermissionPolicy.Revision
		if decodeErr := json.Unmarshal([]byte(raw), &item.PermissionPolicy); decodeErr != nil {
			return automationdomain.ScheduledTask{}, decodeErr
		}
		item.PermissionPolicy.Revision = storedRevision
	}
	item.PendingPermissionRequestID = nullStringValue(pendingPermissionRequestID)
	return automationdomain.NormalizeScheduledTaskCompatibility(item), nil
}

func scanScheduledTaskRow(row *sql.Row) (*automationdomain.ScheduledTask, error) {
	item, err := scanScheduledTask(row)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func scanScheduledTaskRun(scanner interface {
	Scan(dest ...any) error
}) (automationdomain.ScheduledTaskRun, error) {
	var (
		item                  automationdomain.ScheduledTaskRun
		sessionKey            sql.NullString
		roundID               sql.NullString
		sessionID             sql.NullString
		deliveryMode          sql.NullString
		deliveryTo            sql.NullString
		deliveryTargetJSON    sql.NullString
		deliveryStatus        sql.NullString
		deliveryError         sql.NullString
		deliveredAt           sql.NullTime
		deliveryNextAttemptAt sql.NullTime
		deliveryDeadLetterAt  sql.NullTime
		resultSummary         sql.NullString
		scheduledFor          sql.NullTime
		startedAt             sql.NullTime
		finishedAt            sql.NullTime
		errorMessage          sql.NullString
		assistantText         sql.NullString
		resultText            sql.NullString
		artifactPath          sql.NullString
		blockState            sql.NullString
		blockedRequestID      sql.NullString
	)
	err := scanner.Scan(
		&item.RunID,
		&item.JobID,
		&item.OwnerUserID,
		&item.Status,
		&item.TriggerKind,
		&sessionKey,
		&roundID,
		&sessionID,
		&item.MessageCount,
		&deliveryMode,
		&deliveryTo,
		&deliveryTargetJSON,
		&deliveryStatus,
		&deliveryError,
		&deliveredAt,
		&item.DeliveryAttempts,
		&deliveryNextAttemptAt,
		&deliveryDeadLetterAt,
		&scheduledFor,
		&startedAt,
		&finishedAt,
		&item.Attempts,
		&errorMessage,
		&resultSummary,
		&assistantText,
		&resultText,
		&artifactPath,
		&item.PermissionPolicyRevision,
		&blockState,
		&blockedRequestID,
		&item.EffectStarted,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return automationdomain.ScheduledTaskRun{}, err
	}
	item.ScheduledFor = nullTimePointer(scheduledFor)
	item.StartedAt = nullTimePointer(startedAt)
	item.FinishedAt = nullTimePointer(finishedAt)
	item.ErrorMessage = nullStringToPointer(errorMessage)
	item.SessionKey = nullStringValue(sessionKey)
	item.RoundID = nullStringValue(roundID)
	item.SessionID = nullStringToPointer(sessionID)
	item.DeliveryMode = nullStringValue(deliveryMode)
	item.DeliveryTo = nullStringValue(deliveryTo)
	if raw := strings.TrimSpace(nullStringValue(deliveryTargetJSON)); raw != "" && raw != "{}" {
		var target automationdomain.DeliveryTarget
		if decodeErr := json.Unmarshal([]byte(raw), &target); decodeErr != nil {
			return automationdomain.ScheduledTaskRun{}, decodeErr
		}
		target = target.Normalized()
		item.DeliveryTarget = &target
	}
	item.DeliveryStatus = nullStringValue(deliveryStatus)
	item.DeliveryError = nullStringToPointer(deliveryError)
	item.DeliveredAt = nullTimePointer(deliveredAt)
	item.DeliveryNextAttemptAt = nullTimePointer(deliveryNextAttemptAt)
	item.DeliveryDeadLetterAt = nullTimePointer(deliveryDeadLetterAt)
	item.ResultSummary = nullStringToPointer(resultSummary)
	item.AssistantText = nullStringToPointer(assistantText)
	item.ResultText = nullStringToPointer(resultText)
	item.ArtifactPath = nullStringToPointer(artifactPath)
	item.BlockState = nullStringValue(blockState)
	item.BlockedRequestID = nullStringValue(blockedRequestID)
	return item, nil
}
