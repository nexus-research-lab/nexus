package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (r *Repository) CreateSession(ctx context.Context, record SessionRecord) error {
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

func (r *Repository) GetActiveSessionByTokenHash(
	ctx context.Context,
	tokenHash string,
	now time.Time,
) (*SessionRecord, *UserRecord, error) {
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
    u.avatar,
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
		record       SessionRecord
		user         UserRecord
		sessionIP    sql.NullString
		sessionAgent sql.NullString
		revokedAt    sql.NullTime
		lastLoginAt  sql.NullTime
		avatar       sql.NullString
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
		&avatar,
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
	user.Avatar = nullStringValue(avatar)
	user.LastLoginAt = nullTimePointer(lastLoginAt)
	user.CreatedAt = user.CreatedAt.UTC()
	user.UpdatedAt = user.UpdatedAt.UTC()
	return &record, &user, nil
}

func (r *Repository) TouchSession(ctx context.Context, sessionID string, now time.Time) error {
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

func (r *Repository) RevokeSessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) error {
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

func (r *Repository) CleanupExpiredSessions(ctx context.Context, now time.Time) error {
	_, err := r.db.ExecContext(
		ctx,
		`DELETE FROM auth_sessions WHERE expires_at <= `+r.bind(1)+` OR revoked_at IS NOT NULL`,
		now,
	)
	return err
}
