package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/storage"
)

type controlOwnerBinding struct {
	DeploymentID  string
	ControlUserID string
	LocalOwnerKey string
}

type controlBindingStore struct {
	db           *sql.DB
	dialect      storage.SQLDialect
	projections  *ownerProjectionStore
	entitlements *controlEntitlementProjectionStore
}

func newControlBindingStore(cfgDriver string, db *sql.DB) *controlBindingStore {
	return &controlBindingStore{
		db:           db,
		dialect:      storage.NewSQLDialect(cfgDriver),
		projections:  newOwnerProjectionStore(cfgDriver, db),
		entitlements: newControlEntitlementProjectionStore(cfgDriver, db),
	}
}

func (s *controlBindingStore) resolve(
	ctx context.Context,
	principal controlPrincipal,
) (controlOwnerBinding, error) {
	if s == nil || s.db == nil {
		return controlOwnerBinding{}, errors.New("local owner binding store is not configured")
	}
	principal.normalize()
	if principal.DeploymentID == "" || principal.UserID == "" {
		return controlOwnerBinding{}, errors.New("Control Principal 缺少 deployment_id 或 user_id")
	}
	binding, found, err := s.load(ctx, principal)
	if err != nil || found {
		return binding, err
	}
	binding.LocalOwnerKey, err = s.initialLocalOwnerKey(ctx, principal)
	if err != nil {
		return controlOwnerBinding{}, err
	}
	claimed, err := s.create(ctx, binding, principal)
	if err == nil && claimed {
		return binding, nil
	}
	resolved, found, loadErr := s.load(ctx, principal)
	if loadErr == nil && found {
		return resolved, nil
	}
	if err != nil {
		return controlOwnerBinding{}, err
	}
	if loadErr != nil {
		return controlOwnerBinding{}, loadErr
	}
	return controlOwnerBinding{}, errors.New("Control identity binding claim 未产生结果")
}

func (s *controlBindingStore) load(
	ctx context.Context,
	principal controlPrincipal,
) (controlOwnerBinding, bool, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT b.local_owner_key, p.username, p.display_name, p.role, p.status, COALESCE(p.avatar, '')
FROM local_owner_bindings b
LEFT JOIN owner_profiles p ON p.owner_user_id = b.local_owner_key
WHERE b.deployment_id = `+s.dialect.Bind(1)+`
  AND b.control_user_id = `+s.dialect.Bind(2)+`
LIMIT 1`,
		principal.DeploymentID,
		principal.UserID,
	)
	binding := controlOwnerBinding{
		DeploymentID:  principal.DeploymentID,
		ControlUserID: principal.UserID,
	}
	var username, displayName, role, status, avatar sql.NullString
	err := row.Scan(&binding.LocalOwnerKey, &username, &displayName, &role, &status, &avatar)
	if err == nil {
		if username.String != principal.Username ||
			displayName.String != principal.DisplayName ||
			role.String != principal.Role ||
			status.String != UserStatusActive ||
			avatar.String != principal.Avatar {
			err = s.projections.upsert(ctx, controlOwnerProjection(binding.LocalOwnerKey, principal))
		}
		if err == nil {
			err = s.entitlements.upsert(ctx, binding.LocalOwnerKey, principal.Entitlement)
		}
		return binding, true, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return controlOwnerBinding{}, false, err
	}
	return binding, false, nil
}

func (s *controlBindingStore) initialLocalOwnerKey(
	ctx context.Context,
	principal controlPrincipal,
) (string, error) {
	var username string
	err := s.db.QueryRowContext(
		ctx,
		`SELECT username FROM owner_profiles WHERE owner_user_id = `+s.dialect.Bind(1),
		principal.UserID,
	).Scan(&username)
	if err == nil {
		if strings.TrimSpace(username) != principal.Username {
			return "", errors.New("Control user_id 与现有本地 owner 冲突")
		}
		return principal.UserID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	return deterministicControlOwnerKey(principal.DeploymentID, principal.UserID), nil
}

func deterministicControlOwnerKey(deploymentID, controlUserID string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(deploymentID) + "\x00" + strings.TrimSpace(controlUserID)))
	return fmt.Sprintf("owner_%x", digest[:16])
}

func (s *controlBindingStore) create(
	ctx context.Context,
	binding controlOwnerBinding,
	principal controlPrincipal,
) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	now := s.dialect.TimestampValue(time.Now().UTC())
	if err = s.projections.upsertWith(
		ctx,
		tx,
		controlOwnerProjection(binding.LocalOwnerKey, principal),
		now,
	); err != nil {
		return false, err
	}
	if err = s.entitlements.upsertWith(
		ctx,
		tx,
		binding.LocalOwnerKey,
		principal.Entitlement,
		now,
	); err != nil {
		return false, err
	}
	result, err := tx.ExecContext(
		ctx,
		`INSERT INTO local_owner_bindings (
    deployment_id, control_user_id, local_owner_key, created_at, updated_at
) VALUES (`+s.binds(1, 5)+`)
ON CONFLICT DO NOTHING`,
		binding.DeploymentID,
		binding.ControlUserID,
		binding.LocalOwnerKey,
		now,
		now,
	)
	if err != nil {
		return false, fmt.Errorf("创建 local owner binding: %w", err)
	}
	claimed, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("读取 local owner binding claim: %w", err)
	}
	if claimed == 0 {
		return false, nil
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func controlOwnerProjection(localOwnerKey string, principal controlPrincipal) ownerProjection {
	return ownerProjection{
		OwnerUserID: localOwnerKey,
		Username:    principal.Username,
		DisplayName: principal.DisplayName,
		Role:        principal.Role,
		Status:      UserStatusActive,
		Avatar:      principal.Avatar,
	}
}

func (s *controlBindingStore) controlIdentity(
	ctx context.Context,
	localOwnerKey string,
) (controlOwnerBinding, error) {
	var binding controlOwnerBinding
	err := s.db.QueryRowContext(
		ctx,
		`SELECT deployment_id, control_user_id, local_owner_key
FROM local_owner_bindings
WHERE local_owner_key = `+s.dialect.Bind(1)+`
LIMIT 1`,
		strings.TrimSpace(localOwnerKey),
	).Scan(&binding.DeploymentID, &binding.ControlUserID, &binding.LocalOwnerKey)
	if errors.Is(err, sql.ErrNoRows) {
		return controlOwnerBinding{}, ErrUserNotFound
	}
	return binding, err
}

func (s *controlBindingStore) localOwnerKey(
	ctx context.Context,
	deploymentID string,
	controlUserID string,
) (string, bool, error) {
	var localOwnerKey string
	err := s.db.QueryRowContext(
		ctx,
		`SELECT local_owner_key
FROM local_owner_bindings
WHERE deployment_id = `+s.dialect.Bind(1)+`
  AND control_user_id = `+s.dialect.Bind(2)+`
LIMIT 1`,
		strings.TrimSpace(deploymentID),
		strings.TrimSpace(controlUserID),
	).Scan(&localOwnerKey)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return localOwnerKey, err == nil, err
}

func (s *controlBindingStore) localOwnerKeys(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT local_owner_key
FROM local_owner_bindings
ORDER BY local_owner_key ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	owners := make([]string, 0)
	for rows.Next() {
		var owner string
		if err = rows.Scan(&owner); err != nil {
			return nil, err
		}
		owners = append(owners, owner)
	}
	return owners, rows.Err()
}

func (s *controlBindingStore) binds(first, last int) string {
	items := make([]string, 0, last-first+1)
	for index := first; index <= last; index++ {
		items = append(items, s.dialect.Bind(index))
	}
	return strings.Join(items, ", ")
}

func nullableControlString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}
