// INPUT: Channel 删除协调器、runtime Agent 撤销器与持久删除回调。
// OUTPUT: 数据库提交点后立即撤销 Agent runtime，并汇总 Channel/runtime 后置清理错误。
// POS: app 装配层连接 Agent、Channels 与 runtime 的跨域删除事务协调边界。
package server

import (
	"context"
	"errors"
	"fmt"
)

type agentChannelDeletionCoordinator interface {
	CoordinateAgentDeletion(
		context.Context,
		string,
		string,
		func(context.Context) error,
	) error
}

type agentRuntimeRevoker interface {
	RevokeAgentSessions(context.Context, string, string) (int, error)
}

type agentDeletionCoordinator struct {
	channels agentChannelDeletionCoordinator
	runtimes agentRuntimeRevoker
}

func newAgentDeletionCoordinator(
	channels agentChannelDeletionCoordinator,
	runtimes agentRuntimeRevoker,
) agentDeletionCoordinator {
	return agentDeletionCoordinator{channels: channels, runtimes: runtimes}
}

func (c agentDeletionCoordinator) CoordinateAgentDeletion(
	ctx context.Context,
	ownerUserID string,
	agentID string,
	deletePersistent func(context.Context) error,
) error {
	if deletePersistent == nil {
		return errors.New("Agent 持久删除回调不能为空")
	}
	committed := false
	var runtimeErr error
	deleteAndRevoke := func(deleteCtx context.Context) error {
		if err := deletePersistent(deleteCtx); err != nil {
			return err
		}
		committed = true
		if c.runtimes != nil {
			_, runtimeErr = c.runtimes.RevokeAgentSessions(
				context.WithoutCancel(deleteCtx),
				ownerUserID,
				agentID,
			)
			if runtimeErr != nil {
				runtimeErr = fmt.Errorf("撤销 Agent runtime: %w", runtimeErr)
			}
		}
		// runtime 后置失败不能阻止 Channel 清理；两类错误在协调完成后统一
		// 返回，Agent service 会将其标记为数据库已提交的 reconcile 状态。
		return nil
	}

	var channelErr error
	if c.channels == nil {
		channelErr = deleteAndRevoke(ctx)
	} else {
		channelErr = c.channels.CoordinateAgentDeletion(
			ctx,
			ownerUserID,
			agentID,
			deleteAndRevoke,
		)
	}
	if !committed {
		return channelErr
	}
	return errors.Join(channelErr, runtimeErr)
}
