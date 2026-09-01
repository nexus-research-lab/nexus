// INPUT: Connector 对话授权流程记录、CAS 状态和 Connector credentials key。
// OUTPUT: 跨 SQLite/Postgres 的流程持久化、领取、不改 canonical 配置的阶段推进、终态及秘密加解密。
// POS: OAuth/Device 对话授权的 SQL 存储边界；业务权限与 provider 调用留在 service。
package connectors

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
)

// AuthorizationFlowStore 持久化 owner/session/Connector 绑定的授权流程。
type AuthorizationFlowStore struct {
	db     *sql.DB
	driver string
	key    []byte
}

// NewAuthorizationFlowStore 创建授权流程仓储。
func NewAuthorizationFlowStore(
	db *sql.DB,
	driver string,
	key []byte,
) *AuthorizationFlowStore {
	return &AuthorizationFlowStore{db: db, driver: driver, key: key}
}

// EncryptSecret 加密 provider state URL、device_code 或临时应用凭据。
func (s *AuthorizationFlowStore) EncryptSecret(plain []byte) (string, error) {
	if len(s.key) == 0 {
		return "", errors.New("CONNECTOR_CREDENTIALS_KEY 未配置，无法保存 Connector 授权流程")
	}
	return credentials.EncryptPayload(s.key, plain)
}

// DecryptSecret 解密仅供 provider 调用使用的流程秘密。
func (s *AuthorizationFlowStore) DecryptSecret(encrypted string) ([]byte, error) {
	if len(s.key) == 0 {
		return nil, errors.New("CONNECTOR_CREDENTIALS_KEY 未配置，无法恢复 Connector 授权流程")
	}
	if strings.TrimSpace(encrypted) == "" {
		return nil, errors.New("Connector 授权流程缺少加密临时凭据")
	}
	return credentials.DecryptPayload(s.key, encrypted)
}

// CreateApproved 创建 durable 人工批准记录。相同 owner/request_id 只返回已有记录，
// 是否为同一意图由 service 比较完整绑定后决定。
func (s *AuthorizationFlowStore) CreateApproved(
	ctx context.Context,
	record AuthorizationFlow,
) (*AuthorizationFlow, error) {
	query := `
INSERT INTO connector_authorization_flows (
    flow_id, owner_user_id, human_principal_user_id, human_principal_role,
    human_auth_method, human_auth_session_id, permission_request_id, request_id,
    agent_id, business_session_key, root_round_id, runtime_lease_session_key,
    runtime_lease_round_id, connector_id, authorization_method, device_mode,
    intent_digest, start_configuration_version, expected_configuration_version,
    status, human_approved_at, expires_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(owner_user_id, request_id) DO NOTHING`
	if s.driver == "pgx" {
		query = `
INSERT INTO connector_authorization_flows (
    flow_id, owner_user_id, human_principal_user_id, human_principal_role,
    human_auth_method, human_auth_session_id, permission_request_id, request_id,
    agent_id, business_session_key, root_round_id, runtime_lease_session_key,
    runtime_lease_round_id, connector_id, authorization_method, device_mode,
    intent_digest, start_configuration_version, expected_configuration_version,
    status, human_approved_at, expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
    $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
)
ON CONFLICT(owner_user_id, request_id) DO NOTHING`
	}
	_, err := s.db.ExecContext(
		ctx, query,
		record.FlowID, record.OwnerUserID, record.HumanPrincipalUserID,
		record.HumanPrincipalRole, record.HumanAuthMethod,
		nullStringValue(record.HumanAuthSessionID),
		record.PermissionRequestID, record.RequestID, record.AgentID,
		record.BusinessSessionKey, record.RootRoundID,
		record.RuntimeLeaseSessionKey, record.RuntimeLeaseRoundID,
		record.ConnectorID, record.AuthorizationMethod, record.DeviceMode,
		record.IntentDigest, record.StartConfigurationVersion,
		record.ExpectedConfigurationVersion, record.Status,
		record.HumanApprovedAt, record.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	return s.GetByRequest(ctx, record.OwnerUserID, record.RequestID)
}

// GetByRequest 返回 owner/request_id 唯一流程。
func (s *AuthorizationFlowStore) GetByRequest(
	ctx context.Context,
	ownerUserID string,
	requestID string,
) (*AuthorizationFlow, error) {
	query := authorizationFlowSelect() +
		fmt.Sprintf(
			" WHERE owner_user_id = %s AND request_id = %s",
			s.bind(1), s.bind(2),
		)
	return scanAuthorizationFlow(s.db.QueryRowContext(
		ctx, query, strings.TrimSpace(ownerUserID), strings.TrimSpace(requestID),
	))
}

// Get 返回 owner/flow_id 唯一流程。
func (s *AuthorizationFlowStore) Get(
	ctx context.Context,
	ownerUserID string,
	flowID string,
) (*AuthorizationFlow, error) {
	query := authorizationFlowSelect() +
		fmt.Sprintf(
			" WHERE owner_user_id = %s AND flow_id = %s",
			s.bind(1), s.bind(2),
		)
	return scanAuthorizationFlow(s.db.QueryRowContext(
		ctx, query, strings.TrimSpace(ownerUserID), strings.TrimSpace(flowID),
	))
}

// GetByID 返回 callback 使用的 flow_id 记录；调用方必须再核对 state owner/Connector。
func (s *AuthorizationFlowStore) GetByID(
	ctx context.Context,
	flowID string,
) (*AuthorizationFlow, error) {
	query := authorizationFlowSelect() +
		fmt.Sprintf(" WHERE flow_id = %s", s.bind(1))
	return scanAuthorizationFlow(s.db.QueryRowContext(
		ctx, query, strings.TrimSpace(flowID),
	))
}

// ClaimStart 在调用 provider 前取得跨进程短租约，避免并发 start 创建两个
// OAuth state/device session。崩溃后的空 stage claim 到期可由同一批准重试。
func (s *AuthorizationFlowStore) ClaimStart(
	ctx context.Context,
	ownerUserID string,
	flowID string,
	now time.Time,
	claimUntil time.Time,
) (bool, error) {
	query := fmt.Sprintf(`
UPDATE connector_authorization_flows
SET status = 'polling', poll_claim_until = %s, updated_at = %s
WHERE owner_user_id = %s AND flow_id = %s AND stage = ''
  AND expires_at > %s
  AND (
      status = 'approved'
      OR (status = 'polling' AND poll_claim_until IS NOT NULL AND poll_claim_until <= %s)
  )`,
		s.bind(1), s.bind(2), s.bind(3),
		s.bind(4), s.bind(5), s.bind(6),
	)
	result, err := s.db.ExecContext(
		ctx, query, claimUntil, now,
		strings.TrimSpace(ownerUserID), strings.TrimSpace(flowID),
		now, now,
	)
	return oneRowAffected(result, err)
}

// Activate 把已经领取的 durable approval 原子推进为 provider pending。
func (s *AuthorizationFlowStore) Activate(
	ctx context.Context,
	ownerUserID string,
	flowID string,
	activation AuthorizationFlowActivation,
	now time.Time,
) (bool, error) {
	query := fmt.Sprintf(`
UPDATE connector_authorization_flows SET
    expected_configuration_version = %s,
    status = 'pending',
    stage = %s,
    secret_encrypted = %s,
    public_user_code = %s,
    public_verification_uri = %s,
    public_verification_uri_complete = %s,
    public_open_path = %s,
    poll_interval_seconds = %s,
    next_poll_at = %s,
    expires_at = %s,
    poll_claim_until = NULL,
    updated_at = %s
WHERE owner_user_id = %s AND flow_id = %s
  AND status = 'polling' AND stage = '' AND expires_at > %s`,
		s.bind(1), s.bind(2), s.bind(3), s.bind(4), s.bind(5),
		s.bind(6), s.bind(7), s.bind(8), s.bind(9), s.bind(10),
		s.bind(11), s.bind(12), s.bind(13), s.bind(14),
	)
	result, err := s.db.ExecContext(
		ctx, query,
		activation.ExpectedConfigurationVersion,
		activation.Stage,
		activation.SecretEncrypted,
		activation.PublicUserCode,
		activation.PublicVerificationURI,
		activation.PublicVerificationURIComplete,
		activation.PublicOpenPath,
		activation.PollIntervalSeconds,
		nullTimeValue(activation.NextPollAt),
		activation.ExpiresAt,
		now,
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(flowID),
		now,
	)
	return oneRowAffected(result, err)
}

// MarkOpened 记录浏览器 URL 已由同一人工 principal 打开。
func (s *AuthorizationFlowStore) MarkOpened(
	ctx context.Context,
	ownerUserID string,
	flowID string,
	now time.Time,
) (bool, error) {
	query := fmt.Sprintf(`
UPDATE connector_authorization_flows
SET opened_at = COALESCE(opened_at, %s), updated_at = %s
WHERE owner_user_id = %s AND flow_id = %s
  AND authorization_method = 'oauth_browser'
  AND status = 'pending' AND expires_at > %s`,
		s.bind(1), s.bind(2), s.bind(3), s.bind(4), s.bind(5),
	)
	result, err := s.db.ExecContext(
		ctx, query, now, now,
		strings.TrimSpace(ownerUserID), strings.TrimSpace(flowID), now,
	)
	return oneRowAffected(result, err)
}

// ClaimDevicePoll 防止多个 runtime/process 同时交换同一个 device_code。
func (s *AuthorizationFlowStore) ClaimDevicePoll(
	ctx context.Context,
	ownerUserID string,
	flowID string,
	now time.Time,
	claimUntil time.Time,
) (bool, error) {
	query := fmt.Sprintf(`
UPDATE connector_authorization_flows
SET status = 'polling', poll_claim_until = %s, updated_at = %s
WHERE owner_user_id = %s AND flow_id = %s
  AND authorization_method = 'device'
  AND expires_at > %s
  AND (next_poll_at IS NULL OR next_poll_at <= %s)
  AND (
      status = 'pending'
      OR (status = 'polling' AND poll_claim_until IS NOT NULL AND poll_claim_until <= %s)
  )`,
		s.bind(1), s.bind(2), s.bind(3), s.bind(4),
		s.bind(5), s.bind(6), s.bind(7),
	)
	result, err := s.db.ExecContext(
		ctx, query, claimUntil, now,
		strings.TrimSpace(ownerUserID), strings.TrimSpace(flowID),
		now, now, now,
	)
	return oneRowAffected(result, err)
}

// ReleaseDevicePoll 把 pending/slow_down 结果安全释放，并设置下次轮询时间。
func (s *AuthorizationFlowStore) ReleaseDevicePoll(
	ctx context.Context,
	ownerUserID string,
	flowID string,
	nextPollAt time.Time,
	message string,
	now time.Time,
) (bool, error) {
	query := fmt.Sprintf(`
UPDATE connector_authorization_flows SET
    status = 'pending', result_message = %s, next_poll_at = %s,
    poll_claim_until = NULL, updated_at = %s
WHERE owner_user_id = %s AND flow_id = %s AND status = 'polling'`,
		s.bind(1), s.bind(2), s.bind(3), s.bind(4), s.bind(5),
	)
	result, err := s.db.ExecContext(
		ctx, query, strings.TrimSpace(message), nextPollAt, now,
		strings.TrimSpace(ownerUserID), strings.TrimSpace(flowID),
	)
	return oneRowAffected(result, err)
}

// AdvanceDeviceStage 在不改动 canonical Connector 配置的前提下，
// 原子替换为下一阶段的加密 device_code 与本次 OAuth client。
func (s *AuthorizationFlowStore) AdvanceDeviceStage(
	ctx context.Context,
	ownerUserID string,
	flowID string,
	stage AuthorizationFlowDeviceStage,
	now time.Time,
) error {
	query := fmt.Sprintf(`
UPDATE connector_authorization_flows SET
    expected_configuration_version = %s,
    status = 'pending',
    stage = %s,
    secret_encrypted = %s,
    public_user_code = %s,
    public_verification_uri = %s,
    public_verification_uri_complete = %s,
    poll_interval_seconds = %s,
    result_message = '',
    next_poll_at = %s,
    poll_claim_until = NULL,
    expires_at = %s,
    updated_at = %s
WHERE owner_user_id = %s AND flow_id = %s AND status = 'polling'`,
		s.bind(1), s.bind(2), s.bind(3), s.bind(4), s.bind(5),
		s.bind(6), s.bind(7), s.bind(8), s.bind(9), s.bind(10),
		s.bind(11), s.bind(12),
	)
	result, err := s.db.ExecContext(
		ctx, query,
		stage.ExpectedConfigurationVersion,
		stage.Stage,
		stage.SecretEncrypted,
		stage.PublicUserCode,
		stage.PublicVerificationURI,
		stage.PublicVerificationURIComplete,
		stage.PollIntervalSeconds,
		stage.NextPollAt,
		stage.ExpiresAt,
		now,
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(flowID),
	)
	affected, err := rowsAffected(result, err)
	if err != nil {
		return err
	}
	if affected != 1 {
		return errors.New("Connector authorization flow stage 已变化")
	}
	return nil
}

// MarkConnectedTx 与 Connector connection/version 在同一事务提交，并擦除
// provider 秘密及只在授权期间需要展示的人类授权材料。
func (s *AuthorizationFlowStore) MarkConnectedTx(
	ctx context.Context,
	tx *sql.Tx,
	ownerUserID string,
	flowID string,
	completedVersion int64,
	now time.Time,
) error {
	if tx == nil {
		return errors.New("Connector authorization transaction 不能为空")
	}
	query := fmt.Sprintf(`
UPDATE connector_authorization_flows SET
    status = 'connected',
    completed_configuration_version = %s,
    secret_encrypted = '',
    public_user_code = '',
    public_verification_uri = '',
    public_verification_uri_complete = '',
    public_open_path = '',
    result_message = '',
    next_poll_at = NULL,
    poll_claim_until = NULL,
    completed_at = %s,
    updated_at = %s
WHERE owner_user_id = %s AND flow_id = %s
  AND status IN ('pending', 'polling')`,
		s.bind(1), s.bind(2), s.bind(3), s.bind(4), s.bind(5),
	)
	result, err := tx.ExecContext(
		ctx, query, completedVersion, now, now,
		strings.TrimSpace(ownerUserID), strings.TrimSpace(flowID),
	)
	affected, err := rowsAffected(result, err)
	if err != nil {
		return err
	}
	if affected != 1 {
		return errors.New("Connector authorization flow completion 已变化")
	}
	return nil
}

// MarkTerminal 记录过期、拒绝、冲突或失败，并擦除流程秘密及临时公开授权材料。
func (s *AuthorizationFlowStore) MarkTerminal(
	ctx context.Context,
	ownerUserID string,
	flowID string,
	status string,
	message string,
	now time.Time,
) (bool, error) {
	switch status {
	case "expired", "denied", "conflict", "failed":
	default:
		return false, errors.New("Connector authorization terminal status 无效")
	}
	query := fmt.Sprintf(`
UPDATE connector_authorization_flows SET
    status = %s, secret_encrypted = '', result_message = %s,
    public_user_code = '', public_verification_uri = '',
    public_verification_uri_complete = '', public_open_path = '',
    next_poll_at = NULL, poll_claim_until = NULL,
    completed_at = %s, updated_at = %s
WHERE owner_user_id = %s AND flow_id = %s
  AND status IN ('approved', 'pending', 'polling')`,
		s.bind(1), s.bind(2), s.bind(3), s.bind(4), s.bind(5), s.bind(6),
	)
	result, err := s.db.ExecContext(
		ctx, query, status, strings.TrimSpace(message), now, now,
		strings.TrimSpace(ownerUserID), strings.TrimSpace(flowID),
	)
	return oneRowAffected(result, err)
}

// Cancel 由原 owner/main DM 主动撤销，且立即擦除流程秘密及临时公开授权材料。
func (s *AuthorizationFlowStore) Cancel(
	ctx context.Context,
	ownerUserID string,
	flowID string,
	now time.Time,
) (bool, error) {
	query := fmt.Sprintf(`
UPDATE connector_authorization_flows SET
    status = 'canceled', secret_encrypted = '', result_message = '',
    public_user_code = '', public_verification_uri = '',
    public_verification_uri_complete = '', public_open_path = '',
    next_poll_at = NULL, poll_claim_until = NULL,
    canceled_at = %s, completed_at = %s, updated_at = %s
WHERE owner_user_id = %s AND flow_id = %s
  AND status IN ('approved', 'pending', 'polling')`,
		s.bind(1), s.bind(2), s.bind(3), s.bind(4), s.bind(5),
	)
	result, err := s.db.ExecContext(
		ctx, query, now, now, now,
		strings.TrimSpace(ownerUserID), strings.TrimSpace(flowID),
	)
	return oneRowAffected(result, err)
}

func authorizationFlowSelect() string {
	return `
SELECT
    flow_id, owner_user_id, human_principal_user_id, human_principal_role,
    human_auth_method, human_auth_session_id, permission_request_id, request_id,
    agent_id, business_session_key, root_round_id, runtime_lease_session_key,
    runtime_lease_round_id, connector_id, authorization_method, device_mode,
    intent_digest, start_configuration_version, expected_configuration_version,
    completed_configuration_version, status, stage, secret_encrypted,
    public_user_code, public_verification_uri, public_verification_uri_complete,
    public_open_path, poll_interval_seconds, result_message, human_approved_at,
    opened_at, next_poll_at, poll_claim_until, expires_at, completed_at,
    canceled_at, created_at, updated_at
FROM connector_authorization_flows`
}

type authorizationFlowScanner interface {
	Scan(...any) error
}

func scanAuthorizationFlow(scanner authorizationFlowScanner) (*AuthorizationFlow, error) {
	var record AuthorizationFlow
	err := scanner.Scan(
		&record.FlowID, &record.OwnerUserID,
		&record.HumanPrincipalUserID, &record.HumanPrincipalRole,
		&record.HumanAuthMethod, &record.HumanAuthSessionID,
		&record.PermissionRequestID, &record.RequestID,
		&record.AgentID, &record.BusinessSessionKey, &record.RootRoundID,
		&record.RuntimeLeaseSessionKey, &record.RuntimeLeaseRoundID,
		&record.ConnectorID, &record.AuthorizationMethod, &record.DeviceMode,
		&record.IntentDigest, &record.StartConfigurationVersion,
		&record.ExpectedConfigurationVersion,
		&record.CompletedConfigurationVersion, &record.Status, &record.Stage,
		&record.SecretEncrypted, &record.PublicUserCode,
		&record.PublicVerificationURI, &record.PublicVerificationURIComplete,
		&record.PublicOpenPath, &record.PollIntervalSeconds,
		&record.ResultMessage, &record.HumanApprovedAt, &record.OpenedAt,
		&record.NextPollAt, &record.PollClaimUntil, &record.ExpiresAt,
		&record.CompletedAt, &record.CanceledAt,
		&record.CreatedAt, &record.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *AuthorizationFlowStore) bind(index int) string {
	if s.driver == "pgx" {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

func nullStringValue(value sql.NullString) any {
	if value.Valid && strings.TrimSpace(value.String) != "" {
		return value.String
	}
	return nil
}

func nullTimeValue(value sql.NullTime) any {
	if value.Valid {
		return value.Time
	}
	return nil
}

func oneRowAffected(result sql.Result, err error) (bool, error) {
	affected, err := rowsAffected(result, err)
	return affected == 1, err
}

func rowsAffected(result sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, errors.New("Connector authorization SQL result 为空")
	}
	return result.RowsAffected()
}
