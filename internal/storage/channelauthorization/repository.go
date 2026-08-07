// INPUT: validated flow identities, encrypted ephemeral payloads, and safe outcomes.
// OUTPUT: CAS state transitions, restart invalidation, and immutable completion audits.
// POS: database trust root for conversational Channel authorization.
package channelauthorization

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

type Repository struct {
	db      *sql.DB
	dialect storage.SQLDialect
}

func NewRepository(cfg config.Config, db *sql.DB) *Repository {
	return &Repository{db: db, dialect: storage.NewSQLDialect(cfg.DatabaseDriver)}
}

func (r *Repository) Create(ctx context.Context, flow Flow) error {
	if r == nil || r.db == nil {
		return errors.New("channel authorization repository is not configured")
	}
	if err := validateNewFlow(flow); err != nil {
		return err
	}
	query := `INSERT INTO channel_authorization_flows (
    flow_id, owner_user_id, principal_user_id, principal_role,
    principal_auth_method, principal_auth_session_id, agent_id,
    business_session_key, root_round_id,
    runtime_lease_session_key, runtime_lease_round_id, channel_type,
    account_binding, resolved_account_id, start_control_version,
    committed_control_version, flow_generation, process_generation, status,
    runtime_ref_encrypted, human_presentation_encrypted, outcome_code,
    outcome_message, expires_at, created_at, updated_at
) VALUES (` + r.dialect.BindList(26) + `)`
	now := flow.CreatedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	_, err := r.db.ExecContext(
		ctx,
		query,
		flow.FlowID,
		flow.OwnerUserID,
		flow.PrincipalUserID,
		flow.PrincipalRole,
		flow.PrincipalAuthMethod,
		flow.PrincipalAuthSessionID,
		flow.AgentID,
		flow.BusinessSessionKey,
		flow.RootRoundID,
		flow.RuntimeLeaseSessionKey,
		flow.RuntimeLeaseRoundID,
		flow.ChannelType,
		flow.AccountBinding,
		flow.ResolvedAccountID,
		flow.StartControlVersion,
		nullableVersion(flow.CommittedControlVersion),
		flow.FlowGeneration,
		flow.ProcessGeneration,
		flow.Status,
		nullableText(flow.RuntimeRefEncrypted),
		nullableText(flow.HumanPresentationEncrypted),
		flow.OutcomeCode,
		flow.OutcomeMessage,
		r.dialect.TimestampValue(flow.ExpiresAt),
		r.dialect.TimestampValue(now),
		r.dialect.TimestampValue(now),
	)
	if err != nil && isUniqueViolation(err) {
		return ErrActiveFlow
	}
	return err
}

func (r *Repository) AttachRuntime(
	ctx context.Context,
	flow Flow,
	runtimeRefEncrypted string,
	presentationEncrypted string,
	status string,
	expiresAt time.Time,
	now time.Time,
) error {
	if !IsActiveStatus(status) || status == StatusStarting {
		return errors.New("attached channel authorization status must be active")
	}
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE channel_authorization_flows
SET runtime_ref_encrypted = `+r.dialect.Bind(1)+`,
    human_presentation_encrypted = `+r.dialect.Bind(2)+`,
    status = `+r.dialect.Bind(3)+`,
    expires_at = `+r.dialect.Bind(4)+`,
    updated_at = `+r.dialect.Bind(5)+`
WHERE flow_id = `+r.dialect.Bind(6)+`
  AND owner_user_id = `+r.dialect.Bind(7)+`
  AND flow_generation = `+r.dialect.Bind(8)+`
  AND process_generation = `+r.dialect.Bind(9)+`
  AND status = 'starting'`,
		nullableText(runtimeRefEncrypted),
		nullableText(presentationEncrypted),
		status,
		r.dialect.TimestampValue(expiresAt),
		r.dialect.TimestampValue(now),
		flow.FlowID,
		flow.OwnerUserID,
		flow.FlowGeneration,
		flow.ProcessGeneration,
	)
	return expectOne(result, err)
}

func (r *Repository) UpdateProgress(
	ctx context.Context,
	flow Flow,
	status string,
	resolvedAccountID string,
	committedVersion int64,
	now time.Time,
) error {
	if status != StatusRunning && status != StatusVerifyCodeRequired {
		return errors.New("channel authorization progress status is invalid")
	}
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE channel_authorization_flows
SET status = `+r.dialect.Bind(1)+`,
    resolved_account_id = `+r.dialect.Bind(2)+`,
    committed_control_version = `+r.dialect.Bind(3)+`,
    updated_at = `+r.dialect.Bind(4)+`
WHERE flow_id = `+r.dialect.Bind(5)+`
  AND owner_user_id = `+r.dialect.Bind(6)+`
  AND flow_generation = `+r.dialect.Bind(7)+`
  AND process_generation = `+r.dialect.Bind(8)+`
  AND status IN ('starting', 'running', 'verify_code_required')`,
		status,
		strings.TrimSpace(resolvedAccountID),
		nullableVersion(committedVersion),
		r.dialect.TimestampValue(now),
		flow.FlowID,
		flow.OwnerUserID,
		flow.FlowGeneration,
		flow.ProcessGeneration,
	)
	return expectOne(result, err)
}

func (r *Repository) Get(ctx context.Context, ownerUserID string, flowID string) (*Flow, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("channel authorization repository is not configured")
	}
	query := flowSelect + `
WHERE owner_user_id = ` + r.dialect.Bind(1) + `
  AND flow_id = ` + r.dialect.Bind(2)
	flow, err := scanFlow(r.db.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(flowID),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrFlowNotFound
	}
	return flow, err
}

// GetActiveByGeneration resolves the exact in-process flow bound into a
// Channel login immediately before that login may persist credentials.
func (r *Repository) GetActiveByGeneration(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	flowGeneration string,
	processGeneration string,
) (*Flow, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("channel authorization repository is not configured")
	}
	query := flowSelect + `
WHERE owner_user_id = ` + r.dialect.Bind(1) + `
  AND channel_type = ` + r.dialect.Bind(2) + `
  AND flow_generation = ` + r.dialect.Bind(3) + `
  AND process_generation = ` + r.dialect.Bind(4) + `
  AND status IN ('starting', 'running', 'verify_code_required')`
	flow, err := scanFlow(r.db.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(channelType),
		strings.TrimSpace(flowGeneration),
		strings.TrimSpace(processGeneration),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrFlowNotFound
	}
	return flow, err
}

// ListActiveForProcess returns the bounded shutdown set after the service
// lifecycle fence has prevented every new Start and commit lease.
func (r *Repository) ListActiveForProcess(
	ctx context.Context,
	processGeneration string,
) ([]Flow, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("channel authorization repository is not configured")
	}
	rows, err := r.db.QueryContext(
		ctx,
		flowSelect+`
WHERE process_generation = `+r.dialect.Bind(1)+`
  AND status IN ('starting', 'running', 'verify_code_required')
ORDER BY created_at, flow_id`,
		strings.TrimSpace(processGeneration),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Flow, 0)
	for rows.Next() {
		flow, scanErr := scanFlow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, *flow)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *Repository) Finish(
	ctx context.Context,
	flow Flow,
	update TerminalUpdate,
) (*Flow, error) {
	if !IsTerminalStatus(update.Status) {
		return nil, errors.New("channel authorization terminal status is invalid")
	}
	if strings.TrimSpace(update.AuditID) == "" {
		return nil, errors.New("channel authorization audit_id is required")
	}
	finishedAt := update.FinishedAt.UTC()
	if finishedAt.IsZero() {
		finishedAt = time.Now().UTC()
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(
		ctx,
		`UPDATE channel_authorization_flows
SET status = `+r.dialect.Bind(1)+`,
    resolved_account_id = `+r.dialect.Bind(2)+`,
    committed_control_version = `+r.dialect.Bind(3)+`,
    outcome_code = `+r.dialect.Bind(4)+`,
    outcome_message = `+r.dialect.Bind(5)+`,
    runtime_ref_encrypted = NULL,
    human_presentation_encrypted = NULL,
    finished_at = `+r.dialect.Bind(6)+`,
    updated_at = `+r.dialect.Bind(7)+`
WHERE flow_id = `+r.dialect.Bind(8)+`
  AND owner_user_id = `+r.dialect.Bind(9)+`
  AND flow_generation = `+r.dialect.Bind(10)+`
  AND process_generation = `+r.dialect.Bind(11)+`
  AND status IN ('starting', 'running', 'verify_code_required')`,
		update.Status,
		strings.TrimSpace(update.ResolvedAccountID),
		nullableVersion(update.CommittedControlVersion),
		strings.TrimSpace(update.OutcomeCode),
		strings.TrimSpace(update.OutcomeMessage),
		r.dialect.TimestampValue(finishedAt),
		r.dialect.TimestampValue(finishedAt),
		flow.FlowID,
		flow.OwnerUserID,
		flow.FlowGeneration,
		flow.ProcessGeneration,
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	current, err := r.getWith(ctx, tx, flow.OwnerUserID, flow.FlowID)
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		if IsTerminalStatus(current.Status) {
			if err = tx.Commit(); err != nil {
				return nil, err
			}
			return current, nil
		}
		return nil, ErrFlowConflict
	}
	if err = r.insertAudit(ctx, tx, update.AuditID, *current, finishedAt); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return current, nil
}

// InvalidateStale turns every active flow from an older process generation
// into an explicit, audited terminal state. Ephemeral ciphertext is scrubbed.
func (r *Repository) InvalidateStale(
	ctx context.Context,
	processGeneration string,
	now time.Time,
	newAuditID func() (string, error),
) ([]Flow, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(
		ctx,
		flowSelect+`
WHERE process_generation <> `+r.dialect.Bind(1)+`
  AND status IN ('starting', 'running', 'verify_code_required')`,
		strings.TrimSpace(processGeneration),
	)
	if err != nil {
		return nil, err
	}
	stale := make([]Flow, 0)
	for rows.Next() {
		item, scanErr := scanFlow(rows)
		if scanErr != nil {
			_ = rows.Close()
			return nil, scanErr
		}
		stale = append(stale, *item)
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for index := range stale {
		item := &stale[index]
		result, updateErr := tx.ExecContext(
			ctx,
			`UPDATE channel_authorization_flows
SET status = 'restart_invalidated',
    outcome_code = 'server_restarted',
    outcome_message = '服务已重启；原授权已安全失效，请重新开始。',
    runtime_ref_encrypted = NULL,
    human_presentation_encrypted = NULL,
    finished_at = `+r.dialect.Bind(1)+`,
    updated_at = `+r.dialect.Bind(2)+`
WHERE flow_id = `+r.dialect.Bind(3)+`
  AND process_generation = `+r.dialect.Bind(4)+`
  AND status IN ('starting', 'running', 'verify_code_required')`,
			r.dialect.TimestampValue(now),
			r.dialect.TimestampValue(now),
			item.FlowID,
			item.ProcessGeneration,
		)
		if updateErr != nil {
			return nil, updateErr
		}
		affected, affectedErr := result.RowsAffected()
		if affectedErr != nil {
			return nil, affectedErr
		}
		if affected != 1 {
			return nil, ErrFlowConflict
		}
		item.Status = StatusRestartInvalidated
		item.OutcomeCode = "server_restarted"
		item.OutcomeMessage = "服务已重启；原授权已安全失效，请重新开始。"
		item.RuntimeRefEncrypted = ""
		item.HumanPresentationEncrypted = ""
		item.UpdatedAt = now
		item.FinishedAt = timePointer(now)
		auditID, idErr := newAuditID()
		if idErr != nil {
			return nil, idErr
		}
		if err = r.insertAudit(ctx, tx, auditID, *item, now); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return stale, nil
}

func (r *Repository) GetAudit(ctx context.Context, ownerUserID string, flowID string) (*Audit, error) {
	query := `SELECT audit_id, flow_id, owner_user_id, principal_user_id,
       principal_role, principal_auth_method, principal_auth_session_id,
       agent_id, business_session_key,
       root_round_id, runtime_lease_session_key, runtime_lease_round_id,
       channel_type, account_binding, resolved_account_id,
       start_control_version, committed_control_version, flow_generation,
       status, outcome_code, outcome_message, created_at, completed_at
FROM channel_authorization_audit
WHERE owner_user_id = ` + r.dialect.Bind(1) + `
  AND flow_id = ` + r.dialect.Bind(2)
	audit, err := scanAudit(r.db.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(flowID),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrFlowNotFound
	}
	return audit, err
}

const flowSelect = `SELECT flow_id, owner_user_id, principal_user_id,
       principal_role, principal_auth_method, principal_auth_session_id,
       agent_id, business_session_key,
       root_round_id, runtime_lease_session_key, runtime_lease_round_id,
       channel_type, account_binding, resolved_account_id,
       start_control_version, committed_control_version, flow_generation,
       process_generation, status, runtime_ref_encrypted,
       human_presentation_encrypted, outcome_code, outcome_message,
       expires_at, created_at, updated_at, finished_at
FROM channel_authorization_flows
`

type rowScanner interface {
	Scan(...any) error
}

func scanFlow(scanner rowScanner) (*Flow, error) {
	var (
		result                Flow
		committedVersion      sql.NullInt64
		runtimeRefEncrypted   sql.NullString
		presentationEncrypted sql.NullString
		finishedAt            sql.NullTime
	)
	if err := scanner.Scan(
		&result.FlowID,
		&result.OwnerUserID,
		&result.PrincipalUserID,
		&result.PrincipalRole,
		&result.PrincipalAuthMethod,
		&result.PrincipalAuthSessionID,
		&result.AgentID,
		&result.BusinessSessionKey,
		&result.RootRoundID,
		&result.RuntimeLeaseSessionKey,
		&result.RuntimeLeaseRoundID,
		&result.ChannelType,
		&result.AccountBinding,
		&result.ResolvedAccountID,
		&result.StartControlVersion,
		&committedVersion,
		&result.FlowGeneration,
		&result.ProcessGeneration,
		&result.Status,
		&runtimeRefEncrypted,
		&presentationEncrypted,
		&result.OutcomeCode,
		&result.OutcomeMessage,
		&result.ExpiresAt,
		&result.CreatedAt,
		&result.UpdatedAt,
		&finishedAt,
	); err != nil {
		return nil, err
	}
	result.CommittedControlVersion = committedVersion.Int64
	result.RuntimeRefEncrypted = runtimeRefEncrypted.String
	result.HumanPresentationEncrypted = presentationEncrypted.String
	if finishedAt.Valid {
		result.FinishedAt = timePointer(finishedAt.Time)
	}
	return &result, nil
}

func scanAudit(scanner rowScanner) (*Audit, error) {
	var (
		result           Audit
		committedVersion sql.NullInt64
	)
	if err := scanner.Scan(
		&result.AuditID,
		&result.FlowID,
		&result.OwnerUserID,
		&result.PrincipalUserID,
		&result.PrincipalRole,
		&result.PrincipalAuthMethod,
		&result.PrincipalAuthSessionID,
		&result.AgentID,
		&result.BusinessSessionKey,
		&result.RootRoundID,
		&result.RuntimeLeaseSessionKey,
		&result.RuntimeLeaseRoundID,
		&result.ChannelType,
		&result.AccountBinding,
		&result.ResolvedAccountID,
		&result.StartControlVersion,
		&committedVersion,
		&result.FlowGeneration,
		&result.Status,
		&result.OutcomeCode,
		&result.OutcomeMessage,
		&result.CreatedAt,
		&result.CompletedAt,
	); err != nil {
		return nil, err
	}
	result.CommittedControlVersion = committedVersion.Int64
	return &result, nil
}

func (r *Repository) getWith(
	ctx context.Context,
	tx *sql.Tx,
	ownerUserID string,
	flowID string,
) (*Flow, error) {
	result, err := scanFlow(tx.QueryRowContext(
		ctx,
		flowSelect+`
WHERE owner_user_id = `+r.dialect.Bind(1)+`
  AND flow_id = `+r.dialect.Bind(2),
		ownerUserID,
		flowID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrFlowNotFound
	}
	return result, err
}

func (r *Repository) insertAudit(
	ctx context.Context,
	tx *sql.Tx,
	auditID string,
	flow Flow,
	completedAt time.Time,
) error {
	query := `INSERT INTO channel_authorization_audit (
    audit_id, flow_id, owner_user_id, principal_user_id, principal_role,
    principal_auth_method, principal_auth_session_id, agent_id,
    business_session_key, root_round_id,
    runtime_lease_session_key, runtime_lease_round_id, channel_type,
    account_binding, resolved_account_id, start_control_version,
    committed_control_version, flow_generation, status, outcome_code,
    outcome_message, created_at, completed_at
) VALUES (` + r.dialect.BindList(23) + `)
ON CONFLICT (flow_id) DO NOTHING`
	_, err := tx.ExecContext(
		ctx,
		query,
		auditID,
		flow.FlowID,
		flow.OwnerUserID,
		flow.PrincipalUserID,
		flow.PrincipalRole,
		flow.PrincipalAuthMethod,
		flow.PrincipalAuthSessionID,
		flow.AgentID,
		flow.BusinessSessionKey,
		flow.RootRoundID,
		flow.RuntimeLeaseSessionKey,
		flow.RuntimeLeaseRoundID,
		flow.ChannelType,
		flow.AccountBinding,
		flow.ResolvedAccountID,
		flow.StartControlVersion,
		nullableVersion(flow.CommittedControlVersion),
		flow.FlowGeneration,
		flow.Status,
		flow.OutcomeCode,
		flow.OutcomeMessage,
		r.dialect.TimestampValue(flow.CreatedAt),
		r.dialect.TimestampValue(completedAt),
	)
	return err
}

func validateNewFlow(flow Flow) error {
	if err := flow.Binding.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(flow.FlowID) == "" ||
		strings.TrimSpace(flow.FlowGeneration) == "" ||
		strings.TrimSpace(flow.ProcessGeneration) == "" {
		return errors.New("channel authorization opaque identifiers are required")
	}
	if flow.StartControlVersion <= 0 {
		return errors.New("channel authorization start_control_version must be positive")
	}
	if flow.Status != StatusStarting {
		return errors.New("new channel authorization flow must start in starting state")
	}
	if flow.ExpiresAt.IsZero() {
		return errors.New("channel authorization expires_at is required")
	}
	return nil
}

func expectOne(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrFlowConflict
	}
	return nil
}

func nullableText(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullableVersion(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func timePointer(value time.Time) *time.Time {
	copyValue := value
	return &copyValue
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "duplicate key") ||
		strings.Contains(message, "constraint failed")
}

func (f Flow) String() string {
	return fmt.Sprintf(
		"channel authorization flow %s owner=%s channel=%s generation=%s status=%s",
		f.FlowID,
		f.OwnerUserID,
		f.ChannelType,
		f.FlowGeneration,
		f.Status,
	)
}
