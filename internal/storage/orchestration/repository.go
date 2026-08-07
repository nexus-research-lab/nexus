// INPUT: SQL driver、database connection、command identity 与 expected Execution version。
// OUTPUT: Repository、事务性幂等/CAS 与 append-only event sequence。
// POS: 所有 Orchestration mutation 共享的根事务边界。
package orchestration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

type sqlQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type sqlExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// Repository 封装 Execution Orchestration 的跨方言 SQL 事务。
type Repository struct {
	db      *sql.DB
	dialect storage.SQLDialect
	now     func() time.Time
}

// NewRepository 使用应用配置创建 Repository。
func NewRepository(cfg config.Config, db *sql.DB) *Repository {
	return NewSQLRepository(cfg.DatabaseDriver, db)
}

// NewSQLRepository 使用显式 database/sql driver 创建 Repository。
func NewSQLRepository(driver string, db *sql.DB) *Repository {
	return &Repository{
		db:      db,
		dialect: storage.NewSQLDialect(driver),
		now:     time.Now,
	}
}

func (r *Repository) bind(index int) string {
	return r.dialect.Bind(index)
}

func (r *Repository) jsonBind(index int) string {
	return r.dialect.JSONValue(index)
}

func (r *Repository) timestamp(value time.Time) any {
	return r.dialect.TimestampValue(value)
}

func (r *Repository) currentTime() time.Time {
	return r.now().UTC()
}

func validateMeta(meta CommandMeta) error {
	if strings.TrimSpace(meta.CommandID) == "" || strings.TrimSpace(meta.EventID) == "" {
		return fmt.Errorf("%w: command id and event id are required", ErrInvariant)
	}
	switch meta.ActorKind {
	case protocol.ExecutionActorUser,
		protocol.ExecutionActorAgent,
		protocol.ExecutionActorRuntime,
		protocol.ExecutionActorSystem:
	default:
		return fmt.Errorf("%w: actor kind %q is invalid", ErrInvariant, meta.ActorKind)
	}
	return nil
}

func validateExpectedVersion(value int64, name string) error {
	if value <= 0 {
		return fmt.Errorf("%w: %s must be positive", ErrVersionConflict, name)
	}
	return nil
}

type mutation struct {
	tx              *sql.Tx
	executionID     string
	commandID       string
	expectedVersion int64
	eventType       protocol.ExecutionEventType
	replayed        bool
}

func (r *Repository) beginMutation(
	ctx context.Context,
	executionID string,
	expectedVersion int64,
	meta CommandMeta,
	eventType protocol.ExecutionEventType,
) (*mutation, error) {
	executionID = strings.TrimSpace(executionID)
	if executionID == "" {
		return nil, fmt.Errorf("%w: execution id is required", ErrInvariant)
	}
	if err := validateExpectedVersion(expectedVersion, "expected execution version"); err != nil {
		return nil, err
	}
	if err := validateMeta(meta); err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	value := &mutation{
		tx:              tx,
		executionID:     executionID,
		commandID:       strings.TrimSpace(meta.CommandID),
		expectedVersion: expectedVersion,
		eventType:       eventType,
	}
	existing, err := r.findEventByCommand(ctx, tx, executionID, value.commandID)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if existing != nil {
		if existing.Type != eventType {
			_ = tx.Rollback()
			return nil, fmt.Errorf("%w: command %q was recorded as %q", ErrCommandConflict, value.commandID, existing.Type)
		}
		value.replayed = true
		return value, nil
	}
	result, err := tx.ExecContext(ctx, `
UPDATE executions
SET version = version + 1,
    updated_at = `+r.bind(1)+`
WHERE execution_id = `+r.bind(2)+`
  AND version = `+r.bind(3)+`
  AND status IN ('active', 'waiting', 'paused')`,
		r.timestamp(r.currentTime()),
		executionID,
		expectedVersion,
	)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if affected != 1 {
		_ = tx.Rollback()
		existing, findErr := r.findEventByCommand(ctx, r.db, executionID, value.commandID)
		if findErr != nil {
			return nil, findErr
		}
		if existing != nil {
			if existing.Type != eventType {
				return nil, fmt.Errorf("%w: command %q was recorded as %q", ErrCommandConflict, value.commandID, existing.Type)
			}
			value.replayed = true
			value.tx = nil
			return value, nil
		}
		return nil, ErrVersionConflict
	}
	return value, nil
}

func (r *Repository) finishMutation(
	ctx context.Context,
	value *mutation,
	meta CommandMeta,
	event protocol.ExecutionEvent,
) (*protocol.ExecutionSnapshot, error) {
	if value.replayed {
		if value.tx != nil {
			_ = value.tx.Rollback()
		}
		return r.GetSnapshot(ctx, value.executionID)
	}
	event.ID = strings.TrimSpace(meta.EventID)
	event.ExecutionID = value.executionID
	event.Type = value.eventType
	event.CommandID = value.commandID
	event.ActorKind = meta.ActorKind
	event.ActorID = strings.TrimSpace(meta.ActorID)
	event.RootRoundID = strings.TrimSpace(meta.RootRoundID)
	event.RuntimeRoundID = strings.TrimSpace(meta.RuntimeRoundID)
	event.AgentRoundID = strings.TrimSpace(meta.AgentRoundID)
	event.Payload = meta.Payload
	event.CreatedAt = timeOr(meta.CreatedAt, r.currentTime())
	if err := r.insertEvent(ctx, value.tx, event); err != nil {
		_ = value.tx.Rollback()
		return nil, err
	}
	if err := value.tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetSnapshot(ctx, value.executionID)
}

func (r *Repository) abortMutation(value *mutation) {
	if value != nil && value.tx != nil {
		_ = value.tx.Rollback()
	}
}

func (r *Repository) insertEvent(
	ctx context.Context,
	tx *sql.Tx,
	event protocol.ExecutionEvent,
) error {
	if event.ID == "" || event.ExecutionID == "" || event.CommandID == "" ||
		event.EntityID == "" || event.EntityVersion <= 0 {
		return fmt.Errorf("%w: execution event identity is incomplete", ErrInvariant)
	}
	payloadJSON, err := marshalMap(event.Payload)
	if err != nil {
		return fmt.Errorf("marshal execution event payload: %w", err)
	}
	if err = tx.QueryRowContext(ctx, `
SELECT COALESCE(MAX(sequence), 0) + 1
FROM execution_events
WHERE execution_id = `+r.bind(1),
		event.ExecutionID,
	).Scan(&event.Sequence); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO execution_events (
    event_id, execution_id, sequence, command_id, event_type,
    entity_type, entity_id, entity_version, actor_kind, actor_id,
    goal_id, plan_id, work_item_id, spec_id, assignment_id,
    dispatch_id, attempt_id, submission_id, review_dispatch_id, acceptance_id,
    root_round_id, runtime_round_id, agent_round_id, payload_json, created_at
) VALUES (`+
		r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+
		r.bind(6)+`,`+r.bind(7)+`,`+r.bind(8)+`,`+r.bind(9)+`,`+r.bind(10)+`,`+
		r.bind(11)+`,`+r.bind(12)+`,`+r.bind(13)+`,`+r.bind(14)+`,`+r.bind(15)+`,`+
		r.bind(16)+`,`+r.bind(17)+`,`+r.bind(18)+`,`+r.bind(19)+`,`+r.bind(20)+`,`+
		r.bind(21)+`,`+r.bind(22)+`,`+r.bind(23)+`,`+r.jsonBind(24)+`,`+r.bind(25)+`)`,
		event.ID, event.ExecutionID, event.Sequence, event.CommandID, event.Type,
		event.EntityType, event.EntityID, event.EntityVersion, event.ActorKind, nullString(event.ActorID),
		nullString(event.GoalID), nullString(event.PlanID), nullString(event.WorkItemID), nullString(event.SpecID),
		nullString(event.AssignmentID), nullString(event.DispatchID), nullString(event.AttemptID),
		nullString(event.SubmissionID), nullString(event.ReviewDispatchID), nullString(event.AcceptanceID), nullString(event.RootRoundID),
		nullString(event.RuntimeRoundID), nullString(event.AgentRoundID), payloadJSON, r.timestamp(event.CreatedAt),
	)
	return err
}

func (r *Repository) findEventByCommand(
	ctx context.Context,
	queryer sqlQueryer,
	executionID string,
	commandID string,
) (*protocol.ExecutionEvent, error) {
	row := queryer.QueryRowContext(ctx, eventSelect(r.dialect.JSONText("payload_json"))+`
WHERE execution_id = `+r.bind(1)+`
  AND command_id = `+r.bind(2),
		executionID,
		commandID,
	)
	event, err := scanEvent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &event, nil
}
