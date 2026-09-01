package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	errPasswordChangeOutcomeUnknown = errors.New("password change outcome unknown")
	// ErrPasswordCredentialChanged 表示当前凭据已被其他 exact request 推进。
	ErrPasswordCredentialChanged = errors.New("password credential changed")
	// ErrPasswordChangeNotApplied 表示该 exact request 已被持久化收口为未执行。
	ErrPasswordChangeNotApplied = errors.New("password change not applied")
)

// PasswordChangeOutcome 是 exact password request 的持久终态。
type PasswordChangeOutcome string

const (
	PasswordChangeOutcomeCommitted  PasswordChangeOutcome = "committed"
	PasswordChangeOutcomeNotApplied PasswordChangeOutcome = "not_applied"
)

func (r *Repository) GetUserWithPasswordByUsername(
	ctx context.Context,
	username string,
) (*UserRecord, *PasswordCredential, error) {
	return r.getUserWithPassword(ctx, "username", username)
}

func (r *Repository) GetUserWithPasswordByID(
	ctx context.Context,
	userID string,
) (*UserRecord, *PasswordCredential, error) {
	return r.getUserWithPassword(ctx, "user_id", userID)
}

func (r *Repository) getUserWithPassword(
	ctx context.Context,
	field string,
	value string,
) (*UserRecord, *PasswordCredential, error) {
	if field != "user_id" && field != "username" {
		return nil, nil, fmt.Errorf("unsupported user field: %s", field)
	}
	row := r.db.QueryRowContext(
		ctx,
		`SELECT
    u.user_id,
    u.username,
    u.display_name,
    u.role,
    u.status,
    u.avatar,
    u.last_login_at,
    u.created_at,
    u.updated_at,
    c.credential_id,
    c.password_hash,
    c.password_algo,
    c.password_updated_at,
    c.created_at,
    c.updated_at
FROM users u
LEFT JOIN auth_password_credentials c ON c.user_id = u.user_id
WHERE u.`+field+` = `+r.bind(1)+`
LIMIT 1`,
		strings.TrimSpace(value),
	)
	var (
		user         UserRecord
		avatar       sql.NullString
		lastLoginAt  sql.NullTime
		credentialID sql.NullString
		passwordHash sql.NullString
		passwordAlgo sql.NullString
		passwordAt   sql.NullTime
		credCreated  sql.NullTime
		credUpdated  sql.NullTime
	)
	if err := row.Scan(
		&user.UserID,
		&user.Username,
		&user.DisplayName,
		&user.Role,
		&user.Status,
		&avatar,
		&lastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&credentialID,
		&passwordHash,
		&passwordAlgo,
		&passwordAt,
		&credCreated,
		&credUpdated,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	user.Avatar = nullStringValue(avatar)
	user.LastLoginAt = nullTimePointer(lastLoginAt)
	if !credentialID.Valid {
		return &user, nil, nil
	}
	credential := &PasswordCredential{
		CredentialID:      strings.TrimSpace(credentialID.String),
		UserID:            user.UserID,
		PasswordHash:      strings.TrimSpace(passwordHash.String),
		PasswordAlgo:      strings.TrimSpace(passwordAlgo.String),
		PasswordUpdatedAt: passwordAt.Time.UTC(),
		CreatedAt:         credCreated.Time.UTC(),
		UpdatedAt:         credUpdated.Time.UTC(),
	}
	return &user, credential, nil
}

func (r *Repository) UpsertPasswordCredential(
	ctx context.Context,
	credential PasswordCredential,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = r.upsertPasswordCredentialTx(ctx, tx, credential); err != nil {
		return err
	}
	return tx.Commit()
}

// CommitPasswordChange 在同一事务内更新密码凭据并保存 exact request 已提交回执。
// 返回 true 表示该 request 早已提交，本次没有再次修改密码。
func (r *Repository) CommitPasswordChange(
	ctx context.Context,
	credential PasswordCredential,
	expectedPasswordHash string,
	requestID string,
) (bool, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return false, errors.New("password change request_id is required")
	}
	outcome, err := r.PasswordChangeOutcome(ctx, credential.UserID, requestID)
	if err != nil {
		return false, errors.Join(errPasswordChangeOutcomeUnknown, err)
	}
	switch outcome {
	case PasswordChangeOutcomeCommitted:
		return true, nil
	case PasswordChangeOutcomeNotApplied:
		return false, ErrPasswordChangeNotApplied
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, errors.Join(errPasswordChangeOutcomeUnknown, err)
	}
	txCommitted := false
	defer func() {
		if !txCommitted {
			_ = tx.Rollback()
		}
	}()

	claimed, err := r.insertPasswordChangeOutcomeTx(
		ctx,
		tx,
		credential.UserID,
		requestID,
		PasswordChangeOutcomeCommitted,
		credential.UpdatedAt,
	)
	if err == nil && !claimed {
		_ = tx.Rollback()
		return r.resolveExistingPasswordChangeOutcome(ctx, credential.UserID, requestID)
	}
	if err == nil {
		err = r.updatePasswordCredentialCASTx(
			ctx,
			tx,
			credential,
			expectedPasswordHash,
		)
	}
	if err == nil {
		err = tx.Commit()
		txCommitted = err == nil
	}
	if err == nil {
		return false, nil
	}
	_ = tx.Rollback()

	// commit/transport 错误后只信 durable outcome；CAS 冲突且本 request
	// 没有终态时，才能证明是其他 request 推进了凭据。
	replay, outcomeErr := r.resolveExistingPasswordChangeOutcome(ctx, credential.UserID, requestID)
	if outcomeErr == nil || errors.Is(outcomeErr, ErrPasswordChangeNotApplied) {
		return replay, outcomeErr
	}
	if errors.Is(err, ErrPasswordCredentialChanged) && !IsPasswordChangeOutcomeUnknown(outcomeErr) {
		return false, err
	}
	return false, errors.Join(errPasswordChangeOutcomeUnknown, err, outcomeErr)
}

// SettlePasswordChangeNotApplied 原子宣告 exact request 不再允许修改密码。
// 若并发提交已经获胜，则返回 committed，调用方必须按已提交处理。
func (r *Repository) SettlePasswordChangeNotApplied(
	ctx context.Context,
	userID string,
	requestID string,
	resolvedAt time.Time,
) (PasswordChangeOutcome, error) {
	userID = strings.TrimSpace(userID)
	requestID = strings.TrimSpace(requestID)
	result, err := r.db.ExecContext(
		ctx,
		`INSERT INTO auth_password_change_receipts (
    user_id, request_id, effect, resolved_at, created_at
) VALUES (`+r.bind(1)+`, `+r.bind(2)+`, `+r.bind(3)+`, `+r.bind(4)+`, `+r.bind(5)+`)
ON CONFLICT(user_id, request_id) DO NOTHING`,
		userID,
		requestID,
		PasswordChangeOutcomeNotApplied,
		resolvedAt,
		resolvedAt,
	)
	if err == nil {
		if rowsAffected, rowsErr := result.RowsAffected(); rowsErr == nil && rowsAffected == 1 {
			return PasswordChangeOutcomeNotApplied, nil
		}
	}
	outcome, outcomeErr := r.PasswordChangeOutcome(ctx, userID, requestID)
	if outcomeErr == nil && outcome != "" {
		return outcome, nil
	}
	if err == nil {
		err = errors.New("password change outcome insert was not observable")
	}
	return "", errors.Join(errPasswordChangeOutcomeUnknown, err, outcomeErr)
}

// IsPasswordChangeOutcomeUnknown 判断写入失败后是否连 durable receipt 也无法核对。
func IsPasswordChangeOutcomeUnknown(err error) bool {
	return errors.Is(err, errPasswordChangeOutcomeUnknown)
}

func (r *Repository) updatePasswordCredentialCASTx(
	ctx context.Context,
	tx *sql.Tx,
	credential PasswordCredential,
	expectedPasswordHash string,
) error {
	result, err := tx.ExecContext(
		ctx,
		`UPDATE auth_password_credentials
SET password_hash = `+r.bind(1)+`,
    password_algo = `+r.bind(2)+`,
    password_updated_at = `+r.bind(3)+`,
    updated_at = `+r.bind(4)+`
WHERE user_id = `+r.bind(5)+` AND password_hash = `+r.bind(6),
		credential.PasswordHash,
		credential.PasswordAlgo,
		credential.PasswordUpdatedAt,
		credential.UpdatedAt,
		credential.UserID,
		expectedPasswordHash,
	)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected != 1 {
		return ErrPasswordCredentialChanged
	}
	return nil
}

// PasswordChangeOutcome 查询 exact user/request 的 durable 终态；空值表示尚无终态。
func (r *Repository) PasswordChangeOutcome(
	ctx context.Context,
	userID string,
	requestID string,
) (PasswordChangeOutcome, error) {
	row := r.db.QueryRowContext(
		ctx,
		`SELECT effect FROM auth_password_change_receipts
WHERE user_id = `+r.bind(1)+` AND request_id = `+r.bind(2)+`
LIMIT 1`,
		strings.TrimSpace(userID),
		strings.TrimSpace(requestID),
	)
	var outcome PasswordChangeOutcome
	if err := row.Scan(&outcome); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	switch outcome {
	case PasswordChangeOutcomeCommitted, PasswordChangeOutcomeNotApplied:
		return outcome, nil
	default:
		return "", fmt.Errorf("invalid password change outcome %q", outcome)
	}
}

func (r *Repository) insertPasswordChangeOutcomeTx(
	ctx context.Context,
	tx *sql.Tx,
	userID string,
	requestID string,
	outcome PasswordChangeOutcome,
	resolvedAt time.Time,
) (bool, error) {
	result, err := tx.ExecContext(
		ctx,
		`INSERT INTO auth_password_change_receipts (
    user_id, request_id, effect, resolved_at, created_at
) VALUES (`+r.bind(1)+`, `+r.bind(2)+`, `+r.bind(3)+`, `+r.bind(4)+`, `+r.bind(5)+`)
ON CONFLICT(user_id, request_id) DO NOTHING`,
		userID,
		requestID,
		outcome,
		resolvedAt,
		resolvedAt,
	)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected == 1, nil
}

func (r *Repository) resolveExistingPasswordChangeOutcome(
	ctx context.Context,
	userID string,
	requestID string,
) (bool, error) {
	outcome, err := r.PasswordChangeOutcome(ctx, userID, requestID)
	if err != nil {
		return false, errors.Join(errPasswordChangeOutcomeUnknown, err)
	}
	switch outcome {
	case PasswordChangeOutcomeCommitted:
		return true, nil
	case PasswordChangeOutcomeNotApplied:
		return false, ErrPasswordChangeNotApplied
	default:
		return false, errors.New("password change outcome is not resolved")
	}
}

func (r *Repository) upsertPasswordCredentialTx(
	ctx context.Context,
	tx *sql.Tx,
	credential PasswordCredential,
) error {
	row := tx.QueryRowContext(
		ctx,
		`SELECT credential_id FROM auth_password_credentials WHERE user_id = `+r.bind(1)+` LIMIT 1`,
		credential.UserID,
	)
	var existingCredentialID string
	if err := row.Scan(&existingCredentialID); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO auth_password_credentials (
    credential_id, user_id, password_hash, password_algo, password_updated_at, created_at, updated_at
) VALUES (`+r.bind(1)+`, `+r.bind(2)+`, `+r.bind(3)+`, `+r.bind(4)+`, `+r.bind(5)+`, `+r.bind(6)+`, `+r.bind(7)+`)`,
			credential.CredentialID,
			credential.UserID,
			credential.PasswordHash,
			credential.PasswordAlgo,
			credential.PasswordUpdatedAt,
			credential.CreatedAt,
			credential.UpdatedAt,
		)
		return err
	}

	_, err := tx.ExecContext(
		ctx,
		`UPDATE auth_password_credentials
SET password_hash = `+r.bind(1)+`,
    password_algo = `+r.bind(2)+`,
    password_updated_at = `+r.bind(3)+`,
    updated_at = `+r.bind(4)+`
WHERE credential_id = `+r.bind(5),
		credential.PasswordHash,
		credential.PasswordAlgo,
		credential.PasswordUpdatedAt,
		credential.UpdatedAt,
		existingCredentialID,
	)
	return err
}
