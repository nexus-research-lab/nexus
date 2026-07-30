package dm

import (
	"context"
	"strings"
)

// startSessionBackgroundTask 把仍会写入 workspace 的 DM 后台工作绑定到
// runtime session。session 被关闭或 owner 权限撤销时，runtime manager 会取消
// 并等待这些任务，避免清理目录后又被异步创建。
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
		ctx = contextWithExactOwner(ctx, ownerUserID)
		if ctx.Err() != nil {
			return
		}
		task(ctx)
	}
	if s.runtime == nil {
		// 没有 runtime manager 时通常是精简测试服务；同步执行，避免测试
		// 返回后临时 workspace 被清理，而后台协程仍在写入。
		run(context.Background())
		return
	}
	s.runtime.StartBackgroundTaskForOwner(sessionKey, ownerUserID, run)
}
