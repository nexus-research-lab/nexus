package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

func (r *Repository) GetUserByID(ctx context.Context, userID string) (*UserRecord, error) {
	return r.getUser(ctx, "user_id", userID)
}

func (r *Repository) GetUserByUsername(ctx context.Context, username string) (*UserRecord, error) {
	return r.getUser(ctx, "username", username)
}

func (r *Repository) getUser(ctx context.Context, field string, value string) (*UserRecord, error) {
	if field != "user_id" && field != "username" {
		return nil, fmt.Errorf("unsupported user field: %s", field)
	}
	row := r.db.QueryRowContext(
		ctx,
		`SELECT user_id, username, display_name, role, status, avatar, last_login_at, created_at, updated_at
FROM users
WHERE `+field+` = `+r.bind(1)+`
LIMIT 1`,
		strings.TrimSpace(value),
	)
	var (
		user      UserRecord
		avatar    sql.NullString
		lastLogin sql.NullTime
	)
	if err := row.Scan(
		&user.UserID,
		&user.Username,
		&user.DisplayName,
		&user.Role,
		&user.Status,
		&avatar,
		&lastLogin,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	user.Avatar = nullStringValue(avatar)
	user.LastLoginAt = nullTimePointer(lastLogin)
	user.CreatedAt = user.CreatedAt.UTC()
	user.UpdatedAt = user.UpdatedAt.UTC()
	return &user, nil
}

func (r *Repository) ListUsers(ctx context.Context) ([]UserRecord, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT user_id, username, display_name, role, status, avatar, last_login_at, created_at, updated_at
FROM users
WHERE user_id <> `+r.bind(1)+`
ORDER BY created_at ASC`,
		authctx.SystemUserID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]UserRecord, 0)
	for rows.Next() {
		var (
			user      UserRecord
			avatar    sql.NullString
			lastLogin sql.NullTime
		)
		if err = rows.Scan(
			&user.UserID,
			&user.Username,
			&user.DisplayName,
			&user.Role,
			&user.Status,
			&avatar,
			&lastLogin,
			&user.CreatedAt,
			&user.UpdatedAt,
		); err != nil {
			return nil, err
		}
		user.Avatar = nullStringValue(avatar)
		user.LastLoginAt = nullTimePointer(lastLogin)
		user.CreatedAt = user.CreatedAt.UTC()
		user.UpdatedAt = user.UpdatedAt.UTC()
		items = append(items, user)
	}
	return items, rows.Err()
}

func (r *Repository) UpsertLocalUser(ctx context.Context, user UserRecord) error {
	if strings.TrimSpace(user.UserID) != authctx.SystemUserID {
		return fmt.Errorf("local user upsert only supports %s", authctx.SystemUserID)
	}
	existing, err := r.GetUserByID(ctx, user.UserID)
	if err != nil {
		return err
	}
	if existing == nil {
		_, err = r.db.ExecContext(
			ctx,
			`INSERT INTO users (
    user_id, username, display_name, role, status, avatar, last_login_at, created_at, updated_at
) VALUES (`+r.bind(1)+`, `+r.bind(2)+`, `+r.bind(3)+`, `+r.bind(4)+`, `+r.bind(5)+`, `+r.bind(6)+`, `+r.bind(7)+`, `+r.bind(8)+`, `+r.bind(9)+`)`,
			user.UserID,
			user.Username,
			user.DisplayName,
			user.Role,
			user.Status,
			nullableString(user.Avatar),
			nil,
			user.CreatedAt,
			user.UpdatedAt,
		)
		return err
	}
	_, err = r.db.ExecContext(
		ctx,
		`UPDATE users
SET username = `+r.bind(1)+`, display_name = `+r.bind(2)+`, role = `+r.bind(3)+`, status = `+r.bind(4)+`, avatar = `+r.bind(5)+`, updated_at = `+r.bind(6)+`
WHERE user_id = `+r.bind(7),
		user.Username,
		user.DisplayName,
		user.Role,
		user.Status,
		nullableString(user.Avatar),
		user.UpdatedAt,
		user.UserID,
	)
	return err
}

func (r *Repository) CreateUserWithPassword(
	ctx context.Context,
	user UserRecord,
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

	if _, err = tx.ExecContext(
		ctx,
		`INSERT INTO users (
    user_id, username, display_name, role, status, avatar, last_login_at, created_at, updated_at
) VALUES (`+r.bind(1)+`, `+r.bind(2)+`, `+r.bind(3)+`, `+r.bind(4)+`, `+r.bind(5)+`, `+r.bind(6)+`, `+r.bind(7)+`, `+r.bind(8)+`, `+r.bind(9)+`)`,
		user.UserID,
		user.Username,
		user.DisplayName,
		user.Role,
		user.Status,
		nullableString(user.Avatar),
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

func (r *Repository) UpdateUserLastLogin(ctx context.Context, userID string, now time.Time) error {
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

func (r *Repository) UpdateUserAvatar(ctx context.Context, userID string, avatar string, now time.Time) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE users
SET avatar = `+r.bind(1)+`, updated_at = `+r.bind(2)+`
WHERE user_id = `+r.bind(3),
		nullableString(avatar),
		now,
		userID,
	)
	return err
}
