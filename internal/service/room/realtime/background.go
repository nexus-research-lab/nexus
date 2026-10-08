package realtime

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
)

// 绑定到共享 conversation session。删除 Room 或撤销 owner 时，runtime manager
// 会先取消并等待任务，避免清理目录后又被旧 goroutine 重建。
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
	if s.Runtime != nil {
		s.Runtime.StartBackgroundTaskForOwner(sessionKey, ownerUserID, run)
		return
	}
	// 精简嵌入或单元测试没有 runtime manager 时，不再制造无法取消、
	// 无法等待的孤儿 goroutine。同步执行至少保证调用返回时写盘已经收敛。
	run(context.Background())
}
