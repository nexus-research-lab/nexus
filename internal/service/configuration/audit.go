// INPUT: 已脱敏的变更请求、执行结果、revision 与可信 Actor。
// OUTPUT: 幂等键唯一、按 owner 隔离的 applying/success/failed 审计记录。
// POS: configuration 写入前置门闩与事后追溯仓储。
package configuration

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func (s *Service) beginAudit(ctx context.Context, actor Actor, request ChangeRequest, plan ChangePlan) (*AuditRecord, bool, error) {
	existing, err := s.auditByID(ctx, actor.OwnerUserID, request.RequestID)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		return existing, false, nil
	}
	query := fmt.Sprintf(
		`INSERT INTO configuration_changes (
			request_id, owner_user_id, actor_agent_id, session_key, domain, operation, target,
			request_json, result_json, revision_before, revision_after, status, error_message
		) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)`,
		s.dialect.Bind(1), s.dialect.Bind(2), s.dialect.Bind(3), s.dialect.Bind(4),
		s.dialect.Bind(5), s.dialect.Bind(6), s.dialect.Bind(7), s.dialect.Bind(8),
		s.dialect.Bind(9), s.dialect.Bind(10), s.dialect.Bind(11), s.dialect.Bind(12),
		s.dialect.Bind(13),
	)
	requestPayload := sanitizedJSON(request)
	if _, err = s.db.ExecContext(
		ctx, query,
		request.RequestID, actor.OwnerUserID, actor.AgentID, actor.SessionKey,
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
		ctx, query, string(sanitizedJSON(result)), revisionAfter, status, errorMessage,
		requestID, actor.OwnerUserID,
	)
	return err
}

func (s *Service) auditByID(ctx context.Context, ownerUserID, requestID string) (*AuditRecord, error) {
	query := fmt.Sprintf(
		`SELECT request_id, owner_user_id, actor_agent_id, session_key, domain, operation, target,
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
	if err := requireMainActor(actor); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	args := []any{actor.OwnerUserID}
	query := `SELECT request_id, owner_user_id, actor_agent_id, session_key, domain, operation, target,
	                 request_json, result_json, revision_before, revision_after, status, error_message,
	                 created_at, updated_at
	          FROM configuration_changes
	          WHERE owner_user_id = ` + s.dialect.Bind(1)
	if strings.TrimSpace(domain) != "" {
		definition, err := definitionFor(domain)
		if err != nil {
			return nil, err
		}
		args = append(args, definition.Name)
		query += " AND domain = " + s.dialect.Bind(len(args))
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
	if err := scanner.Scan(
		&record.RequestID, &record.OwnerUserID, &record.ActorAgentID, &record.SessionKey,
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
	record.CreatedAt = record.CreatedAt.UTC()
	record.UpdatedAt = record.UpdatedAt.UTC()
	return &record, nil
}

func replayResult(record *AuditRecord) (*ApplyResult, error) {
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
	default:
		return nil, fmt.Errorf("request_id=%s 已执行失败，不能复用；请修正后使用新的 request_id", record.RequestID)
	}
}
