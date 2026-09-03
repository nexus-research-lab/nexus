// INPUT: canonical direct-user queue admissions and dispatch-time bindings.
// OUTPUT: idempotent durable records plus one-time claim/release/consume transitions.
// POS: host DB trust root for conversational configuration queue provenance.
package queueadmission

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

var ErrAdmissionConflict = errors.New("queue admission binding conflict")

// Repository stores configuration-capable queue provenance outside Agent workspaces.
type Repository struct {
	db         *sql.DB
	dialect    storage.SQLDialect
	isPostgres bool
}

// NewRepository creates a queue admission repository.
func NewRepository(cfg config.Config, db *sql.DB) *Repository {
	return &Repository{
		db:         db,
		dialect:    storage.NewSQLDialect(cfg.DatabaseDriver),
		isPostgres: storage.NormalizeSQLDriver(cfg.DatabaseDriver) == "pgx",
	}
}

// Record durably admits one exact queue payload. Existing matching rows remain
// in their current state; a retry can never reopen a consumed admission.
func (r *Repository) Record(ctx context.Context, admission Admission) error {
	binding, targetsJSON, err := normalizeAdmission(admission.Binding)
	if err != nil {
		return err
	}
	principal, err := normalizePrincipalBinding(admission.Principal, binding.OwnerUserID)
	if err != nil {
		return err
	}
	if r == nil || r.db == nil {
		return errors.New("queue admission repository is not configured")
	}
	query := `INSERT INTO configuration_queue_admissions (
    owner_user_id, scope, queue_item_id, agent_id, session_key,
    room_id, conversation_id, source_message_id, principal_user_id,
    principal_auth_method, principal_auth_session_id,
    target_agent_ids_json, payload_digest, status, created_at, updated_at
) VALUES (` + r.dialect.BindList(13) + `, 'pending', ` +
		r.dialect.Bind(14) + `, ` + r.dialect.Bind(15) + `)
ON CONFLICT (owner_user_id, scope, queue_item_id, agent_id) DO NOTHING`
	now := r.dialect.TimestampValue(time.Now().UTC())
	result, err := r.execWithRetry(
		ctx,
		query,
		binding.OwnerUserID,
		string(binding.Scope),
		binding.QueueItemID,
		binding.AgentID,
		binding.SessionKey,
		binding.RoomID,
		binding.ConversationID,
		binding.SourceMessageID,
		principal.UserID,
		principal.AuthMethod,
		principal.SessionID,
		targetsJSON,
		binding.PayloadDigest,
		now,
		now,
	)
	if err != nil {
		return err
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr == nil && affected == 1 {
		return nil
	}
	existing, found, err := r.load(ctx, binding)
	if err != nil {
		return err
	}
	if !found ||
		!sameBinding(existing.Binding, binding) ||
		!samePrincipalBinding(existing.Principal, principal) {
		return ErrAdmissionConflict
	}
	return nil
}

// Claim atomically leases a matching pending admission. A row with the same
// identity but a tampered payload is permanently revoked before dispatch.
func (r *Repository) Claim(ctx context.Context, expected Binding) (Claim, bool, error) {
	expected, _, err := normalizeAdmission(expected)
	if err != nil {
		return Claim{}, false, err
	}
	if r == nil || r.db == nil {
		return Claim{}, false, errors.New("queue admission repository is not configured")
	}
	var lastErr error
	for attempt := 0; attempt < queueAdmissionWriteRetryAttempts; attempt++ {
		claim, trusted, claimErr := r.claimOnce(ctx, expected)
		if claimErr == nil || r.isPostgres || !isSQLiteLockedError(claimErr) {
			return claim, trusted, claimErr
		}
		lastErr = claimErr
		if err = waitQueueAdmissionRetry(ctx, attempt); err != nil {
			return Claim{}, false, err
		}
	}
	return Claim{}, false, lastErr
}

func (r *Repository) claimOnce(
	ctx context.Context,
	expected Binding,
) (Claim, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Claim{}, false, err
	}
	defer func() { _ = tx.Rollback() }()

	existing, found, err := r.loadWith(ctx, tx, expected)
	if err != nil || !found {
		return Claim{}, false, err
	}
	if existing.Status != StatusPending {
		return Claim{}, false, nil
	}
	if !sameBinding(existing.Binding, expected) {
		if err = r.revokeTx(ctx, tx, expected); err != nil {
			return Claim{}, false, err
		}
		if err = tx.Commit(); err != nil {
			return Claim{}, false, err
		}
		return Claim{}, false, nil
	}
	principal, principalErr := normalizePrincipalBinding(existing.Principal, expected.OwnerUserID)
	if principalErr != nil {
		if err = r.revokeTx(ctx, tx, expected); err != nil {
			return Claim{}, false, err
		}
		if err = tx.Commit(); err != nil {
			return Claim{}, false, err
		}
		return Claim{}, false, nil
	}
	token, err := newClaimToken()
	if err != nil {
		return Claim{}, false, err
	}
	now := r.dialect.TimestampValue(time.Now().UTC())
	result, err := tx.ExecContext(
		ctx,
		`UPDATE configuration_queue_admissions
SET status = 'claimed', claim_token = `+r.dialect.Bind(1)+`,
    claimed_at = `+r.dialect.Bind(2)+`, updated_at = `+r.dialect.Bind(3)+`
WHERE owner_user_id = `+r.dialect.Bind(4)+`
  AND scope = `+r.dialect.Bind(5)+`
  AND queue_item_id = `+r.dialect.Bind(6)+`
  AND agent_id = `+r.dialect.Bind(7)+`
  AND status = 'pending'`,
		token,
		now,
		now,
		expected.OwnerUserID,
		string(expected.Scope),
		expected.QueueItemID,
		expected.AgentID,
	)
	if err != nil {
		return Claim{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Claim{}, false, err
	}
	if affected != 1 {
		return Claim{}, false, nil
	}
	if err = tx.Commit(); err != nil {
		return Claim{}, false, err
	}
	return Claim{Binding: expected, Principal: principal, Token: token}, true, nil
}

// Release returns a failed dispatch's own claim to pending.
func (r *Repository) Release(ctx context.Context, claim Claim) error {
	return r.transitionClaim(ctx, claim, `status = 'pending', claim_token = '',
    claimed_at = NULL, updated_at = `+r.dialect.Bind(1), false)
}

// Consume makes a successfully dispatched claim permanently non-replayable.
func (r *Repository) Consume(ctx context.Context, claim Claim) error {
	return r.transitionClaim(ctx, claim, `status = 'consumed', consumed_at = `+
		r.dialect.Bind(1)+`, updated_at = `+r.dialect.Bind(2), true)
}

// Revoke permanently removes configuration capability from an undelivered item.
func (r *Repository) Revoke(ctx context.Context, binding Binding) error {
	binding, _, err := normalizeAdmission(binding)
	if err != nil {
		return err
	}
	if r == nil || r.db == nil {
		return errors.New("queue admission repository is not configured")
	}
	now := r.dialect.TimestampValue(time.Now().UTC())
	_, err = r.execWithRetry(
		ctx,
		`UPDATE configuration_queue_admissions
SET status = 'revoked', claim_token = '', updated_at = `+r.dialect.Bind(1)+`
WHERE owner_user_id = `+r.dialect.Bind(2)+`
  AND scope = `+r.dialect.Bind(3)+`
  AND queue_item_id = `+r.dialect.Bind(4)+`
  AND agent_id = `+r.dialect.Bind(5)+`
  AND status IN ('pending', 'claimed')`,
		now,
		binding.OwnerUserID,
		string(binding.Scope),
		binding.QueueItemID,
		binding.AgentID,
	)
	return err
}

type dbExecutor interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type storedAdmission struct {
	Admission
	Status string
}

func (r *Repository) load(ctx context.Context, key Binding) (storedAdmission, bool, error) {
	return r.loadWith(ctx, r.db, key)
}

func (r *Repository) loadWith(
	ctx context.Context,
	executor dbExecutor,
	key Binding,
) (storedAdmission, bool, error) {
	row := executor.QueryRowContext(
		ctx,
		`SELECT session_key, room_id, conversation_id, source_message_id,
       principal_user_id, principal_auth_method, principal_auth_session_id,
       target_agent_ids_json, payload_digest, status
FROM configuration_queue_admissions
WHERE owner_user_id = `+r.dialect.Bind(1)+`
  AND scope = `+r.dialect.Bind(2)+`
  AND queue_item_id = `+r.dialect.Bind(3)+`
  AND agent_id = `+r.dialect.Bind(4),
		key.OwnerUserID,
		string(key.Scope),
		key.QueueItemID,
		key.AgentID,
	)
	var (
		result      storedAdmission
		targetsJSON string
	)
	result.OwnerUserID = key.OwnerUserID
	result.Scope = key.Scope
	result.QueueItemID = key.QueueItemID
	result.AgentID = key.AgentID
	if err := row.Scan(
		&result.SessionKey,
		&result.RoomID,
		&result.ConversationID,
		&result.SourceMessageID,
		&result.Principal.UserID,
		&result.Principal.AuthMethod,
		&result.Principal.SessionID,
		&targetsJSON,
		&result.PayloadDigest,
		&result.Status,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storedAdmission{}, false, nil
		}
		return storedAdmission{}, false, err
	}
	if err := json.Unmarshal([]byte(targetsJSON), &result.TargetAgentIDs); err != nil {
		return storedAdmission{}, false, err
	}
	return result, true, nil
}

func (r *Repository) revokeTx(
	ctx context.Context,
	tx *sql.Tx,
	key Binding,
) error {
	now := r.dialect.TimestampValue(time.Now().UTC())
	_, err := tx.ExecContext(
		ctx,
		`UPDATE configuration_queue_admissions
SET status = 'revoked', claim_token = '', updated_at = `+r.dialect.Bind(1)+`
WHERE owner_user_id = `+r.dialect.Bind(2)+`
  AND scope = `+r.dialect.Bind(3)+`
  AND queue_item_id = `+r.dialect.Bind(4)+`
  AND agent_id = `+r.dialect.Bind(5)+`
  AND status = 'pending'`,
		now,
		key.OwnerUserID,
		string(key.Scope),
		key.QueueItemID,
		key.AgentID,
	)
	return err
}

func (r *Repository) transitionClaim(
	ctx context.Context,
	claim Claim,
	setClause string,
	consume bool,
) error {
	if r == nil || r.db == nil {
		return errors.New("queue admission repository is not configured")
	}
	if strings.TrimSpace(claim.Token) == "" {
		return errors.New("queue admission claim token is required")
	}
	now := r.dialect.TimestampValue(time.Now().UTC())
	args := []any{now}
	nextIndex := 2
	if consume {
		args = append(args, now)
		nextIndex = 3
	}
	query := `UPDATE configuration_queue_admissions
SET ` + setClause + `
WHERE owner_user_id = ` + r.dialect.Bind(nextIndex) + `
  AND scope = ` + r.dialect.Bind(nextIndex+1) + `
  AND queue_item_id = ` + r.dialect.Bind(nextIndex+2) + `
  AND agent_id = ` + r.dialect.Bind(nextIndex+3) + `
  AND claim_token = ` + r.dialect.Bind(nextIndex+4) + `
  AND status = 'claimed'`
	args = append(
		args,
		claim.OwnerUserID,
		string(claim.Scope),
		claim.QueueItemID,
		claim.AgentID,
		claim.Token,
	)
	result, err := r.execWithRetry(ctx, query, args...)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return errors.New("queue admission claim is no longer current")
	}
	return nil
}

func normalizeAdmission(binding Binding) (Binding, string, error) {
	scope, err := canonicalAdmissionScope(binding.Scope)
	if err != nil {
		return Binding{}, "", err
	}
	binding.OwnerUserID = strings.TrimSpace(binding.OwnerUserID)
	binding.QueueItemID = strings.TrimSpace(binding.QueueItemID)
	binding.AgentID = strings.TrimSpace(binding.AgentID)
	binding.SessionKey = strings.TrimSpace(binding.SessionKey)
	binding.RoomID = strings.TrimSpace(binding.RoomID)
	binding.ConversationID = strings.TrimSpace(binding.ConversationID)
	binding.SourceMessageID = strings.TrimSpace(binding.SourceMessageID)
	binding.PayloadDigest = strings.TrimSpace(binding.PayloadDigest)
	binding.Scope = scope
	binding.TargetAgentIDs = normalizedTargets("", binding.TargetAgentIDs)
	if err := binding.Validate(); err != nil {
		return Binding{}, "", err
	}
	targets, err := json.Marshal(binding.TargetAgentIDs)
	if err != nil {
		return Binding{}, "", err
	}
	return binding, string(targets), nil
}

func sameBinding(left Binding, right Binding) bool {
	return strings.TrimSpace(left.OwnerUserID) == strings.TrimSpace(right.OwnerUserID) &&
		left.Scope == right.Scope &&
		strings.TrimSpace(left.QueueItemID) == strings.TrimSpace(right.QueueItemID) &&
		strings.TrimSpace(left.AgentID) == strings.TrimSpace(right.AgentID) &&
		strings.TrimSpace(left.SessionKey) == strings.TrimSpace(right.SessionKey) &&
		strings.TrimSpace(left.RoomID) == strings.TrimSpace(right.RoomID) &&
		strings.TrimSpace(left.ConversationID) == strings.TrimSpace(right.ConversationID) &&
		strings.TrimSpace(left.SourceMessageID) == strings.TrimSpace(right.SourceMessageID) &&
		strings.TrimSpace(left.PayloadDigest) == strings.TrimSpace(right.PayloadDigest) &&
		reflect.DeepEqual(
			normalizedTargets("", left.TargetAgentIDs),
			normalizedTargets("", right.TargetAgentIDs),
		)
}

func normalizePrincipalBinding(
	principal PrincipalBinding,
	ownerUserID string,
) (PrincipalBinding, error) {
	principal.UserID = strings.TrimSpace(principal.UserID)
	principal.AuthMethod = strings.TrimSpace(principal.AuthMethod)
	principal.SessionID = strings.TrimSpace(principal.SessionID)
	ownerUserID = strings.TrimSpace(ownerUserID)
	if principal.UserID == "" || principal.UserID != ownerUserID {
		return PrincipalBinding{}, errors.New("queue admission principal must match owner_user_id")
	}
	switch principal.AuthMethod {
	case authctx.AuthMethodPassword:
		if principal.SessionID == "" {
			return PrincipalBinding{}, errors.New("password queue admission requires an auth session id")
		}
	case authctx.AuthMethodLocal:
		if principal.UserID != authctx.SystemUserID || principal.SessionID != "" {
			return PrincipalBinding{}, errors.New("local queue admission requires the sessionless system owner")
		}
	default:
		return PrincipalBinding{}, errors.New("queue admission auth method is not trusted")
	}
	return principal, nil
}

func samePrincipalBinding(left PrincipalBinding, right PrincipalBinding) bool {
	return strings.TrimSpace(left.UserID) == strings.TrimSpace(right.UserID) &&
		strings.TrimSpace(left.AuthMethod) == strings.TrimSpace(right.AuthMethod) &&
		strings.TrimSpace(left.SessionID) == strings.TrimSpace(right.SessionID)
}

func newClaimToken() (string, error) {
	buffer := make([]byte, 24)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate queue admission claim token: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

const (
	queueAdmissionWriteRetryAttempts = 8
	queueAdmissionWriteRetryDelay    = 25 * time.Millisecond
)

func (r *Repository) execWithRetry(
	ctx context.Context,
	query string,
	args ...any,
) (sql.Result, error) {
	if r.isPostgres {
		return r.db.ExecContext(ctx, query, args...)
	}
	var lastErr error
	for attempt := 0; attempt < queueAdmissionWriteRetryAttempts; attempt++ {
		result, err := r.db.ExecContext(ctx, query, args...)
		if err == nil || !isSQLiteLockedError(err) {
			return result, err
		}
		lastErr = err
		if err = waitQueueAdmissionRetry(ctx, attempt); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func waitQueueAdmissionRetry(ctx context.Context, attempt int) error {
	timer := time.NewTimer(queueAdmissionWriteRetryDelay * time.Duration(attempt+1))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func isSQLiteLockedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "database is locked") ||
		strings.Contains(strings.ToLower(err.Error()), "sqlite_busy")
}
