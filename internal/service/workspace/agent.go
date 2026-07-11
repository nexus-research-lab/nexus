package workspace

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) ensureAgentWorkspace(ctx context.Context, agentID string) (*protocol.Agent, error) {
	agentValue, err := s.agents.GetAgent(ctx, strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	if err = EnsureInitialized(
		agentValue.AgentID,
		agentValue.Name,
		agentValue.WorkspacePath,
		agentValue.IsMain,
		agentValue.CreatedAt,
	); err != nil {
		return nil, err
	}
	return agentValue, nil
}
