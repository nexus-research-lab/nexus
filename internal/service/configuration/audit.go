// INPUT: 已脱敏的变更请求、执行结果、revision 与含业务/lease 双重标识的可信 Actor。
// OUTPUT: 幂等键唯一、绑定执行 lease、按 owner 与资源 scope 隔离的 applying/success/failed 审计记录。
// POS: configuration 写入前置门闩与事后追溯仓储。
package configuration

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	auditFinishTimeout = 5 * time.Second
	staleAuditAfter    = 5 * time.Minute
)

func (s *Service) beginAudit(
	ctx context.Context,
	actor *resolvedActor,
	request ChangeRequest,
	plan ChangePlan,
	humanApproval *humanApprovalRecord,
) (*AuditRecord, bool, error) {
	existing, err := s.auditByID(ctx, actor.OwnerUserID, request.RequestID)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		return existing, false, nil
	}
	query := fmt.Sprintf(
		`INSERT INTO configuration_changes (
			request_id, owner_user_id, actor_agent_id, session_key, round_id,
			lease_session_key, lease_round_id,
			context_kind, context_id, scope_kind, scope_id, authority, intent_digest,
			human_approval_request_id, human_principal_user_id, human_principal_role,
			human_auth_method, human_approved_at,
			domain, operation, target,
			request_json, result_json, revision_before, revision_after, status, error_message
		) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)`,
		s.dialect.Bind(1), s.dialect.Bind(2), s.dialect.Bind(3), s.dialect.Bind(4),
		s.dialect.Bind(5), s.dialect.Bind(6), s.dialect.Bind(7), s.dialect.Bind(8),
		s.dialect.Bind(9), s.dialect.Bind(10), s.dialect.Bind(11), s.dialect.Bind(12),
		s.dialect.Bind(13), s.dialect.Bind(14), s.dialect.Bind(15), s.dialect.Bind(16),
		s.dialect.Bind(17), s.dialect.Bind(18), s.dialect.Bind(19), s.dialect.Bind(20),
		s.dialect.Bind(21), s.dialect.Bind(22), s.dialect.Bind(23), s.dialect.Bind(24),
		s.dialect.Bind(25), s.dialect.Bind(26), s.dialect.Bind(27),
	)
	humanApprovalRequestID := ""
	humanPrincipalUserID := ""
	humanPrincipalRole := ""
	humanAuthMethod := ""
	var humanApprovedAt any
	if humanApproval != nil {
		humanApprovalRequestID = humanApproval.PermissionRequestID
		humanPrincipalUserID = humanApproval.OwnerUserID
		humanPrincipalRole = humanApproval.PrincipalRole
		humanAuthMethod = humanApproval.PrincipalAuthMethod
		humanApprovedAt = humanApproval.ApprovedAt
	}
	requestPayload := sanitizedJSON(request)
	if _, err = s.db.ExecContext(
		ctx, query,
		request.RequestID, actor.OwnerUserID, actor.AgentID, actor.SessionKey, actor.RoundID,
		actor.LeaseSessionKey, actor.LeaseRoundID,
		actor.Context.Kind, actor.Context.ID, plan.Scope.Kind, plan.Scope.ID,
		actor.Authority, plan.PlanDigest,
		humanApprovalRequestID, humanPrincipalUserID, humanPrincipalRole,
		humanAuthMethod, humanApprovedAt,
		request.Domain, request.Operation, request.Target, string(requestPayload), "{}",
		plan.CurrentRevision, "", "applying", "",
	); err != nil {
		existing, lookupErr := s.auditByID(ctx, actor.OwnerUserID, request.RequestID)
		if lookupErr == nil && existing != nil {
			return existing, false, nil
		}
		return nil, false, err
	}
	record, err := s.auditByID(ctx, actor.OwnerUserID, request.RequestID)
	return record, true, err
}

func (s *Service) finishAudit(
	ctx context.Context,
	actor Actor,
	requestID string,
	status string,
	result any,
	revisionAfter string,
	executionErr error,
) error {
	// 领域写入一旦开始，审计收尾不能继承请求断连或 deadline。否则客户端
	// 取消会把记录永久留在 applying，后续同 request_id 无法判断真实结果。
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), auditFinishTimeout)
	defer cancel()
	errorMessage := ""
	if executionErr != nil {
		errorMessage = executionErr.Error()
	}
	query := fmt.Sprintf(
		`UPDATE configuration_changes
		 SET result_json = %s, revision_after = %s, status = %s, error_message = %s, updated_at = %s
		 WHERE request_id = %s AND owner_user_id = %s`,
		s.dialect.Bind(1), s.dialect.Bind(2), s.dialect.Bind(3), s.dialect.Bind(4),
		s.dialect.CurrentTimestamp(), s.dialect.Bind(5), s.dialect.Bind(6),
	)
	_, err := s.db.ExecContext(
		finishCtx, query, string(sanitizedJSON(result)), revisionAfter, status, errorMessage,
		requestID, actor.OwnerUserID,
	)
	return err
}

func (s *Service) replayOrRecover(
	ctx context.Context,
	actor Actor,
	record *AuditRecord,
) (*ApplyResult, error) {
	if record == nil || record.Status != "applying" ||
		time.Since(record.UpdatedAt) < staleAuditAfter {
		return replayResult(record)
	}
	recoveryErr := errors.New("配置执行租约已过期，实际写入结果未知；必须重新 inspect 并使用新的 request_id reconcile")
	if err := s.finishAudit(
		ctx,
		actor,
		record.RequestID,
		"reconcile_required",
		map[string]any{
			"applied": "unknown",
			"error":   recoveryErr.Error(),
		},
		record.RevisionAfter,
		recoveryErr,
	); err != nil {
		return nil, fmt.Errorf("回收过期配置审计失败: %w", err)
	}
	record.Status = "reconcile_required"
	record.ErrorMessage = recoveryErr.Error()
	return replayResult(record)
}

func (s *Service) auditByID(ctx context.Context, ownerUserID, requestID string) (*AuditRecord, error) {
	query := fmt.Sprintf(
		`SELECT request_id, owner_user_id, actor_agent_id, session_key, round_id,
		        lease_session_key, lease_round_id,
		        context_kind, context_id, scope_kind, scope_id, authority, intent_digest,
		        human_approval_request_id, human_principal_user_id, human_principal_role,
		        human_auth_method, human_approved_at,
		        domain, operation, target,
		        request_json, result_json, revision_before, revision_after, status, error_message,
		        created_at, updated_at
		 FROM configuration_changes
		 WHERE owner_user_id = %s AND request_id = %s`,
		s.dialect.Bind(1), s.dialect.Bind(2),
	)
	record, err := scanAudit(s.db.QueryRowContext(ctx, query, ownerUserID, requestID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return record, err
}

// ListChanges 返回当前 owner 的配置变更审计。
func (s *Service) ListChanges(ctx context.Context, actor Actor, domain string, limit int) ([]AuditRecord, error) {
	resolved, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	args := []any{resolved.OwnerUserID}
	query := `SELECT request_id, owner_user_id, actor_agent_id, session_key, round_id,
		                 lease_session_key, lease_round_id,
		                 context_kind, context_id, scope_kind, scope_id, authority, intent_digest,
		                 human_approval_request_id, human_principal_user_id, human_principal_role,
		                 human_auth_method, human_approved_at,
		                 domain, operation, target,
		                 request_json, result_json, revision_before, revision_after, status, error_message,
		                 created_at, updated_at
	          FROM configuration_changes
	          WHERE owner_user_id = ` + s.dialect.Bind(1)
	if strings.TrimSpace(domain) != "" {
		definition, _, definitionErr := definitionForActor(resolved, domain)
		if definitionErr != nil {
			return nil, definitionErr
		}
		args = append(args, definition.Name)
		query += " AND domain = " + s.dialect.Bind(len(args))
	} else if !resolved.canManageHostConfiguration() {
		// An owner-main Agent attached to a member principal still manages that
		// user's private resources, but must not infer deployment or native host
		// state from an unfiltered audit listing.
		args = append(args, DomainHost)
		query += " AND domain <> " + s.dialect.Bind(len(args))
	}
	switch resolved.Authority {
	case AuthorityAgentSelf:
		args = append(args, ScopeKindAgent, resolved.AgentID)
		query += " AND scope_kind = " + s.dialect.Bind(len(args)-1) +
			" AND scope_id = " + s.dialect.Bind(len(args))
	case AuthorityRoomHost, AuthorityRoomMember:
		args = append(args, ScopeKindRoom, resolved.RoomID)
		query += " AND scope_kind = " + s.dialect.Bind(len(args)-1) +
			" AND scope_id = " + s.dialect.Bind(len(args))
	case AuthorityOwnerMain:
	default:
		return nil, fmt.Errorf("%s 无权读取配置历史", resolved.Authority)
	}
	args = append(args, limit)
	query += " ORDER BY created_at DESC LIMIT " + s.dialect.Bind(len(args))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]AuditRecord, 0, limit)
	for rows.Next() {
		record, scanErr := scanAudit(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, *record)
	}
	return result, rows.Err()
}

type auditScanner interface {
	Scan(...any) error
}

func scanAudit(scanner auditScanner) (*AuditRecord, error) {
	var record AuditRecord
	var requestJSON string
	var resultJSON string
	var humanApprovedAt sql.NullTime
	if err := scanner.Scan(
		&record.RequestID, &record.OwnerUserID, &record.ActorAgentID, &record.SessionKey,
		&record.RoundID, &record.LeaseSessionKey, &record.LeaseRoundID,
		&record.ContextKind, &record.ContextID, &record.ScopeKind, &record.ScopeID,
		&record.Authority, &record.IntentDigest,
		&record.HumanApprovalRequestID, &record.HumanPrincipalUserID,
		&record.HumanPrincipalRole, &record.HumanAuthMethod, &humanApprovedAt,
		&record.Domain, &record.Operation, &record.Target, &requestJSON, &resultJSON,
		&record.RevisionBefore, &record.RevisionAfter, &record.Status, &record.ErrorMessage,
		&record.CreatedAt, &record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if !json.Valid([]byte(requestJSON)) {
		requestJSON = "{}"
	}
	if !json.Valid([]byte(resultJSON)) {
		resultJSON = "{}"
	}
	record.Request = json.RawMessage(requestJSON)
	record.Result = json.RawMessage(resultJSON)
	if humanApprovedAt.Valid {
		approvedAt := humanApprovedAt.Time.UTC()
		record.HumanApprovedAt = &approvedAt
	}
	record.CreatedAt = record.CreatedAt.UTC()
	record.UpdatedAt = record.UpdatedAt.UTC()
	return &record, nil
}

func replayResult(record *AuditRecord) (*ApplyResult, error) {
	if record == nil {
		return nil, errors.New("配置审计记录不存在")
	}
	switch record.Status {
	case "success":
		var result ApplyResult
		if err := json.Unmarshal(record.Result, &result); err != nil {
			return nil, fmt.Errorf("读取幂等变更结果: %w", err)
		}
		result.IdempotentReplay = true
		return &result, nil
	case "applying":
		return nil, fmt.Errorf("request_id=%s 的配置变更仍在执行，请查询审计后再重试", record.RequestID)
	case "reconcile_required":
		return nil, fmt.Errorf(
			"request_id=%s 的配置结果需要 reconcile，不能盲目重放；请重新 inspect/plan 并使用新的 request_id",
			record.RequestID,
		)
	default:
		return nil, fmt.Errorf("request_id=%s 已执行失败，不能复用；请修正后使用新的 request_id", record.RequestID)
	}
}
