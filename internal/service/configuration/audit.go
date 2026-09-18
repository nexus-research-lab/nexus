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

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

const (
	auditFinishTimeout = 5 * time.Second
	staleAuditAfter    = 5 * time.Minute
)

// RecoverStaleApplyingChanges closes durable applying receipts whose executor
// lease has expired. It only records that the outcome requires reconciliation;
// it never guesses whether the underlying settings write committed.
//
// This is intentionally a recovery primitive. Callers still need to inspect
// the current settings state before presenting a resolved outcome.
func (s *Service) RecoverStaleApplyingChanges(
	ctx context.Context,
	ownerUserID string,
	limit int,
) ([]AuditRecord, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return nil, errors.New("owner_user_id 不能为空")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	cutoff := time.Now().UTC().Add(-staleAuditAfter)
	query := fmt.Sprintf(
		`SELECT request_id
		 FROM configuration_changes
		 WHERE owner_user_id = %s AND status = 'applying' AND updated_at <= %s
		 ORDER BY updated_at ASC
		 LIMIT %s`,
		s.dialect.Bind(1), s.dialect.Bind(2), s.dialect.Bind(3),
	)
	rows, err := s.db.QueryContext(ctx, query, ownerUserID, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	requestIDs := make([]string, 0, limit)
	for rows.Next() {
		var requestID string
		if err := rows.Scan(&requestID); err != nil {
			return nil, err
		}
		requestIDs = append(requestIDs, requestID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	recovered := make([]AuditRecord, 0, len(requestIDs))
	recoveryErr := errors.New("配置执行租约已过期，实际写入结果未知；必须重新 inspect 并使用新的 request_id reconcile")
	for _, requestID := range requestIDs {
		resultJSON := string(sanitizedJSON(map[string]any{
			"applied": "unknown",
			"error":   recoveryErr.Error(),
		}))
		updateQuery := fmt.Sprintf(
			`UPDATE configuration_changes
			 SET result_json = %s, status = 'reconcile_required', error_message = %s,
			     updated_at = %s
			 WHERE owner_user_id = %s AND request_id = %s
			   AND status = 'applying' AND updated_at <= %s`,
			s.dialect.Bind(1), s.dialect.Bind(2), s.dialect.CurrentTimestamp(),
			s.dialect.Bind(3), s.dialect.Bind(4), s.dialect.Bind(5),
		)
		updated, err := s.db.ExecContext(
			ctx, updateQuery, resultJSON, recoveryErr.Error(),
			ownerUserID, requestID, cutoff,
		)
		if err != nil {
			return nil, err
		}
		if count, err := updated.RowsAffected(); err != nil {
			return nil, err
		} else if count == 0 {
			continue
		}
		record, err := s.auditByID(ctx, ownerUserID, requestID)
		if err != nil {
			return nil, err
		}
		if record != nil {
			recovered = append(recovered, *record)
		}
	}
	return recovered, nil
}

// RecoverStaleApplyingChangesForAllOwners is the process recovery entrypoint
// used by the host lifecycle. It discovers only owners with stale applying
// receipts and then delegates each owner to the scoped recovery primitive.
// The limit is global to this invocation; a later scheduler tick can continue
// with any remaining owners. Unknown outcomes remain reconcile_required and
// are never replayed by this sweep.
func (s *Service) RecoverStaleApplyingChangesForAllOwners(
	ctx context.Context,
	limit int,
) ([]AuditRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	cutoff := time.Now().UTC().Add(-staleAuditAfter)
	query := fmt.Sprintf(
		`SELECT DISTINCT owner_user_id
		 FROM configuration_changes
		 WHERE status = 'applying' AND updated_at <= %s
		 ORDER BY owner_user_id ASC`,
		s.dialect.Bind(1),
	)
	rows, err := s.db.QueryContext(ctx, query, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	owners := make([]string, 0)
	for rows.Next() {
		var ownerUserID string
		if err := rows.Scan(&ownerUserID); err != nil {
			return nil, err
		}
		ownerUserID = strings.TrimSpace(ownerUserID)
		if ownerUserID != "" {
			owners = append(owners, ownerUserID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	recovered := make([]AuditRecord, 0, limit)
	for _, ownerUserID := range owners {
		remaining := limit - len(recovered)
		if remaining <= 0 {
			break
		}
		items, err := s.RecoverStaleApplyingChanges(ctx, ownerUserID, remaining)
		if err != nil {
			return nil, err
		}
		recovered = append(recovered, items...)
	}
	return recovered, nil
}

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

// ReviewChange reads one durable receipt together with the current scoped
// configuration. It is deliberately read-only: a receipt in
// reconcile_required remains unknown until a human explicitly confirms a
// decision through ReconcileChange.
func (s *Service) ReviewChange(
	ctx context.Context,
	actor Actor,
	requestID string,
) (*ChangeReconciliation, error) {
	resolved, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	requestID = strings.TrimSpace(requestID)
	if !requestIDPattern.MatchString(requestID) {
		return nil, errors.New("request_id 格式无效")
	}
	record, err := s.auditByID(ctx, resolved.OwnerUserID, requestID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, fmt.Errorf("配置审计记录不存在: request_id=%s", requestID)
	}
	if err := authorizeAuditReview(resolved, record); err != nil {
		return nil, err
	}
	current, err := s.currentReconciliationSnapshot(
		scopedContext(ctx, resolved.Actor), resolved, record,
	)
	if err != nil {
		return nil, err
	}
	return &ChangeReconciliation{
		Receipt:  *record,
		Current:  current,
		Evidence: reconciliationEvidence(*record, current),
	}, nil
}

// ReconcileChange records an explicit human decision for an unknown receipt.
// It never calls executeChange and therefore cannot replay a provider request,
// file write, or any other side effect. The observed revision is checked
// immediately before the durable status transition so a stale settings page
// cannot close a newer state as if it were reviewed.
func (s *Service) ReconcileChange(
	ctx context.Context,
	actor Actor,
	request ReconcileRequest,
) (*ChangeReconciliation, error) {
	resolved, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	if resolved.Authority != AuthorityOwnerMain || resolved.RoundLeaseRequired {
		return nil, errors.New("未知配置写入只能由当前 owner 的人工配置入口 reconcile")
	}
	if !resolved.LocalSingleUser &&
		resolved.AuthMethod != "nexuscfg" &&
		!(resolved.AuthMethod == authctx.AuthMethodPassword && resolved.AuthSessionID != "") {
		return nil, errors.New("reconcile 缺少当前真人的本地配置入口或远程登录会话")
	}
	if !request.Confirmed {
		return nil, errors.New("reconcile 必须明确确认；它只记录人工决定，不会重放写入")
	}
	request.RequestID = strings.TrimSpace(request.RequestID)
	request.Decision = strings.ToLower(strings.TrimSpace(request.Decision))
	request.ObservedRevision = strings.TrimSpace(request.ObservedRevision)
	if !requestIDPattern.MatchString(request.RequestID) {
		return nil, errors.New("request_id 格式无效")
	}
	if request.Decision != "applied" && request.Decision != "not_applied" {
		return nil, errors.New("reconcile decision 必须为 applied 或 not_applied")
	}
	if request.ObservedRevision == "" {
		return nil, errors.New("observed_revision 不能为空；请先 review 当前配置")
	}
	if len(request.Note) > 512 {
		return nil, errors.New("reconcile note 不能超过 512 个字符")
	}

	review, err := s.ReviewChange(ctx, resolved.Actor, request.RequestID)
	if err != nil {
		return nil, err
	}
	if review.Receipt.Status != "reconcile_required" {
		return nil, fmt.Errorf(
			"request_id=%s 当前状态为 %s，只有 reconcile_required 可以人工收口",
			request.RequestID, review.Receipt.Status,
		)
	}
	if request.ObservedRevision != review.Current.Revision {
		return nil, fmt.Errorf(
			"当前配置已变化：observed_revision=%s current_revision=%s；请重新 review",
			request.ObservedRevision, review.Current.Revision,
		)
	}

	// Serialize the status transition with other configuration mutations for
	// this receipt's scope. The receipt itself remains owner/request scoped in
	// the conditional update below, so a second process cannot win twice.
	unlock := s.lockMutation(resolved.OwnerUserID + ":reconcile:" + review.Receipt.ScopeKind + ":" + review.Receipt.ScopeID)
	defer unlock()
	// Re-read after taking the same in-process scope lock used by apply. The
	// first review only lets the user choose a receipt; it is not the final
	// concurrency check for the status transition.
	latestReview, err := s.ReviewChange(ctx, resolved.Actor, request.RequestID)
	if err != nil {
		return nil, err
	}
	if latestReview.Receipt.Status != "reconcile_required" {
		return nil, fmt.Errorf(
			"request_id=%s 已被另一项 reconcile 收口为 %s",
			request.RequestID, latestReview.Receipt.Status,
		)
	}
	if request.ObservedRevision != latestReview.Current.Revision {
		return nil, fmt.Errorf(
			"当前配置已变化：observed_revision=%s current_revision=%s；请重新 review",
			request.ObservedRevision, latestReview.Current.Revision,
		)
	}
	review = latestReview
	resultPayload := map[string]any{
		"applied":               request.Decision == "applied",
		"decision":              request.Decision,
		"decision_source":       "human_confirmation",
		"note_present":          strings.TrimSpace(request.Note) != "",
		"current_revision":      review.Current.Revision,
		"revision_relation":     review.Evidence.RevisionRelation,
		"current_state_version": review.Current.StateVersion,
	}
	query := fmt.Sprintf(
		`UPDATE configuration_changes
		 SET result_json = %s, revision_after = %s, status = 'reconciled',
		     error_message = '', updated_at = %s
		 WHERE owner_user_id = %s AND request_id = %s AND status = 'reconcile_required'`,
		s.dialect.Bind(1), s.dialect.Bind(2), s.dialect.CurrentTimestamp(),
		s.dialect.Bind(3), s.dialect.Bind(4),
	)
	updated, err := s.db.ExecContext(
		ctx, query, string(sanitizedJSON(resultPayload)), review.Current.Revision,
		resolved.OwnerUserID, request.RequestID,
	)
	if err != nil {
		return nil, err
	}
	count, err := updated.RowsAffected()
	if err != nil {
		return nil, err
	}
	if count != 1 {
		latest, latestErr := s.auditByID(ctx, resolved.OwnerUserID, request.RequestID)
		if latestErr != nil {
			return nil, latestErr
		}
		if latest == nil {
			return nil, fmt.Errorf("配置审计记录不存在: request_id=%s", request.RequestID)
		}
		return nil, fmt.Errorf("request_id=%s 已被另一项 reconcile 收口为 %s", request.RequestID, latest.Status)
	}

	return s.ReviewChange(ctx, resolved.Actor, request.RequestID)
}

func authorizeAuditReview(actor *resolvedActor, record *AuditRecord) error {
	if actor == nil || record == nil {
		return errors.New("配置审计 review 身份无效")
	}
	if _, _, err := definitionForActor(actor, record.Domain); err != nil {
		return err
	}
	scopeID := strings.TrimSpace(record.ScopeID)
	switch actor.Authority {
	case AuthorityOwnerMain:
		if record.ScopeKind == ScopeKindOwner && scopeID != actor.OwnerUserID {
			return errors.New("配置审计不属于当前 owner")
		}
	case AuthorityAgentSelf:
		if record.ScopeKind != ScopeKindAgent || scopeID != actor.AgentID {
			return errors.New("普通 Agent 不能读取其他 scope 的配置审计")
		}
	case AuthorityRoomHost, AuthorityRoomMember:
		if record.ScopeKind != ScopeKindRoom || scopeID != actor.RoomID {
			return errors.New("Room Agent 不能读取其他 Room 的配置审计")
		}
	default:
		return fmt.Errorf("%s 无权读取配置审计", actor.Authority)
	}
	return nil
}

func (s *Service) currentReconciliationSnapshot(
	ctx context.Context,
	actor *resolvedActor,
	record *AuditRecord,
) (DomainSnapshot, error) {
	target := record.Target
	if isTargetDeletion(ChangeRequest{Domain: record.Domain, Operation: record.Operation}) {
		target = ""
	}
	return s.domainSnapshot(ctx, actor, record.Domain, target, true)
}

func reconciliationEvidence(record AuditRecord, current DomainSnapshot) ReconciliationEvidence {
	relation := "different"
	switch {
	case current.Revision == "":
		relation = "unavailable"
	case record.RevisionBefore != "" && current.Revision == record.RevisionBefore:
		relation = "matches_recorded_before"
	case record.RevisionAfter != "" && current.Revision == record.RevisionAfter:
		relation = "matches_recorded_after"
	}
	evidence := ReconciliationEvidence{
		CurrentRevision:        current.Revision,
		RecordedRevisionBefore: record.RevisionBefore,
		RecordedRevisionAfter:  record.RevisionAfter,
		RevisionRelation:       relation,
		CurrentStateVersion:    current.StateVersion,
		DecisionSource:         "human_confirmation_required",
		Checks:                 current.Checks,
	}
	if record.Status == "reconciled" {
		evidence.DecisionSource = "human_confirmation"
	}
	return evidence
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
