// INPUT: protocol.SandboxPolicyReceiptSnapshot 与 owner-scoped lifecycle transition。
// OUTPUT: 可跨进程/重启读取的 desktop sandbox effective-policy receipt。
// POS: storage 只保存事实快照；不把 receipt 当成授权或 OS 隔离证明。
package sandbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

var (
	ErrInvalidReceipt = errors.New("invalid desktop sandbox policy receipt")
)

// Repository stores one immutable identity slot per owner/session/generation.
// Upserts are idempotent for retries of the same confirmed generation, while
// phase transitions never change the policy payload or generation identity.
type Repository struct {
	db      *sql.DB
	dialect storage.SQLDialect
}

func NewRepository(cfg config.Config, db *sql.DB) *Repository {
	return &Repository{db: db, dialect: storage.NewSQLDialect(cfg.DatabaseDriver)}
}

func (r *Repository) validateSnapshot(snapshot protocol.SandboxPolicyReceiptSnapshot) error {
	if r == nil || r.db == nil {
		return errors.New("desktop sandbox policy receipt repository is unavailable")
	}
	if strings.TrimSpace(snapshot.OwnerUserID) == "" || strings.TrimSpace(snapshot.SessionKey) == "" {
		return fmt.Errorf("%w: owner and session are required", ErrInvalidReceipt)
	}
	if snapshot.Version <= 0 || snapshot.Generation == 0 {
		return fmt.Errorf("%w: version and generation are required", ErrInvalidReceipt)
	}
	if strings.TrimSpace(snapshot.PolicyDigest) == "" {
		return fmt.Errorf("%w: policy digest is required", ErrInvalidReceipt)
	}
	if !validReceiptPhase(snapshot.Phase) {
		return fmt.Errorf("%w: unsupported phase %q", ErrInvalidReceipt, snapshot.Phase)
	}
	if snapshot.Phase == protocol.SandboxPolicyReceiptUnknown && strings.TrimSpace(snapshot.UnknownReason) == "" {
		return fmt.Errorf("%w: unknown phase requires a reason", ErrInvalidReceipt)
	}
	return nil
}

func validReceiptPhase(phase protocol.SandboxPolicyReceiptPhase) bool {
	switch phase {
	case protocol.SandboxPolicyReceiptConfirmed,
		protocol.SandboxPolicyReceiptRetiring,
		protocol.SandboxPolicyReceiptRetired,
		protocol.SandboxPolicyReceiptUnknown,
		protocol.SandboxPolicyReceiptReconciled:
		return true
	default:
		return false
	}
}

// receiptPhaseUpdateAllowed is intentionally monotonic. Runtime cleanup can
// race with a late callback from an older generation; a terminal observation
// must never be reopened as retiring or confirmed. Repeating the same terminal
// phase is idempotent.
func receiptPhaseUpdateAllowed(current, next protocol.SandboxPolicyReceiptPhase) bool {
	if current == next {
		return true
	}
	switch current {
	case protocol.SandboxPolicyReceiptConfirmed:
		return next == protocol.SandboxPolicyReceiptRetiring ||
			next == protocol.SandboxPolicyReceiptRetired ||
			next == protocol.SandboxPolicyReceiptUnknown
	case protocol.SandboxPolicyReceiptRetiring:
		return next == protocol.SandboxPolicyReceiptRetired ||
			next == protocol.SandboxPolicyReceiptUnknown
	case protocol.SandboxPolicyReceiptRetired,
		protocol.SandboxPolicyReceiptUnknown:
		// A later owner-process reaper failure can invalidate an earlier
		// "retired" observation. Moving retired -> unknown is a conservative
		// correction of cleanup certainty, not a reopening of the runtime.
		if current == protocol.SandboxPolicyReceiptRetired {
			return next == protocol.SandboxPolicyReceiptUnknown
		}
		return false
	case protocol.SandboxPolicyReceiptReconciled:
		return false
	default:
		return false
	}
}

func (r *Repository) encodeCapabilities(values []string) (string, error) {
	if values == nil {
		values = []string{}
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("encode sandbox receipt capabilities: %w", err)
	}
	return string(encoded), nil
}

// Save inserts one exact receipt identity. A retry of a still-confirmed row may
// refresh its observation timestamp, but the generation payload and original
// confirmation time are immutable; phase reconciliation remains explicit
// through UpdatePhase. Terminal rows are left untouched by late retries.
func (r *Repository) Save(ctx context.Context, snapshot protocol.SandboxPolicyReceiptSnapshot) error {
	if err := r.validateSnapshot(snapshot); err != nil {
		return err
	}
	required, err := r.encodeCapabilities(snapshot.RequiredCapabilities)
	if err != nil {
		return err
	}
	acknowledged, err := r.encodeCapabilities(snapshot.AcknowledgedCapabilities)
	if err != nil {
		return err
	}
	if snapshot.ConfirmedAt.IsZero() {
		snapshot.ConfirmedAt = time.Now().UTC()
	}
	if snapshot.UpdatedAt.IsZero() {
		snapshot.UpdatedAt = snapshot.ConfirmedAt
	}
	query := `INSERT INTO sandbox_policy_receipts (
owner_user_id, session_key, generation, version, session_id,
 session_id_provisional, runtime_kind, round_id, policy_digest,
 required_capabilities_json, acknowledged_capabilities_json,
 capability_evidence, isolation_evidence, resource_policy_json, lease_id,
 phase, unknown_reason, confirmed_at, updated_at
) VALUES (` + r.dialect.BindList(19) + `)
ON CONFLICT(owner_user_id, session_key, generation) DO UPDATE SET
	-- A retry of the original connect receipt must never reopen a retired or
	-- unknown generation. The exact generation payload is immutable; lifecycle
	-- transitions are owned by UpdatePhase.
phase = sandbox_policy_receipts.phase,
unknown_reason = sandbox_policy_receipts.unknown_reason,
confirmed_at = sandbox_policy_receipts.confirmed_at,
updated_at = CASE
	WHEN excluded.updated_at > sandbox_policy_receipts.updated_at THEN excluded.updated_at
	ELSE sandbox_policy_receipts.updated_at
END
-- Connect retries may refresh a still-confirmed row, but a lifecycle terminal
-- row is immutable. In particular, a late retry must not replace the policy
-- payload or lease identity after cleanup has become unknown/retired.
WHERE sandbox_policy_receipts.phase = 'confirmed'`
	_, err = r.db.ExecContext(ctx, query,
		snapshot.OwnerUserID, snapshot.SessionKey, snapshot.Generation,
		snapshot.Version, snapshot.SessionID, snapshot.SessionIDProvisional,
		snapshot.RuntimeKind, snapshot.RoundID, snapshot.PolicyDigest,
		required, acknowledged, snapshot.CapabilityEvidence,
		snapshot.IsolationEvidence, snapshot.ResourcePolicyJSON, snapshot.LeaseID,
		string(snapshot.Phase), snapshot.UnknownReason,
		r.dialect.TimestampValue(snapshot.ConfirmedAt),
		r.dialect.TimestampValue(snapshot.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("save desktop sandbox policy receipt: %w", err)
	}
	return nil
}

// UpdatePhase records a lifecycle transition without changing the policy
// payload. Unknown transitions require a reason so a later reconciliation can
// explain why automatic cleanup was not attempted.
func (r *Repository) UpdatePhase(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
	generation uint64,
	phase protocol.SandboxPolicyReceiptPhase,
	unknownReason string,
) error {
	if r == nil || r.db == nil {
		return errors.New("desktop sandbox policy receipt repository is unavailable")
	}
	ownerUserID = strings.TrimSpace(ownerUserID)
	sessionKey = strings.TrimSpace(sessionKey)
	if ownerUserID == "" || sessionKey == "" || generation == 0 {
		return fmt.Errorf("%w: owner, session and generation are required", ErrInvalidReceipt)
	}
	if !validReceiptPhase(phase) {
		return fmt.Errorf("%w: unsupported phase %q", ErrInvalidReceipt, phase)
	}
	if phase == protocol.SandboxPolicyReceiptUnknown && strings.TrimSpace(unknownReason) == "" {
		return fmt.Errorf("%w: unknown phase requires a reason", ErrInvalidReceipt)
	}
	now := r.dialect.TimestampValue(time.Now().UTC())
	// Keep the transition conditional in SQL so two late cleanup goroutines
	// cannot pass a read-then-write check independently. The allowed current
	// phases are derived from the requested next phase; repeated terminal
	// updates remain idempotent.
	allowed := make([]protocol.SandboxPolicyReceiptPhase, 0, 2)
	switch phase {
	case protocol.SandboxPolicyReceiptConfirmed:
		allowed = append(allowed, protocol.SandboxPolicyReceiptConfirmed)
	case protocol.SandboxPolicyReceiptRetiring:
		allowed = append(allowed, protocol.SandboxPolicyReceiptRetiring, protocol.SandboxPolicyReceiptConfirmed)
	case protocol.SandboxPolicyReceiptRetired, protocol.SandboxPolicyReceiptUnknown:
		allowed = append(allowed, protocol.SandboxPolicyReceiptConfirmed, protocol.SandboxPolicyReceiptRetiring)
		if phase == protocol.SandboxPolicyReceiptUnknown {
			allowed = append(allowed, protocol.SandboxPolicyReceiptRetired)
		}
	case protocol.SandboxPolicyReceiptReconciled:
		// Reconciliation is intentionally not exposed by the runtime lifecycle;
		// keep the phase terminal until a future explicit human-only control
		// surface supplies evidence.
		allowed = append(allowed, protocol.SandboxPolicyReceiptReconciled)
	}
	allowedPlaceholders := make([]string, 0, len(allowed))
	args := []any{ownerUserID, sessionKey, generation, string(phase), strings.TrimSpace(unknownReason), now}
	for index, current := range allowed {
		allowedPlaceholders = append(allowedPlaceholders, r.dialect.Bind(7+index))
		args = append(args, string(current))
	}
	if r.dialect.Bind(1) == "?" {
		// SQLite uses anonymous placeholders, so database/sql binds by textual
		// occurrence rather than by the numeric indexes used by PostgreSQL.
		args = append([]any{string(phase), strings.TrimSpace(unknownReason), now, ownerUserID, sessionKey, generation}, args[6:]...)
	}
	query := `UPDATE sandbox_policy_receipts SET phase = ` + r.dialect.Bind(4) +
		`, unknown_reason = ` + r.dialect.Bind(5) + `, updated_at = ` + r.dialect.Bind(6) +
		` WHERE owner_user_id = ` + r.dialect.Bind(1) + ` AND session_key = ` + r.dialect.Bind(2) +
		` AND generation = ` + r.dialect.Bind(3) + ` AND phase IN (` + strings.Join(allowedPlaceholders, ",") + `)`
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update desktop sandbox policy receipt phase: %w", err)
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr == nil && affected == 0 {
		// A missing row is an actual lifecycle error. An existing terminal row
		// is a stale callback and is deliberately treated as an idempotent no-op.
		var current string
		lookup := `SELECT phase FROM sandbox_policy_receipts WHERE owner_user_id = ` + r.dialect.Bind(1) +
			` AND session_key = ` + r.dialect.Bind(2) + ` AND generation = ` + r.dialect.Bind(3)
		if lookupErr := r.db.QueryRowContext(ctx, lookup, ownerUserID, sessionKey, generation).Scan(&current); errors.Is(lookupErr, sql.ErrNoRows) {
			return sql.ErrNoRows
		} else if lookupErr != nil {
			return fmt.Errorf("read desktop sandbox policy receipt phase: %w", lookupErr)
		} else if !receiptPhaseUpdateAllowed(protocol.SandboxPolicyReceiptPhase(current), phase) {
			return nil
		}
	}
	return nil
}

// Latest reads only the newest generation for the exact owner/session. It is
// intentionally owner-scoped so a session key cannot become a cross-owner
// lookup primitive.
func (r *Repository) Latest(ctx context.Context, ownerUserID, sessionKey string) (protocol.SandboxPolicyReceiptSnapshot, bool, error) {
	if r == nil || r.db == nil {
		return protocol.SandboxPolicyReceiptSnapshot{}, false, errors.New("desktop sandbox policy receipt repository is unavailable")
	}
	ownerUserID = strings.TrimSpace(ownerUserID)
	sessionKey = strings.TrimSpace(sessionKey)
	if ownerUserID == "" || sessionKey == "" {
		return protocol.SandboxPolicyReceiptSnapshot{}, false, fmt.Errorf("%w: owner and session are required", ErrInvalidReceipt)
	}
	query := `SELECT version, generation, session_id, session_id_provisional,
 runtime_kind, round_id, policy_digest, required_capabilities_json,
 acknowledged_capabilities_json, capability_evidence, isolation_evidence,
 resource_policy_json, lease_id, phase, unknown_reason, confirmed_at, updated_at
FROM sandbox_policy_receipts WHERE owner_user_id = ` + r.dialect.Bind(1) +
		` AND session_key = ` + r.dialect.Bind(2) + ` ORDER BY generation DESC LIMIT 1`
	snapshot, found, err := r.scan(ctx, query, ownerUserID, sessionKey)
	if found {
		snapshot.OwnerUserID = ownerUserID
		snapshot.SessionKey = sessionKey
	}
	return snapshot, found, err
}

func (r *Repository) Get(ctx context.Context, ownerUserID, sessionKey string, generation uint64) (protocol.SandboxPolicyReceiptSnapshot, bool, error) {
	if r == nil || r.db == nil {
		return protocol.SandboxPolicyReceiptSnapshot{}, false, errors.New("desktop sandbox policy receipt repository is unavailable")
	}
	ownerUserID = strings.TrimSpace(ownerUserID)
	sessionKey = strings.TrimSpace(sessionKey)
	if ownerUserID == "" || sessionKey == "" || generation == 0 {
		return protocol.SandboxPolicyReceiptSnapshot{}, false, fmt.Errorf("%w: owner, session and generation are required", ErrInvalidReceipt)
	}
	query := `SELECT version, generation, session_id, session_id_provisional,
 runtime_kind, round_id, policy_digest, required_capabilities_json,
 acknowledged_capabilities_json, capability_evidence, isolation_evidence,
 resource_policy_json, lease_id, phase, unknown_reason, confirmed_at, updated_at
FROM sandbox_policy_receipts WHERE owner_user_id = ` + r.dialect.Bind(1) +
		` AND session_key = ` + r.dialect.Bind(2) + ` AND generation = ` + r.dialect.Bind(3)
	snapshot, found, err := r.scan(ctx, query, ownerUserID, sessionKey, generation)
	if found {
		snapshot.OwnerUserID = ownerUserID
		snapshot.SessionKey = sessionKey
	}
	return snapshot, found, err
}

func (r *Repository) scan(ctx context.Context, query string, args ...any) (protocol.SandboxPolicyReceiptSnapshot, bool, error) {
	var snapshot protocol.SandboxPolicyReceiptSnapshot
	var provisional bool
	var required, acknowledged string
	var confirmedAt, updatedAt any
	var generation int64
	var phase string
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&snapshot.Version, &generation, &snapshot.SessionID, &provisional,
		&snapshot.RuntimeKind, &snapshot.RoundID, &snapshot.PolicyDigest,
		&required, &acknowledged, &snapshot.CapabilityEvidence,
		&snapshot.IsolationEvidence, &snapshot.ResourcePolicyJSON,
		&snapshot.LeaseID, &phase, &snapshot.UnknownReason,
		&confirmedAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return protocol.SandboxPolicyReceiptSnapshot{}, false, nil
	}
	if err != nil {
		return protocol.SandboxPolicyReceiptSnapshot{}, false, fmt.Errorf("read desktop sandbox policy receipt: %w", err)
	}
	if generation <= 0 {
		return protocol.SandboxPolicyReceiptSnapshot{}, false, fmt.Errorf("%w: invalid persisted generation", ErrInvalidReceipt)
	}
	snapshot.Generation = uint64(generation)
	snapshot.SessionIDProvisional = provisional
	snapshot.Phase = protocol.SandboxPolicyReceiptPhase(phase)
	if !validReceiptPhase(snapshot.Phase) {
		return protocol.SandboxPolicyReceiptSnapshot{}, false, fmt.Errorf("%w: invalid persisted phase %q", ErrInvalidReceipt, phase)
	}
	if err := json.Unmarshal([]byte(required), &snapshot.RequiredCapabilities); err != nil {
		return protocol.SandboxPolicyReceiptSnapshot{}, false, fmt.Errorf("%w: required capabilities JSON", ErrInvalidReceipt)
	}
	if err := json.Unmarshal([]byte(acknowledged), &snapshot.AcknowledgedCapabilities); err != nil {
		return protocol.SandboxPolicyReceiptSnapshot{}, false, fmt.Errorf("%w: acknowledged capabilities JSON", ErrInvalidReceipt)
	}
	confirmed, err := storage.NullableTime(confirmedAt)
	if err != nil {
		return protocol.SandboxPolicyReceiptSnapshot{}, false, fmt.Errorf("%w: confirmed time", ErrInvalidReceipt)
	}
	updated, err := storage.NullableTime(updatedAt)
	if err != nil {
		return protocol.SandboxPolicyReceiptSnapshot{}, false, fmt.Errorf("%w: updated time", ErrInvalidReceipt)
	}
	if confirmed != nil {
		snapshot.ConfirmedAt = *confirmed
	}
	if updated != nil {
		snapshot.UpdatedAt = *updated
	}
	return snapshot, true, nil
}
