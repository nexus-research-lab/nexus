package automation

import (
	"database/sql"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func scanCronJob(scanner interface {
	Scan(dest ...any) error
}) (protocol.CronJob, error) {
	var (
		item               protocol.CronJob
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
	)
	err := scanner.Scan(
		&item.JobID,
		&item.Name,
		&item.AgentID,
		&item.Schedule.Kind,
		&runAt,
		&intervalSeconds,
		&cronExpression,
		&item.Schedule.Timezone,
		&item.Instruction,
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
		&item.Enabled,
	)
	if err != nil {
		return protocol.CronJob{}, err
	}
	item.Schedule.RunAt = nullStringToPointer(runAt)
	item.Schedule.IntervalSeconds = nullIntToPointer(intervalSeconds)
	item.Schedule.CronExpression = nullStringToPointer(cronExpression)
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
	return item, nil
}

func scanCronJobRow(row *sql.Row) (*protocol.CronJob, error) {
	item, err := scanCronJob(row)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func scanCronRun(scanner interface {
	Scan(dest ...any) error
}) (protocol.CronRun, error) {
	var (
		item         protocol.CronRun
		scheduledFor sql.NullTime
		startedAt    sql.NullTime
		finishedAt   sql.NullTime
		errorMessage sql.NullString
	)
	err := scanner.Scan(
		&item.RunID,
		&item.JobID,
		&item.Status,
		&scheduledFor,
		&startedAt,
		&finishedAt,
		&item.Attempts,
		&errorMessage,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return protocol.CronRun{}, err
	}
	item.ScheduledFor = nullTimePointer(scheduledFor)
	item.StartedAt = nullTimePointer(startedAt)
	item.FinishedAt = nullTimePointer(finishedAt)
	item.ErrorMessage = nullStringToPointer(errorMessage)
	return item, nil
}
