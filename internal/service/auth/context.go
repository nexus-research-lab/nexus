package auth

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

// WithPrincipal 把认证后的主体写入请求上下文。
func WithPrincipal(ctx context.Context, principal *Principal) context.Context {
	return authctx.WithPrincipal(ctx, principal)
}

// PrincipalFromContext 读取请求上下文中的主体。
func PrincipalFromContext(ctx context.Context) *Principal {
	return authctx.PrincipalFromContext(ctx)
}

// CurrentUserID 从上下文读取当前用户标识。
func CurrentUserID(ctx context.Context) (string, bool) {
	return authctx.CurrentUserID(ctx)
}

// OwnerUserID 返回当前请求的 owner 用户标识，未绑定认证主体时回落到本地单用户主体。
func OwnerUserID(ctx context.Context) string {
	return authctx.OwnerUserID(ctx)
}

// WithState 把认证系统状态写入请求上下文。
func WithState(ctx context.Context, state State) context.Context {
	return authctx.WithState(ctx, state)
}

// StateFromContext 读取请求上下文中的认证系统状态。
func StateFromContext(ctx context.Context) (State, bool) {
	return authctx.StateFromContext(ctx)
}
