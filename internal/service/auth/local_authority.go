// INPUT: Desktop 宿主认证证据与本地 owner 资料投影。
// OUTPUT: 无密码、无浏览器 Session 的单一本地主体。
// POS: Desktop Local 的完整认证边界；不实现 Web 账号系统。
package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/runtimeadmission"
)

const (
	localDesktopUsername    = "local"
	localDesktopDisplayName = "Local User"
)

// LocalAuthority 为 Desktop 提供固定的本地 owner。
type LocalAuthority struct {
	profiles         *ownerProjectionStore
	runtimeAdmission RuntimeAdmissionCoordinator
}

func NewLocalAuthority(
	cfgDriver string,
	db *sql.DB,
	runtimeAdmission RuntimeAdmissionCoordinator,
) *LocalAuthority {
	return &LocalAuthority{
		profiles:         newOwnerProjectionStore(cfgDriver, db),
		runtimeAdmission: runtimeAdmission,
	}
}

func (a *LocalAuthority) InspectRequest(
	ctx context.Context,
	_ *http.Request,
) (*Principal, State, error) {
	principal, err := a.principal(ctx)
	return principal, localAuthState(), err
}

func (a *LocalAuthority) BuildStatusPayload(
	ctx context.Context,
	request *http.Request,
) (StatusPayload, error) {
	principal, state, err := a.InspectRequest(ctx, request)
	if err != nil {
		return StatusPayload{}, err
	}
	return StatusPayload{
		AuthRequired:         state.AuthRequired,
		PasswordLoginEnabled: state.PasswordLoginEnabled,
		Authenticated:        true,
		Username:             stringPointer(principal.Username),
		UserID:               stringPointer(principal.UserID),
		DisplayName:          stringPointer(principal.DisplayName),
		Role:                 stringPointer(principal.Role),
		Avatar:               stringPointer(principal.Avatar),
		AuthMethod:           stringPointer(principal.AuthMethod),
		SetupRequired:        state.SetupRequired,
		SetupEnabled:         state.SetupEnabled,
	}, nil
}

func (a *LocalAuthority) BeginAgentRuntimeAdmission(
	ctx context.Context,
) (*runtimeadmission.Lease, error) {
	if a.runtimeAdmission == nil {
		return runtimeadmission.NewDetachedLease(ctx), nil
	}
	return a.runtimeAdmission.BeginRuntimeAdmission(ctx)
}

func (a *LocalAuthority) VerifyInteractiveHuman(
	ctx context.Context,
	principal *Principal,
) (*Principal, error) {
	if principal == nil ||
		principal.UserID != SystemUserID ||
		principal.AuthMethod != AuthMethodLocal {
		return nil, errors.New("desktop human principal is required")
	}
	evidence, ok := authctx.InteractiveHumanEvidenceFromContext(ctx)
	if !ok || evidence.Source != "desktop_session_token" {
		return nil, errors.New("desktop human-presence evidence is required")
	}
	return a.principal(ctx)
}

func (a *LocalAuthority) VerifyBoundInteractiveHuman(
	ctx context.Context,
	userID string,
	authMethod string,
	sessionID string,
) (*Principal, error) {
	if strings.TrimSpace(userID) != SystemUserID ||
		strings.TrimSpace(authMethod) != AuthMethodLocal ||
		strings.TrimSpace(sessionID) != "" {
		return nil, errors.New("desktop human identity is no longer active")
	}
	return a.principal(ctx)
}

func (a *LocalAuthority) AcquireBoundInteractiveHumanLease(
	ctx context.Context,
	userID string,
	authMethod string,
	sessionID string,
) (*Principal, func(), error) {
	principal, err := a.VerifyBoundInteractiveHuman(ctx, userID, authMethod, sessionID)
	if err != nil {
		return nil, nil, err
	}
	return principal, func() {}, nil
}

func (a *LocalAuthority) ResolveActivePrincipalRole(
	_ context.Context,
	userID string,
) (string, error) {
	if strings.TrimSpace(userID) != SystemUserID {
		return "", ErrUserNotFound
	}
	return RoleOwner, nil
}

// UpdateLocalAvatar 更新 Desktop 本地主体的展示资料。
func (a *LocalAuthority) UpdateLocalAvatar(ctx context.Context, avatar string) (*Principal, error) {
	normalizedAvatar, err := normalizeAvatar(avatar)
	if err != nil {
		return nil, err
	}
	projection := localOwnerProjection(normalizedAvatar)
	if err = a.profiles.upsert(ctx, projection); err != nil {
		return nil, err
	}
	return projectionPrincipal(projection), nil
}

func (a *LocalAuthority) principal(ctx context.Context) (*Principal, error) {
	projection, found, err := a.profiles.load(ctx, SystemUserID)
	if err != nil {
		return nil, err
	}
	needsRefresh := !found ||
		projection.Username != localDesktopUsername ||
		projection.DisplayName != localDesktopDisplayName ||
		projection.Role != RoleOwner ||
		projection.Status != UserStatusActive
	avatar := ""
	if found {
		avatar = projection.Avatar
	}
	projection = localOwnerProjection(avatar)
	if needsRefresh {
		if err = a.profiles.upsert(ctx, projection); err != nil {
			return nil, err
		}
	}
	return projectionPrincipal(projection), nil
}

func localOwnerProjection(avatar string) ownerProjection {
	now := time.Now().UTC()
	return ownerProjection{
		OwnerUserID: SystemUserID,
		Username:    localDesktopUsername,
		DisplayName: localDesktopDisplayName,
		Role:        RoleOwner,
		Status:      UserStatusActive,
		Avatar:      strings.TrimSpace(avatar),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func projectionPrincipal(projection ownerProjection) *Principal {
	return &Principal{
		UserID:      projection.OwnerUserID,
		Username:    projection.Username,
		DisplayName: projection.DisplayName,
		Role:        RoleOwner,
		Avatar:      projection.Avatar,
		AuthMethod:  AuthMethodLocal,
	}
}

func localAuthState() State {
	return State{
		AuthRequired:         false,
		PasswordLoginEnabled: false,
		SetupRequired:        false,
		SetupEnabled:         false,
	}
}
