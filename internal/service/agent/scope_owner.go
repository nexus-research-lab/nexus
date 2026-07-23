package agent

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

const systemOwnerUserID = authctx.SystemUserID

func scopedOwnerUserID(ctx context.Context) (string, bool) {
	if ownerUserID, ok := authctx.CurrentUserID(ctx); ok {
		return ownerUserID, true
	}
	// 无认证部署仍然是单用户作用域；只有明确没有认证状态的内部
	// maintenance/context 才保留空 owner，供专用的跨用户查询使用。
	if state, ok := authctx.StateFromContext(ctx); ok && !state.AuthRequired {
		return authctx.SystemUserID, true
	}
	return "", false
}

func effectiveOwnerUserID(ctx context.Context) string {
	return authctx.OwnerUserID(ctx)
}
