package auth

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

var (
	// ErrUserNotFound 表示用户不存在。
	ErrUserNotFound = errors.New("user not found")
)

type repository struct {
	db         *sql.DB
	isPostgres bool
}

func newRepository(cfg config.Config, db *sql.DB) *repository {
	return &repository{
		db:         db,
		isPostgres: storage.NormalizeSQLDriver(cfg.DatabaseDriver) == "pgx",
	}
}

func (r *repository) bind(index int) string {
	if r.isPostgres {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

func (r *repository) loadState(ctx context.Context, accessTokenEnabled bool) (State, error) {
	state := State{}
	userCount, err := r.scalarCount(ctx, "SELECT COUNT(*) FROM users WHERE status = "+r.bind(1), UserStatusActive)
	if err != nil {
		return state, err
	}
	passwordUserCount, err := r.scalarCount(
		ctx,
		`SELECT COUNT(*)
FROM auth_password_credentials c
INNER JOIN users u ON u.user_id = c.user_id
WHERE u.status = `+r.bind(1),
		UserStatusActive,
	)
	if err != nil {
		return state, err
	}
	state.UserCount = userCount
	state.PasswordUserCount = passwordUserCount
	state.SetupRequired = userCount == 0
	state.PasswordLoginEnabled = passwordUserCount > 0
	state.AccessTokenEnabled = accessTokenEnabled && userCount == 0
	state.AuthRequired = userCount > 0 || state.AccessTokenEnabled
	return state, nil
}

func (r *repository) scalarCount(ctx context.Context, query string, args ...any) (int, error) {
	row := r.db.QueryRowContext(ctx, query, args...)
	var value int
	if err := row.Scan(&value); err != nil {
		return 0, err
	}
	return value, nil
}

func (r *repository) getUserWithPasswordByUsername(
	ctx context.Context,
	username string,
) (*User, *passwordCredential, error) {
	return r.getUserWithPassword(ctx, "username", username)
}

func (r *repository) getUserWithPasswordByID(
	ctx context.Context,
	userID string,
) (*User, *passwordCredential, error) {
	return r.getUserWithPassword(ctx, "user_id", userID)
}

func (r *repository) getUserWithPassword(
	ctx context.Context,
	field string,
	value string,
) (*User, *passwordCredential, error) {
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
		user         User
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
	user.LastLoginAt = nullTimePointer(lastLoginAt)
	if !credentialID.Valid {
		return &user, nil, nil
	}
	credential := &passwordCredential{
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

func (r *repository) getUserByID(ctx context.Context, userID string) (*User, error) {
	return r.getUser(ctx, "user_id", userID)
}

func (r *repository) getUserByUsername(ctx context.Context, username string) (*User, error) {
	return r.getUser(ctx, "username", username)
}

func (r *repository) getUser(ctx context.Context, field string, value string) (*User, error) {
	if field != "user_id" && field != "username" {
		return nil, fmt.Errorf("unsupported user field: %s", field)
	}
	row := r.db.QueryRowContext(
		ctx,
		`SELECT user_id, username, display_name, role, status, last_login_at, created_at, updated_at
FROM users
WHERE `+field+` = `+r.bind(1)+`
LIMIT 1`,
		strings.TrimSpace(value),
	)
	var (
		user      User
		lastLogin sql.NullTime
	)
	if err := row.Scan(
		&user.UserID,
		&user.Username,
		&user.DisplayName,
		&user.Role,
		&user.Status,
		&lastLogin,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	user.LastLoginAt = nullTimePointer(lastLogin)
	user.CreatedAt = user.CreatedAt.UTC()
	user.UpdatedAt = user.UpdatedAt.UTC()
	return &user, nil
}

func (r *repository) listUsers(ctx context.Context) ([]User, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT user_id, username, display_name, role, status, last_login_at, created_at, updated_at
FROM users
ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]User, 0)
	for rows.Next() {
		var (
			user      User
			lastLogin sql.NullTime
		)
		if err = rows.Scan(
			&user.UserID,
			&user.Username,
			&user.DisplayName,
			&user.Role,
			&user.Status,
			&lastLogin,
			&user.CreatedAt,
			&user.UpdatedAt,
		); err != nil {
			return nil, err
		}
		user.LastLoginAt = nullTimePointer(lastLogin)
		user.CreatedAt = user.CreatedAt.UTC()
		user.UpdatedAt = user.UpdatedAt.UTC()
		items = append(items, user)
	}
	return items, rows.Err()
}

func (r *repository) createUserWithPassword(
	ctx context.Context,
	user User,
	credential passwordCredential,
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

	if _, err = tx.ExecContext(
		ctx,
		`INSERT INTO users (
    user_id, username, display_name, role, status, last_login_at, created_at, updated_at
) VALUES (`+r.bind(1)+`, `+r.bind(2)+`, `+r.bind(3)+`, `+r.bind(4)+`, `+r.bind(5)+`, `+r.bind(6)+`, `+r.bind(7)+`, `+r.bind(8)+`)`,
		user.UserID,
		user.Username,
		user.DisplayName,
		user.Role,
		user.Status,
		nil,
		user.CreatedAt,
		user.UpdatedAt,
	); err != nil {
		return err
	}

	if err = r.upsertPasswordCredentialTx(ctx, tx, credential); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *repository) upsertPasswordCredential(
	ctx context.Context,
	credential passwordCredential,
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

func (r *repository) upsertPasswordCredentialTx(
	ctx context.Context,
	tx *sql.Tx,
	credential passwordCredential,
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

func (r *repository) createSession(ctx context.Context, record sessionRecord) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO auth_sessions (
    session_id, user_id, session_token_hash, auth_method, expires_at, last_seen_at,
    client_ip, user_agent, revoked_at, created_at, updated_at
) VALUES (`+r.bind(1)+`, `+r.bind(2)+`, `+r.bind(3)+`, `+r.bind(4)+`, `+r.bind(5)+`, `+r.bind(6)+`,
`+r.bind(7)+`, `+r.bind(8)+`, `+r.bind(9)+`, `+r.bind(10)+`, `+r.bind(11)+`)`,
		record.SessionID,
		record.UserID,
		record.SessionTokenHash,
		record.AuthMethod,
		record.ExpiresAt,
		record.LastSeenAt,
		nullableString(record.ClientIP),
		nullableString(record.UserAgent),
		nil,
		record.CreatedAt,
		record.UpdatedAt,
	)
	return err
}

func (r *repository) getActiveSessionByTokenHash(
	ctx context.Context,
	tokenHash string,
	now time.Time,
) (*sessionRecord, *User, error) {
	row := r.db.QueryRowContext(
		ctx,
		`SELECT
    s.session_id,
    s.user_id,
    s.session_token_hash,
    s.auth_method,
    s.expires_at,
    s.last_seen_at,
    s.client_ip,
    s.user_agent,
    s.revoked_at,
    s.created_at,
    s.updated_at,
    u.user_id,
    u.username,
    u.display_name,
    u.role,
    u.status,
    u.last_login_at,
    u.created_at,
    u.updated_at
FROM auth_sessions s
INNER JOIN users u ON u.user_id = s.user_id
WHERE s.session_token_hash = `+r.bind(1)+`
  AND s.revoked_at IS NULL
  AND s.expires_at > `+r.bind(2)+`
LIMIT 1`,
		tokenHash,
		now,
	)
	var (
		record       sessionRecord
		user         User
		sessionIP    sql.NullString
		sessionAgent sql.NullString
		revokedAt    sql.NullTime
		lastLoginAt  sql.NullTime
	)
	if err := row.Scan(
		&record.SessionID,
		&record.UserID,
		&record.SessionTokenHash,
		&record.AuthMethod,
		&record.ExpiresAt,
		&record.LastSeenAt,
		&sessionIP,
		&sessionAgent,
		&revokedAt,
		&record.CreatedAt,
		&record.UpdatedAt,
		&user.UserID,
		&user.Username,
		&user.DisplayName,
		&user.Role,
		&user.Status,
		&lastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	record.ClientIP = nullStringValue(sessionIP)
	record.UserAgent = nullStringValue(sessionAgent)
	record.RevokedAt = nullTimePointer(revokedAt)
	record.ExpiresAt = record.ExpiresAt.UTC()
	record.LastSeenAt = record.LastSeenAt.UTC()
	record.CreatedAt = record.CreatedAt.UTC()
	record.UpdatedAt = record.UpdatedAt.UTC()
	user.LastLoginAt = nullTimePointer(lastLoginAt)
	user.CreatedAt = user.CreatedAt.UTC()
	user.UpdatedAt = user.UpdatedAt.UTC()
	return &record, &user, nil
}

func (r *repository) touchSession(ctx context.Context, sessionID string, now time.Time) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE auth_sessions
SET last_seen_at = `+r.bind(1)+`, updated_at = `+r.bind(2)+`
WHERE session_id = `+r.bind(3),
		now,
		now,
		sessionID,
	)
	return err
}

func (r *repository) revokeSessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE auth_sessions
SET revoked_at = `+r.bind(1)+`, updated_at = `+r.bind(2)+`
WHERE session_token_hash = `+r.bind(3)+` AND revoked_at IS NULL`,
		now,
		now,
		tokenHash,
	)
	return err
}

func (r *repository) updateUserLastLogin(ctx context.Context, userID string, now time.Time) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE users
SET last_login_at = `+r.bind(1)+`, updated_at = `+r.bind(2)+`
WHERE user_id = `+r.bind(3),
		now,
		now,
		userID,
	)
	return err
}

func (r *repository) cleanupExpiredSessions(ctx context.Context, now time.Time) error {
	_, err := r.db.ExecContext(
		ctx,
		`DELETE FROM auth_sessions WHERE expires_at <= `+r.bind(1)+` OR revoked_at IS NOT NULL`,
		now,
	)
	return err
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	normalized := value.Time.UTC()
	return &normalized
}
