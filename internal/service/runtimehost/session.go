// INPUT: 会话宿主的 session key、owner、runtime client 与输入队列条目。
// OUTPUT: DM 与 Room 共用的后台任务、owner 上下文、fork runtime 回收与队列恢复阶段。
// POS: 两个宿主逐字相同的会话生命周期阶段；会话拓扑仍由各宿主决定。
package runtimehost

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// ToolSurfaceRetireTimeout 覆盖 process transport 的优雅退出与强制回收两个阶段；
// 宿主必须等完两阶段，否则会把即将完成的安全换代误报为失败。
const ToolSurfaceRetireTimeout = 2*runtimectx.RoundIdleAbortTimeout + time.Second

// RetireExistingRuntimeClient 在工具面换代前退休旧 runtime，不受调用方取消影响。
func RetireExistingRuntimeClient(ctx context.Context, startup *runtimectx.ClientStartup) (bool, error) {
	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), ToolSurfaceRetireTimeout)
	defer cancel()
	return startup.RetireExisting(closeCtx)
}

// ContextWithExactOwner 让后台任务以 owner 身份运行；已是同一用户时保留原 principal。
func ContextWithExactOwner(ctx context.Context, ownerUserID string) context.Context {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return ctx
	}
	if currentUserID, ok := authctx.CurrentUserID(ctx); ok && currentUserID == ownerUserID {
		return ctx
	}
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID: ownerUserID,
		Role:   authctx.RoleOwner,
	})
}

// StartSessionBackgroundTask 在 runtime manager 的 session 后台任务中以 owner 身份执行 task。
func (h *Host) StartSessionBackgroundTask(sessionKey string, ownerUserID string, task func(context.Context)) {
	if task == nil {
		return
	}
	sessionKey = strings.TrimSpace(sessionKey)
	ownerUserID = strings.TrimSpace(ownerUserID)
	h.Runtime.StartBackgroundTaskForOwner(sessionKey, ownerUserID, func(ctx context.Context) {
		ctx = ContextWithExactOwner(ctx, ownerUserID)
		if ctx.Err() != nil {
			return
		}
		task(ctx)
	})
}

// RestoreInputQueueItems 把未投递的引导条目原样放回输入队列。
func (h *Host) RestoreInputQueueItems(
	location workspacestore.InputQueueLocation,
	items []protocol.InputQueueItem,
) ([]protocol.InputQueueItem, error) {
	entries := make([]workspacestore.InputQueueEnqueue, 0, len(items))
	for _, item := range items {
		entries = append(entries, workspacestore.InputQueueEnqueue{Location: location, Item: item})
	}
	return h.InputQueue.EnqueueBatchWithItems(entries)
}

// CloseUncommittedForkRuntime 关闭 fork 失败后仍持有该 session 的 runtime client。
func (h *Host) CloseUncommittedForkRuntime(
	sessionKey string,
	client runtimectx.Client,
	logger *slog.Logger,
	forkErr error,
) {
	lease, ok := h.Runtime.CaptureClientLease(sessionKey, client)
	if !ok {
		return
	}
	closeCtx, cancel := context.WithTimeout(context.Background(), runtimectx.RoundIdleAbortTimeout)
	defer cancel()
	_, closeErr := h.Runtime.CloseSessionIfLease(closeCtx, lease)
	if closeErr != nil && !runtimectx.IsRuntimeTransportClosedError(closeErr) {
		logger.Warn("关闭未提交的 fork runtime 失败", "fork_err", forkErr, "close_err", closeErr)
	}
}
