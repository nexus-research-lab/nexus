package realtime

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

// startSessionBackgroundTask 把会继续写入 Room workspace/ledger 的异步工作
// 绑定到共享 conversation session。删除 Room 或撤销 owner 时，runtime manager
// 会先取消并等待任务，避免清理目录后又被旧 goroutine 重建。
func (s *Service) startSessionBackgroundTask(
	sessionKey string,
	ownerUserID string,
	task func(context.Context),
) {
	if s == nil || task == nil {
		return
	}
	sessionKey = strings.TrimSpace(sessionKey)
	ownerUserID = strings.TrimSpace(ownerUserID)
	run := func(ctx context.Context) {
		ctx = contextWithExactQueueOwner(ctx, ownerUserID)
		if ctx.Err() != nil {
			return
		}
		task(ctx)
	}
	if s.runtime != nil {
		s.runtime.StartBackgroundTaskForOwner(sessionKey, ownerUserID, run)
		return
	}
	// 精简嵌入或单元测试没有 runtime manager 时，不再制造无法取消、
	// 无法等待的孤儿 goroutine。同步执行至少保证调用返回时写盘已经收敛。
	run(context.Background())
}

func contextWithExactQueueOwner(ctx context.Context, ownerUserID string) context.Context {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return ctx
	}
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID: ownerUserID,
		Role:   authctx.RoleOwner,
	})
}
