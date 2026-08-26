// INPUT: owner-scoped runtime command request_id、actor、operation 与 plan digest。
// OUTPUT: 原子 claim、已完成结果重放或不可安全重放的 uncertain 状态。
// POS: Agent-facing Automation command 副作用命令的 durable idempotency ledger。
package automation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

const (
	runtimeCommandStatusStarted   = "started"
	runtimeCommandStatusApplied   = "applied"
	runtimeCommandStatusUncertain = "uncertain"
)

type RuntimeCommandRecord struct {
	OwnerUserID       string
	RequestID         string
	ActorAgentID      string
	Operation         string
	IntentDigest      string
	ApprovalRequestID string
	Status            string
	ResultJSON        string
	ErrorMessage      string
}

// ClaimRuntimeCommand 原子认领命令；相同已完成意图返回持久结果，started/uncertain fail closed。
func (r *Repository) ClaimRuntimeCommand(
	ctx context.Context,
	record RuntimeCommandRecord,
) (*RuntimeCommandRecord, bool, error) {
	record.OwnerUserID = strings.TrimSpace(record.OwnerUserID)
	record.RequestID = strings.TrimSpace(record.RequestID)
	record.ActorAgentID = strings.TrimSpace(record.ActorAgentID)
	record.Operation = strings.TrimSpace(record.Operation)
	record.IntentDigest = strings.TrimSpace(record.IntentDigest)
	record.ApprovalRequestID = strings.TrimSpace(record.ApprovalRequestID)
	if record.OwnerUserID == "" || record.RequestID == "" || record.ActorAgentID == "" ||
		record.Operation == "" || record.IntentDigest == "" || record.ApprovalRequestID == "" {
		return nil, false, errors.New("runtime command claim is incomplete")
	}
	result, err := r.db.ExecContext(
		ctx,
		fmt.Sprintf(`INSERT INTO automation_runtime_commands (
    owner_user_id, request_id, actor_agent_id, operation, intent_digest, approval_request_id, status,
    created_at, updated_at
) VALUES (%s,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
ON CONFLICT(owner_user_id, request_id) DO NOTHING`, r.bindList(7)),
		record.OwnerUserID,
		record.RequestID,
		record.ActorAgentID,
		record.Operation,
		record.IntentDigest,
		record.ApprovalRequestID,
		runtimeCommandStatusStarted,
	)
	if err != nil {
		return nil, false, err
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return nil, false, err
	}
	if inserted == 1 {
		record.Status = runtimeCommandStatusStarted
		return &record, true, nil
	}
	existing, err := r.GetRuntimeCommand(ctx, record.OwnerUserID, record.RequestID)
	if err != nil {
		return nil, false, err
	}
	if existing.ActorAgentID != record.ActorAgentID || existing.Operation != record.Operation ||
		existing.IntentDigest != record.IntentDigest {
		return nil, false, automationdomain.ErrRuntimeCommandConflict
	}
	switch existing.Status {
	case runtimeCommandStatusApplied:
		return existing, false, nil
	case runtimeCommandStatusStarted, runtimeCommandStatusUncertain:
		return existing, false, automationdomain.ErrRuntimeCommandUncertain
	default:
		return existing, false, automationdomain.ErrRuntimeCommandUncertain
	}
}

func (r *Repository) GetRuntimeCommand(
	ctx context.Context,
	ownerUserID string,
	requestID string,
) (*RuntimeCommandRecord, error) {
	var result RuntimeCommandRecord
	var resultJSON sql.NullString
	var errorMessage sql.NullString
	err := r.db.QueryRowContext(
		ctx,
		fmt.Sprintf(`SELECT owner_user_id, request_id, actor_agent_id, operation,
       intent_digest, approval_request_id, status, result_json, error_message
FROM automation_runtime_commands
WHERE owner_user_id = %s AND request_id = %s`, r.bind(1), r.bind(2)),
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(requestID),
	).Scan(
		&result.OwnerUserID,
		&result.RequestID,
		&result.ActorAgentID,
		&result.Operation,
		&result.IntentDigest,
		&result.ApprovalRequestID,
		&result.Status,
		&resultJSON,
		&errorMessage,
	)
	if err != nil {
		return nil, err
	}
	result.ResultJSON = strings.TrimSpace(resultJSON.String)
	result.ErrorMessage = strings.TrimSpace(errorMessage.String)
	return &result, nil
}

func (r *Repository) CompleteRuntimeCommand(
	ctx context.Context,
	ownerUserID string,
	requestID string,
	intentDigest string,
	resultJSON string,
) error {
	result, err := r.db.ExecContext(
		ctx,
		fmt.Sprintf(`UPDATE automation_runtime_commands
SET status = %s, result_json = %s, error_message = NULL, updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = %s AND request_id = %s AND intent_digest = %s AND status = %s`,
			r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6)),
		runtimeCommandStatusApplied,
		resultJSON,
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(requestID),
		strings.TrimSpace(intentDigest),
		runtimeCommandStatusStarted,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return automationdomain.ErrRuntimeCommandUncertain
	}
	return nil
}

func (r *Repository) MarkRuntimeCommandUncertain(
	ctx context.Context,
	ownerUserID string,
	requestID string,
	intentDigest string,
	cause error,
) error {
	message := ""
	if cause != nil {
		message = strings.TrimSpace(cause.Error())
	}
	_, err := r.db.ExecContext(
		ctx,
		fmt.Sprintf(`UPDATE automation_runtime_commands
SET status = %s, error_message = %s, updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = %s AND request_id = %s AND intent_digest = %s AND status = %s`,
			r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6)),
		runtimeCommandStatusUncertain,
		message,
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(requestID),
		strings.TrimSpace(intentDigest),
		runtimeCommandStatusStarted,
	)
	return err
}
