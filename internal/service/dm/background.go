package dm

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
)

// runtime session。session 被关闭或 owner 权限撤销时，runtime manager 会取消
// 并等待这些任务，避免清理目录后又被异步创建。
func (s *Service) StartSessionBackgroundTask(
	sessionKey string,
	ownerUserID string,
	task func(context.Context),
) {
	if task == nil {
		return
	}
	sessionKey = strings.TrimSpace(sessionKey)
	ownerUserID = strings.TrimSpace(ownerUserID)
	run := func(ctx context.Context) {
		ctx = runtimehost.ContextWithExactOwner(ctx, ownerUserID)
		if ctx.Err() != nil {
			return
		}
		task(ctx)
	}
	s.Runtime.StartBackgroundTaskForOwner(sessionKey, ownerUserID, run)
}
