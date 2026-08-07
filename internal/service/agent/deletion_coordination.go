// INPUT: 已鉴权 Agent、owner 作用域与持久删除回调。
// OUTPUT: 可由应用层注入的删除协调，以及“数据库已提交但外围清理失败”状态。
// POS: Agent 删除对外部能力域的消费侧接口，避免 agent 反向依赖 channels。
package agent

import (
	"context"
	"errors"
	"fmt"
)

type deletionCoordinator interface {
	CoordinateAgentDeletion(
		context.Context,
		string,
		string,
		func(context.Context) error,
	) error
}

type DeletionReconcileError struct {
	cause error
}

func (e *DeletionReconcileError) Error() string {
	return fmt.Sprintf("Agent 数据已删除，但关联运行态清理需要 reconcile: %v", e.cause)
}

func (e *DeletionReconcileError) Unwrap() error {
	return e.cause
}

func AgentDeletionCommitted(err error) bool {
	var committed *DeletionReconcileError
	return errors.As(err, &committed)
}

func (s *Service) deleteAgentPersistence(
	ctx context.Context,
	ownerUserID string,
	agentID string,
) error {
	return s.deleteAgentPersistenceAtVersion(ctx, ownerUserID, agentID, nil)
}

func (s *Service) deleteAgentPersistenceAtVersion(
	ctx context.Context,
	ownerUserID string,
	agentID string,
	expectedRuntimeVersion *int64,
) error {
	deletePersistent := func(deleteCtx context.Context) error {
		if expectedRuntimeVersion != nil {
			return s.repository.DeleteAgentAtVersion(
				deleteCtx,
				agentID,
				ownerUserID,
				*expectedRuntimeVersion,
			)
		}
		return s.repository.DeleteAgent(deleteCtx, agentID, ownerUserID)
	}
	if s.deletionCoordinator == nil {
		return deletePersistent(ctx)
	}
	called := false
	committed := false
	err := s.deletionCoordinator.CoordinateAgentDeletion(
		ctx,
		ownerUserID,
		agentID,
		func(deleteCtx context.Context) error {
			if called {
				return errors.New("Agent 持久删除回调不能重复执行")
			}
			called = true
			deleteErr := deletePersistent(deleteCtx)
			if deleteErr == nil {
				committed = true
			}
			return deleteErr
		},
	)
	if !called && err == nil {
		return errors.New("Agent 删除协调器未执行持久删除")
	}
	if err != nil && committed {
		return &DeletionReconcileError{cause: err}
	}
	return err
}
