package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/storage"
)

type ownerProjection struct {
	OwnerUserID string
	Username    string
	DisplayName string
	Role        string
	Status      string
	Avatar      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ownerProjectionStore struct {
	db      *sql.DB
	dialect storage.SQLDialect
}

type ownerProjectionExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func newOwnerProjectionStore(cfgDriver string, db *sql.DB) *ownerProjectionStore {
	return &ownerProjectionStore{db: db, dialect: storage.NewSQLDialect(cfgDriver)}
}

func (s *ownerProjectionStore) load(
	ctx context.Context,
	ownerUserID string,
) (ownerProjection, bool, error) {
	var projection ownerProjection
	var avatar sql.NullString
	err := s.db.QueryRowContext(ctx, `
SELECT owner_user_id, username, display_name, role, status, avatar, created_at, updated_at
FROM owner_profiles
WHERE owner_user_id = `+s.dialect.Bind(1)+`
LIMIT 1`, strings.TrimSpace(ownerUserID)).Scan(
		&projection.OwnerUserID,
		&projection.Username,
		&projection.DisplayName,
		&projection.Role,
		&projection.Status,
		&avatar,
		&projection.CreatedAt,
		&projection.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ownerProjection{}, false, nil
	}
	projection.Avatar = avatar.String
	return projection, err == nil, err
}

func (s *ownerProjectionStore) upsert(
	ctx context.Context,
	projection ownerProjection,
) error {
	return s.upsertWith(ctx, s.db, projection, s.dialect.TimestampValue(time.Now().UTC()))
}

func (s *ownerProjectionStore) upsertWith(
	ctx context.Context,
	executor ownerProjectionExecutor,
	projection ownerProjection,
	now any,
) error {
	_, err := executor.ExecContext(ctx, `
INSERT INTO owner_profiles (
    owner_user_id, username, display_name, role, status, avatar, created_at, updated_at
) VALUES (`+s.dialect.BindList(8)+`)
ON CONFLICT(owner_user_id) DO UPDATE SET
    username = excluded.username,
    display_name = excluded.display_name,
    role = excluded.role,
    status = excluded.status,
    avatar = excluded.avatar,
    updated_at = excluded.updated_at`,
		projection.OwnerUserID,
		projection.Username,
		projection.DisplayName,
		projection.Role,
		projection.Status,
		nullableControlString(projection.Avatar),
		now,
		now,
	)
	return err
}
