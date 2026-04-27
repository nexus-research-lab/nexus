package authctx

import (
	"context"
	"strings"
)

type principalContextKey struct{}
type stateContextKey struct{}

// WithPrincipal 把认证后的主体写入请求上下文。
func WithPrincipal(ctx context.Context, principal *Principal) context.Context {
	if principal == nil {
		return ctx
	}
	return context.WithValue(ctx, principalContextKey{}, principal)
}

// PrincipalFromContext 读取请求上下文中的主体。
func PrincipalFromContext(ctx context.Context) *Principal {
	principal, _ := ctx.Value(principalContextKey{}).(*Principal)
	return principal
}

// CurrentUserID 从上下文读取当前用户标识。
func CurrentUserID(ctx context.Context) (string, bool) {
	principal := PrincipalFromContext(ctx)
	if principal == nil {
		return "", false
	}
	userID := strings.TrimSpace(principal.UserID)
	if userID == "" {
		return "", false
	}
	return userID, true
}

// OwnerUserID 返回当前请求的 owner 用户标识，未绑定认证主体时回落到本地单用户主体。
func OwnerUserID(ctx context.Context) string {
	if userID, ok := CurrentUserID(ctx); ok {
		return userID
	}
	return SystemUserID
}

// WithState 把认证系统状态写入请求上下文。
func WithState(ctx context.Context, state State) context.Context {
	return context.WithValue(ctx, stateContextKey{}, state)
}

// StateFromContext 读取请求上下文中的认证系统状态。
func StateFromContext(ctx context.Context) (State, bool) {
	state, ok := ctx.Value(stateContextKey{}).(State)
	return state, ok
}
