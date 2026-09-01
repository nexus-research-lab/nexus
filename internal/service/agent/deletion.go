// INPUT: 待删除的 Agent、全部 Session 快照与可选 runtime_version CAS。
// OUTPUT: 已先撤销持久身份，再清理 Goal、Task、runtime、transcript 与 workspace 的 Agent。
// POS: Agent 领域的完整删除与提交后 reconcile 边界。
package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) applyAgentDeletion(
	ctx context.Context,
	agentValue protocol.Agent,
	sessions []protocol.Session,
	expectedRuntimeVersion *int64,
) error {
	deleteOwnerUserID := agentValue.OwnerUserID
	persistenceErr := s.deleteAgentPersistenceAtVersion(
		ctx,
		deleteOwnerUserID,
		agentValue.AgentID,
		expectedRuntimeVersion,
	)
	if persistenceErr != nil && !AgentDeletionCommitted(persistenceErr) {
		return persistenceErr
	}

	cleanupCtx := context.WithoutCancel(ctx)
	cleanupErrs := make([]error, 0, 6)
	if persistenceErr != nil {
		var committed *DeletionReconcileError
		if errors.As(persistenceErr, &committed) {
			cleanupErrs = append(cleanupErrs, committed.cause)
		} else {
			cleanupErrs = append(cleanupErrs, persistenceErr)
		}
	}
	// Task 记录先移除，isolated Session 文件由下面的 Session 快照统一清理。
	// 这样即使 Agent 身份已经撤销，也不需要再次读取 Agent workspace。
	if s.tasks != nil {
		if err := s.tasks.DeleteTasksForAgent(cleanupCtx, agentValue.OwnerUserID, agentValue.AgentID); err != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("清理 Agent Task: %w", err))
		}
	}
	if s.sessions != nil {
		if err := s.sessions.DeleteAgentSessionArtifacts(cleanupCtx, agentValue, sessions); err != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("清理 Agent Session: %w", err))
		}
	}
	if s.goals != nil {
		if _, err := s.goals.DeleteGoalsForAgent(cleanupCtx, agentValue.AgentID); err != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("清理 Agent Goal: %w", err))
		}
	}
	if s.history != nil {
		if _, err := s.history.ForOwner(agentValue.OwnerUserID).DeleteTranscriptProject(
			agentValue.WorkspacePath,
		); err != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("清理 Agent transcript: %w", err))
		}
	}
	if err := s.cleanupAgentWorkspace(cleanupCtx, agentValue); err != nil {
		cleanupErrs = append(cleanupErrs, fmt.Errorf("清理 Agent workspace: %w", err))
	}
	if cleanupErr := errors.Join(cleanupErrs...); cleanupErr != nil {
		return &DeletionReconcileError{cause: cleanupErr}
	}
	return nil
}
