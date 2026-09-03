package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const ControlIdentityInvalidationBatchSize = 256

// ControlIdentityInvalidationCursor 返回本 Nexus 副本已持久应用的 Control 事件游标。
func (a *ControlAuthority) ControlIdentityInvalidationCursor(ctx context.Context) (int64, error) {
	var cursor int64
	err := a.bindings.db.QueryRowContext(ctx, `
SELECT identity_invalidation_cursor
FROM control_projection_state
WHERE singleton_id = 1`).Scan(&cursor)
	if err != nil || cursor < 0 {
		return 0, errors.Join(err, errors.New("Control identity invalidation cursor 无效"))
	}
	return cursor, nil
}

// CommitControlIdentityInvalidationCursor 只在本地投影成功后单调推进副本游标。
func (a *ControlAuthority) CommitControlIdentityInvalidationCursor(
	ctx context.Context,
	cursor int64,
) error {
	if cursor <= 0 {
		return errors.New("Control identity invalidation cursor 无效")
	}
	result, err := a.bindings.db.ExecContext(ctx, `
UPDATE control_projection_state
SET identity_invalidation_cursor = `+a.bindings.dialect.Bind(1)+`
WHERE singleton_id = 1 AND identity_invalidation_cursor < `+a.bindings.dialect.Bind(2), cursor, cursor)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		current, loadErr := a.ControlIdentityInvalidationCursor(ctx)
		// 共享数据库中的其他副本可能已先推进水位；当前副本仍需继续本机失效流程。
		if loadErr != nil || current < cursor {
			return errors.Join(loadErr, errors.New("Control identity invalidation cursor 未推进"))
		}
	}
	return nil
}

// ControlIdentityInvalidations 读取给定游标之后的有序身份变更。
func (a *ControlAuthority) ControlIdentityInvalidations(
	ctx context.Context,
	after int64,
) ([]ControlIdentityInvalidation, error) {
	if after < 0 {
		return nil, errors.New("Control identity invalidation cursor 无效")
	}
	var response controlInvalidationBatch
	path := fmt.Sprintf(
		"/internal/identity-invalidations?after=%d&limit=%d",
		after,
		ControlIdentityInvalidationBatchSize,
	)
	if err := a.call(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	last := after
	for index := range response.Events {
		event := &response.Events[index]
		event.DeploymentID = strings.TrimSpace(event.DeploymentID)
		event.UserID = strings.TrimSpace(event.UserID)
		event.SessionID = strings.TrimSpace(event.SessionID)
		event.Reason = strings.TrimSpace(event.Reason)
		if event.EventID <= last || event.DeploymentID == "" || event.UserID == "" {
			return nil, errors.New("Control identity invalidation event 无效或乱序")
		}
		switch event.Reason {
		case "principal_changed", "profile_changed", "entitlement_changed":
			if event.SessionID != "" {
				return nil, errors.New("Control identity invalidation session_id 与 reason 不匹配")
			}
		case "session_revoked":
			if event.SessionID == "" {
				return nil, errors.New("Control session invalidation 缺少 session_id")
			}
		default:
			return nil, errors.New("Control identity invalidation reason 无效")
		}
		last = event.EventID
	}
	if response.NextCursor != last {
		return nil, errors.New("Control identity invalidation next_cursor 与事件不一致")
	}
	return response.Events, nil
}

// ApplyControlIdentityInvalidation 丢弃身份租约并返回对应的本地 owner。
func (a *ControlAuthority) ApplyControlIdentityInvalidation(
	ctx context.Context,
	event ControlIdentityInvalidation,
) (string, error) {
	localOwnerKey, found, err := a.bindings.localOwnerKey(
		ctx,
		event.DeploymentID,
		event.UserID,
	)
	if err != nil || !found {
		return "", err
	}
	a.humanSessionMu.Lock()
	defer a.humanSessionMu.Unlock()
	if event.Reason == "session_revoked" {
		a.deleteSessionLeases(event.SessionID)
	} else {
		a.deleteOwnerLeases(localOwnerKey)
	}
	if event.Reason == "entitlement_changed" {
		return localOwnerKey, a.refreshEntitlementProjection(
			ctx,
			localOwnerKey,
			event.DeploymentID,
			event.UserID,
		)
	}
	if event.Reason != "principal_changed" {
		return localOwnerKey, nil
	}
	return localOwnerKey, a.refreshPrincipalProjection(ctx, localOwnerKey, event.UserID)
}

func (a *ControlAuthority) refreshEntitlementProjection(
	ctx context.Context,
	localOwnerKey string,
	deploymentID string,
	controlUserID string,
) error {
	entitlement, err := a.controlEntitlement(ctx, deploymentID, controlUserID)
	if err != nil {
		return err
	}
	return a.bindings.entitlements.upsert(ctx, localOwnerKey, entitlement)
}

func (a *ControlAuthority) refreshPrincipalProjection(
	ctx context.Context,
	localOwnerKey string,
	controlUserID string,
) error {
	projection, found, err := a.bindings.projections.load(ctx, localOwnerKey)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("Control owner binding 缺少本地资料投影")
	}
	role, err := a.controlRole(ctx, controlUserID)
	if errors.Is(err, ErrUserNotFound) {
		projection.Status = UserStatusDisabled
		return a.bindings.projections.upsert(ctx, projection)
	}
	if err != nil {
		return err
	}
	projection.Role = role
	projection.Status = UserStatusActive
	return a.bindings.projections.upsert(ctx, projection)
}

// FailClosedControlIdentities 清空所有 Control 租约并返回全部已绑定 owner。
func (a *ControlAuthority) FailClosedControlIdentities(ctx context.Context) ([]string, error) {
	owners, err := a.bindings.localOwnerKeys(ctx)
	if err != nil {
		return nil, err
	}
	a.leaseMu.Lock()
	clear(a.leases)
	a.leaseMu.Unlock()
	return owners, nil
}
