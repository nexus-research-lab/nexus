package automation

import (
	"database/sql"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation"
)

func scanCronJob(scanner interface {
	Scan(dest ...any) error
}) (automationdomain.CronJob, error) {
	var (
		item               automationdomain.CronJob
		runAt              sql.NullString
		intervalSeconds    sql.NullInt64
		cronExpression     sql.NullString
		boundSessionKey    sql.NullString
		namedSessionKey    sql.NullString
		deliveryChannel    sql.NullString
		deliveryTo         sql.NullString
		deliveryAccountID  sql.NullString
		deliveryThreadID   sql.NullString
		sourceKind         sql.NullString
		sourceCreatorID    sql.NullString
		sourceContextType  sql.NullString
		sourceContextID    sql.NullString
		sourceContextLabel sql.NullString
		sourceSessionKey   sql.NullString
		sourceSessionLabel sql.NullString
		nextRunAt          sql.NullTime
		runningRunID       sql.NullString
		runningStartedAt   sql.NullTime
		lastRunAt          sql.NullTime
		lastRunStatus      sql.NullString
		failureStreak      sql.NullInt64
		lastError          sql.NullString
		lastDeliveryStatus sql.NullString
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
		&item.SessionTarget.Kind,
		&boundSessionKey,
		&namedSessionKey,
		&item.SessionTarget.WakeMode,
		&item.Delivery.Mode,
		&deliveryChannel,
		&deliveryTo,
		&deliveryAccountID,
		&deliveryThreadID,
		&sourceKind,
		&sourceCreatorID,
		&sourceContextType,
		&sourceContextID,
		&sourceContextLabel,
		&sourceSessionKey,
		&sourceSessionLabel,
		&item.OverlapPolicy,
		&item.Enabled,
		&nextRunAt,
		&runningRunID,
		&runningStartedAt,
		&lastRunAt,
		&lastRunStatus,
		&failureStreak,
		&lastError,
		&lastDeliveryStatus,
	)
	if err != nil {
		return automationdomain.CronJob{}, err
	}
	item.Schedule.RunAt = nullStringToPointer(runAt)
	item.Schedule.IntervalSeconds = nullIntToPointer(intervalSeconds)
	item.Schedule.CronExpression = nullStringToPointer(cronExpression)
	item.ExecutionKind = automationdomain.NormalizeExecutionKind(item.ExecutionKind)
	item.SessionTarget.BoundSessionKey = nullStringValue(boundSessionKey)
	item.SessionTarget.NamedSessionKey = nullStringValue(namedSessionKey)
	item.Delivery.Channel = nullStringValue(deliveryChannel)
	item.Delivery.To = nullStringValue(deliveryTo)
	item.Delivery.AccountID = nullStringValue(deliveryAccountID)
	item.Delivery.ThreadID = nullStringValue(deliveryThreadID)
	item.Source.Kind = nullStringValue(sourceKind)
	item.Source.CreatorAgentID = nullStringValue(sourceCreatorID)
	item.Source.ContextType = nullStringValue(sourceContextType)
	item.Source.ContextID = nullStringValue(sourceContextID)
	item.Source.ContextLabel = nullStringValue(sourceContextLabel)
	item.Source.SessionKey = nullStringValue(sourceSessionKey)
	item.Source.SessionLabel = nullStringValue(sourceSessionLabel)
	item.Source = item.Source.Normalized()
	item.OverlapPolicy = automationdomain.NormalizeOverlapPolicy(item.OverlapPolicy)
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
	return item, nil
}

func scanCronJobRow(row *sql.Row) (*automationdomain.CronJob, error) {
	item, err := scanCronJob(row)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func scanCronRun(scanner interface {
	Scan(dest ...any) error
}) (automationdomain.CronRun, error) {
	var (
		item                  automationdomain.CronRun
		sessionKey            sql.NullString
		roundID               sql.NullString
		sessionID             sql.NullString
		deliveryMode          sql.NullString
		deliveryTo            sql.NullString
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
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return automationdomain.CronRun{}, err
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
	item.DeliveryStatus = nullStringValue(deliveryStatus)
	item.DeliveryError = nullStringToPointer(deliveryError)
	item.DeliveredAt = nullTimePointer(deliveredAt)
	item.DeliveryNextAttemptAt = nullTimePointer(deliveryNextAttemptAt)
	item.DeliveryDeadLetterAt = nullTimePointer(deliveryDeadLetterAt)
	item.ResultSummary = nullStringToPointer(resultSummary)
	item.AssistantText = nullStringToPointer(assistantText)
	item.ResultText = nullStringToPointer(resultText)
	item.ArtifactPath = nullStringToPointer(artifactPath)
	return item, nil
}
