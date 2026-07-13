package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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
