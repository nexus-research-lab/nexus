// INPUT: exact Agent/owner/request/intent、可选 heartbeat 配置版本与 wake payload。
// OUTPUT: 与配置更新线性化的 durable wake acceptance、租约领取和恢复 deadline。
// POS: Heartbeat wake outbox 的唯一事务边界；Agent 行只作栅栏，不改变身份。
package automation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	storagebase "github.com/nexus-research-lab/nexus/internal/storage"
)

const heartbeatWakeEventType = "heartbeat.wake"

// HeartbeatWakeAcceptanceInput 描述一次 durable wake 受理。
type HeartbeatWakeAcceptanceInput struct {
	EventID                      string
	AgentID                      string
	OwnerUserID                  string
	RequestID                    string
	IntentDigest                 string
	Mode                         string
	Text                         *string
	ExpectedConfigurationVersion *int64
	AcceptedAt                   time.Time
}

// HeartbeatWakeAcceptanceResult 返回首次受理或 exact request 重放事实。
type HeartbeatWakeAcceptanceResult struct {
	Event    automationdomain.SystemEvent
	Replayed bool
}

// AcceptHeartbeatWake 在 Agent 配置栅栏内核对 durable configuration_version 并写入 outbox。
func (r *Repository) AcceptHeartbeatWake(
	ctx context.Context,
	input HeartbeatWakeAcceptanceInput,
) (HeartbeatWakeAcceptanceResult, error) {
	input.EventID = strings.TrimSpace(input.EventID)
	input.AgentID = strings.TrimSpace(input.AgentID)
	input.OwnerUserID = strings.TrimSpace(input.OwnerUserID)
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.IntentDigest = strings.TrimSpace(input.IntentDigest)
	input.Mode = strings.TrimSpace(input.Mode)
	if input.AcceptedAt.IsZero() {
		input.AcceptedAt = time.Now().UTC()
	} else {
		input.AcceptedAt = input.AcceptedAt.UTC()
	}
	if input.EventID == "" || input.AgentID == "" || input.Mode == "" {
		return HeartbeatWakeAcceptanceResult{}, errors.New("heartbeat wake acceptance identity is incomplete")
	}
	if input.RequestID != "" && (input.OwnerUserID == "" || input.IntentDigest == "") {
		return HeartbeatWakeAcceptanceResult{}, errors.New("durable heartbeat wake request requires owner and intent digest")
	}
	if input.ExpectedConfigurationVersion != nil && *input.ExpectedConfigurationVersion < 0 {
		return HeartbeatWakeAcceptanceResult{}, automationdomain.ErrConfigurationVersionConflict
	}

	attempts := automationWriteRetryAttempts
	if r.isPostgres {
		attempts = 1
	}
	for attempt := 0; attempt < attempts; attempt++ {
		result, err := r.acceptHeartbeatWakeOnce(ctx, input)
		if err == nil || !isSQLiteLockedError(err) || attempt == attempts-1 {
			return result, err
		}
		timer := time.NewTimer(automationWriteRetryDelay * time.Duration(attempt+1))
		select {
		case <-ctx.Done():
			timer.Stop()
			return HeartbeatWakeAcceptanceResult{}, ctx.Err()
		case <-timer.C:
		}
	}
	return HeartbeatWakeAcceptanceResult{}, errors.New("heartbeat wake acceptance retries exhausted")
}

func (r *Repository) acceptHeartbeatWakeOnce(
	ctx context.Context,
	input HeartbeatWakeAcceptanceInput,
) (HeartbeatWakeAcceptanceResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return HeartbeatWakeAcceptanceResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = r.lockHeartbeatConfigurationTx(ctx, tx, input.AgentID, input.OwnerUserID); err != nil {
		return HeartbeatWakeAcceptanceResult{}, err
	}
	if input.RequestID != "" {
		existing, found, loadErr := r.getHeartbeatWakeByRequestTx(
			ctx, tx, input.OwnerUserID, input.RequestID,
		)
		if loadErr != nil {
			return HeartbeatWakeAcceptanceResult{}, loadErr
		}
		if found {
			if err = validateHeartbeatWakeReplay(existing, input); err != nil {
				return HeartbeatWakeAcceptanceResult{}, err
			}
			if err = tx.Commit(); err != nil {
				return HeartbeatWakeAcceptanceResult{}, err
			}
			return HeartbeatWakeAcceptanceResult{Event: existing, Replayed: true}, nil
		}
	}

	configurationVersion, err := r.heartbeatConfigurationVersionTx(ctx, tx, input.AgentID)
	if err != nil {
		return HeartbeatWakeAcceptanceResult{}, err
	}
	if input.ExpectedConfigurationVersion != nil && configurationVersion != *input.ExpectedConfigurationVersion {
		return HeartbeatWakeAcceptanceResult{}, automationdomain.ErrConfigurationVersionConflict
	}
	payload := map[string]any{
		"agent_id":                       input.AgentID,
		"wake_mode":                      input.Mode,
		"accepted_configuration_version": configurationVersion,
	}
	if input.Text != nil && strings.TrimSpace(*input.Text) != "" {
		payload["text"] = strings.TrimSpace(*input.Text)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return HeartbeatWakeAcceptanceResult{}, err
	}
	inserted, err := tx.ExecContext(ctx, fmt.Sprintf(`INSERT INTO automation_system_events (
    event_id, event_type, source_type, source_id, payload, status,
    owner_user_id, request_id, intent_digest, accepted_configuration_version,
    created_at, updated_at
) VALUES (%s)
ON CONFLICT DO NOTHING`, r.bindList(12)),
		input.EventID,
		heartbeatWakeEventType,
		"heartbeat",
		input.AgentID,
		string(body),
		"new",
		nullString(input.OwnerUserID),
		nullString(input.RequestID),
		nullString(input.IntentDigest),
		configurationVersion,
		input.AcceptedAt,
		input.AcceptedAt,
	)
	if err != nil {
		return HeartbeatWakeAcceptanceResult{}, err
	}
	count, err := inserted.RowsAffected()
	if err != nil {
		return HeartbeatWakeAcceptanceResult{}, err
	}
	if count != 1 {
		if input.RequestID == "" {
			return HeartbeatWakeAcceptanceResult{}, errors.New("heartbeat wake event_id already exists")
		}
		existing, found, loadErr := r.getHeartbeatWakeByRequestTx(
			ctx, tx, input.OwnerUserID, input.RequestID,
		)
		if loadErr != nil {
			return HeartbeatWakeAcceptanceResult{}, loadErr
		}
		if !found {
			return HeartbeatWakeAcceptanceResult{}, errors.New("heartbeat wake request conflict has no durable row")
		}
		if err = validateHeartbeatWakeReplay(existing, input); err != nil {
			return HeartbeatWakeAcceptanceResult{}, err
		}
		if err = tx.Commit(); err != nil {
			return HeartbeatWakeAcceptanceResult{}, err
		}
		return HeartbeatWakeAcceptanceResult{Event: existing, Replayed: true}, nil
	}
	event := automationdomain.SystemEvent{
		EventID: input.EventID, EventType: heartbeatWakeEventType,
		SourceType: "heartbeat", SourceID: input.AgentID, Payload: string(body), Status: "new",
		OwnerUserID: input.OwnerUserID, RequestID: input.RequestID, IntentDigest: input.IntentDigest,
		AcceptedConfigurationVersion: configurationVersion, CreatedAt: input.AcceptedAt,
	}
	if err = tx.Commit(); err != nil {
		return HeartbeatWakeAcceptanceResult{}, err
	}
	return HeartbeatWakeAcceptanceResult{Event: event}, nil
}

func validateHeartbeatWakeReplay(
	event automationdomain.SystemEvent,
	input HeartbeatWakeAcceptanceInput,
) error {
	if strings.TrimSpace(event.SourceID) != input.AgentID ||
		strings.TrimSpace(event.OwnerUserID) != input.OwnerUserID ||
		strings.TrimSpace(event.RequestID) != input.RequestID ||
		strings.TrimSpace(event.IntentDigest) != input.IntentDigest {
		return automationdomain.ErrHeartbeatWakeRequestConflict
	}
	var payload struct {
		Mode string `json:"wake_mode"`
		Text string `json:"text"`
	}
	if json.Unmarshal([]byte(event.Payload), &payload) != nil ||
		strings.TrimSpace(payload.Mode) != input.Mode ||
		strings.TrimSpace(payload.Text) != strings.TrimSpace(anyStringPointer(input.Text)) {
		return automationdomain.ErrHeartbeatWakeRequestConflict
	}
	return nil
}

func anyStringPointer(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (r *Repository) heartbeatConfigurationVersionTx(
	ctx context.Context,
	tx *sql.Tx,
	agentID string,
) (int64, error) {
	query := `SELECT configuration_version FROM automation_heartbeat_states WHERE agent_id = ` + r.bind(1)
	if r.isPostgres {
		query += ` FOR UPDATE`
	}
	var version int64
	err := tx.QueryRowContext(ctx, query, strings.TrimSpace(agentID)).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return version, err
}

// GetHeartbeatWakeByRequest 读取 owner/request 对应的 durable wake 事实。
func (r *Repository) GetHeartbeatWakeByRequest(
	ctx context.Context,
	ownerUserID string,
	requestID string,
) (*automationdomain.SystemEvent, error) {
	row := r.db.QueryRowContext(ctx, heartbeatWakeByRequestQuery(r), strings.TrimSpace(ownerUserID), strings.TrimSpace(requestID))
	event, err := scanHeartbeatWakeEvent(row)
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *Repository) getHeartbeatWakeByRequestTx(
	ctx context.Context,
	tx *sql.Tx,
	ownerUserID string,
	requestID string,
) (automationdomain.SystemEvent, bool, error) {
	event, err := scanHeartbeatWakeEvent(tx.QueryRowContext(
		ctx, heartbeatWakeByRequestQuery(r), strings.TrimSpace(ownerUserID), strings.TrimSpace(requestID),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return automationdomain.SystemEvent{}, false, nil
	}
	return event, err == nil, err
}

func heartbeatWakeByRequestQuery(r *Repository) string {
	return `SELECT event_id, event_type, source_type, source_id, payload, status,
       COALESCE(owner_user_id, ''), COALESCE(request_id, ''), COALESCE(intent_digest, ''),
       COALESCE(accepted_configuration_version, 0), COALESCE(claim_token, ''),
       claim_expires_at, created_at
FROM automation_system_events
WHERE event_type = 'heartbeat.wake' AND owner_user_id = ` + r.bind(1) + ` AND request_id = ` + r.bind(2)
}

type heartbeatWakeScanner interface {
	Scan(...any) error
}

func scanHeartbeatWakeEvent(row heartbeatWakeScanner) (automationdomain.SystemEvent, error) {
	var event automationdomain.SystemEvent
	var claimExpiresAt sql.NullTime
	err := row.Scan(
		&event.EventID, &event.EventType, &event.SourceType, &event.SourceID,
		&event.Payload, &event.Status, &event.OwnerUserID, &event.RequestID,
		&event.IntentDigest, &event.AcceptedConfigurationVersion, &event.ClaimToken,
		&claimExpiresAt, &event.CreatedAt,
	)
	if claimExpiresAt.Valid {
		value := claimExpiresAt.Time.UTC()
		event.ClaimExpiresAt = &value
	}
	return event, err
}

// ListClaimableSystemEventsByAgent 只返回尚未开始消费的 new 事件。
func (r *Repository) ListClaimableSystemEventsByAgent(
	ctx context.Context,
	agentID string,
	_ time.Time,
) ([]automationdomain.SystemEvent, error) {
	query := `SELECT event_id, event_type, source_type, source_id, payload, status,
       COALESCE(owner_user_id, ''), COALESCE(request_id, ''), COALESCE(intent_digest, ''),
       COALESCE(accepted_configuration_version, 0), COALESCE(claim_token, ''),
       claim_expires_at, created_at
FROM automation_system_events
WHERE status = 'new'`
	if r.isPostgres {
		query += ` AND (source_id = ` + r.bind(1) + ` OR payload::jsonb->>'agent_id' = ` + r.bind(2) + `)`
	} else {
		query += ` AND (source_id = ` + r.bind(1) + ` OR json_extract(payload, '$.agent_id') = ` + r.bind(2) + `)`
	}
	query += ` ORDER BY created_at ASC, event_id ASC`
	rows, err := r.db.QueryContext(ctx, query, strings.TrimSpace(agentID), strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]automationdomain.SystemEvent, 0)
	for rows.Next() {
		item, scanErr := scanHeartbeatWakeEvent(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ClaimHeartbeatWakeEvent 以租约唯一领取 accepted wake；processing 不得重新领取。
func (r *Repository) ClaimHeartbeatWakeEvent(
	ctx context.Context,
	eventID string,
	claimToken string,
	_ time.Time,
	claimExpiresAt time.Time,
) (bool, error) {
	result, err := r.execWithRetry(ctx, `UPDATE automation_system_events
SET status = 'processing', claim_token = `+r.bind(1)+`, claim_expires_at = `+r.bind(2)+`, updated_at = CURRENT_TIMESTAMP
WHERE event_id = `+r.bind(3)+` AND event_type = 'heartbeat.wake'
  AND status = 'new'`,
		strings.TrimSpace(claimToken), claimExpiresAt.UTC(), strings.TrimSpace(eventID))
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

// CompleteHeartbeatWakeEvent 仅由 exact claim 把 wake 收口为 processed/failed。
func (r *Repository) CompleteHeartbeatWakeEvent(
	ctx context.Context,
	eventID string,
	claimToken string,
	status string,
) (bool, error) {
	status = strings.TrimSpace(status)
	if status != "processed" && status != "failed" {
		return false, errors.New("heartbeat wake terminal status is invalid")
	}
	result, err := r.execWithRetry(ctx, `UPDATE automation_system_events
SET status = `+r.bind(1)+`, processed_at = CURRENT_TIMESTAMP,
    claim_token = NULL, claim_expires_at = NULL, updated_at = CURRENT_TIMESTAMP
WHERE event_id = `+r.bind(2)+` AND event_type = 'heartbeat.wake'
  AND status = 'processing' AND claim_token = `+r.bind(3),
		status, strings.TrimSpace(eventID), strings.TrimSpace(claimToken))
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

// ClaimSystemEvent 原子领取非 wake 系统事件，避免两个实例同时消费 new 行。
func (r *Repository) ClaimSystemEvent(ctx context.Context, eventID string) (bool, error) {
	result, err := r.execWithRetry(ctx, `UPDATE automation_system_events
SET status = 'processing', updated_at = CURRENT_TIMESTAMP
WHERE event_id = `+r.bind(1)+` AND status = 'new'`, strings.TrimSpace(eventID))
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

// NextRecoverableHeartbeatWakeAt 返回 immediate acceptance 或 processing claim 到期的最早 deadline。
func (r *Repository) NextRecoverableHeartbeatWakeAt(ctx context.Context) (*time.Time, error) {
	modeExpr := `COALESCE(json_extract(payload, '$.wake_mode'), 'now')`
	if r.isPostgres {
		modeExpr = `COALESCE(payload::jsonb->>'wake_mode', 'now')`
	}
	query := `SELECT MIN(CASE WHEN status = 'new' THEN created_at ELSE claim_expires_at END)
FROM automation_system_events
WHERE event_type = 'heartbeat.wake'
  AND ((status = 'new' AND ` + modeExpr + ` = 'now')
       OR (status = 'processing' AND claim_expires_at IS NOT NULL))`
	var value any
	if err := r.db.QueryRowContext(ctx, query).Scan(&value); err != nil {
		return nil, err
	}
	return storagebase.NullableTime(value)
}

// ListRecoverableHeartbeatWakeAgentIDs 返回当前尚未开始的 immediate wake agent。
func (r *Repository) ListRecoverableHeartbeatWakeAgentIDs(
	ctx context.Context,
	now time.Time,
) ([]string, error) {
	modeExpr := `COALESCE(json_extract(payload, '$.wake_mode'), 'now')`
	if r.isPostgres {
		modeExpr = `COALESCE(payload::jsonb->>'wake_mode', 'now')`
	}
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT source_id
FROM automation_system_events
WHERE event_type = 'heartbeat.wake'
  AND status = 'new' AND `+modeExpr+` = 'now' AND created_at <= `+r.bind(1)+`
ORDER BY source_id ASC`, now.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]string, 0)
	for rows.Next() {
		var agentID string
		if err = rows.Scan(&agentID); err != nil {
			return nil, err
		}
		if agentID = strings.TrimSpace(agentID); agentID != "" {
			items = append(items, agentID)
		}
	}
	return items, rows.Err()
}

// FailExpiredHeartbeatWakeClaims 把结果未知的过期 processing claim 收口为 failed；
// 它绝不把已经开始的 wake 放回 new 或重新派发。
func (r *Repository) FailExpiredHeartbeatWakeClaims(ctx context.Context, now time.Time) (int64, error) {
	result, err := r.execWithRetry(ctx, `UPDATE automation_system_events
SET status = 'failed', processed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE event_type = 'heartbeat.wake' AND status = 'processing'
  AND claim_expires_at IS NOT NULL AND claim_expires_at <= `+r.bind(1), now.UTC())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ListPendingHeartbeatWakeAgentIDs 返回启动时需要恢复 PendingWake 的 durable agent 集合。
func (r *Repository) ListPendingHeartbeatWakeAgentIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT source_id
FROM automation_system_events
WHERE event_type = 'heartbeat.wake' AND status IN ('new', 'processing')
ORDER BY source_id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]string, 0)
	for rows.Next() {
		var agentID string
		if err = rows.Scan(&agentID); err != nil {
			return nil, err
		}
		if agentID = strings.TrimSpace(agentID); agentID != "" {
			items = append(items, agentID)
		}
	}
	return items, rows.Err()
}

func (r *Repository) lockHeartbeatAgentTx(
	ctx context.Context,
	tx *sql.Tx,
	agentID string,
	ownerUserID string,
) error {
	agentID = strings.TrimSpace(agentID)
	ownerUserID = strings.TrimSpace(ownerUserID)
	if r.isPostgres {
		query := `SELECT id FROM agents WHERE id = ` + r.bind(1)
		args := []any{agentID}
		if ownerUserID != "" {
			args = append(args, ownerUserID)
			query += ` AND owner_user_id = ` + r.bind(2)
		}
		query += ` FOR UPDATE`
		var storedID string
		return tx.QueryRowContext(ctx, query, args...).Scan(&storedID)
	}
	query := `UPDATE agents SET id = id WHERE id = ` + r.bind(1)
	args := []any{agentID}
	if ownerUserID != "" {
		args = append(args, ownerUserID)
		query += ` AND owner_user_id = ` + r.bind(2)
	}
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return sql.ErrNoRows
	}
	return nil
}

// lockHeartbeatConfigurationTx 优先锁定既有 heartbeat 行，与 Agent 删除保持
// heartbeat→Agent 的一致顺序；仅首次配置没有行时才用稳定 Agent 行作栅栏。
func (r *Repository) lockHeartbeatConfigurationTx(
	ctx context.Context,
	tx *sql.Tx,
	agentID string,
	ownerUserID string,
) error {
	agentID = strings.TrimSpace(agentID)
	ownerUserID = strings.TrimSpace(ownerUserID)
	found := false
	if r.isPostgres {
		var storedAgentID string
		err := tx.QueryRowContext(ctx, `SELECT agent_id FROM automation_heartbeat_states
WHERE agent_id = `+r.bind(1)+` FOR UPDATE`, agentID).Scan(&storedAgentID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		found = err == nil
	} else {
		result, err := tx.ExecContext(ctx, `UPDATE automation_heartbeat_states
SET agent_id = agent_id WHERE agent_id = `+r.bind(1), agentID)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		found = count == 1
	}
	if !found {
		return r.lockHeartbeatAgentTx(ctx, tx, agentID, ownerUserID)
	}
	if ownerUserID == "" {
		return nil
	}
	var storedAgentID string
	return tx.QueryRowContext(ctx, `SELECT id FROM agents
WHERE id = `+r.bind(1)+` AND owner_user_id = `+r.bind(2), agentID, ownerUserID).Scan(&storedAgentID)
}

func (r *Repository) withHeartbeatConfigurationFence(
	ctx context.Context,
	agentID string,
	mutation func(*sql.Tx) error,
) error {
	attempts := automationWriteRetryAttempts
	if r.isPostgres {
		attempts = 1
	}
	for attempt := 0; attempt < attempts; attempt++ {
		tx, err := r.db.BeginTx(ctx, nil)
		if err == nil {
			err = r.lockHeartbeatConfigurationTx(ctx, tx, agentID, "")
		}
		if err == nil {
			err = mutation(tx)
		}
		if err == nil {
			err = tx.Commit()
		} else if tx != nil {
			_ = tx.Rollback()
		}
		if err == nil || !isSQLiteLockedError(err) || attempt == attempts-1 {
			return err
		}
		timer := time.NewTimer(automationWriteRetryDelay * time.Duration(attempt+1))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return errors.New("heartbeat configuration fence retries exhausted")
}
