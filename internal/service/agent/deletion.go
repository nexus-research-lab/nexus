// INPUT: 待删除的 Agent 与全部 Session 快照。
// OUTPUT: 已清理 Goal、Task、runtime、transcript、workspace 与 SQL 的 Agent。
// POS: Agent 领域的完整删除边界。
package agent

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) applyAgentDeletion(
	ctx context.Context,
	agentValue protocol.Agent,
	sessions []protocol.Session,
) error {
	if s.sessions != nil {
		if err := s.sessions.DeleteAgentSessionArtifacts(ctx, agentValue, sessions); err != nil {
			return err
		}
	}
	if s.tasks != nil {
		if err := s.tasks.DeleteTasksForAgent(ctx, agentValue.OwnerUserID, agentValue.AgentID); err != nil {
			return err
		}
	}
	if s.goals != nil {
		if _, err := s.goals.DeleteGoalsForAgent(ctx, agentValue.AgentID); err != nil {
			return err
		}
	}
	if s.history != nil {
		if _, err := s.history.ForOwner(agentValue.OwnerUserID).DeleteTranscriptProject(
			agentValue.WorkspacePath,
		); err != nil {
			return err
		}
	}
	if err := s.cleanupAgentWorkspace(ctx, agentValue); err != nil {
		return err
	}
	return s.repository.DeleteAgent(ctx, agentValue.AgentID, agentValue.OwnerUserID)
}
