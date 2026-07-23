package workspace

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) ensureAgentWorkspace(ctx context.Context, agentID string) (*protocol.Agent, error) {
	if err := EnsurePlatformSkillLibrary(); err != nil {
		return nil, err
	}
	agentValue, err := s.agents.GetAgent(ctx, strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	if err = EnsureUserSkillLibrary(agentValue.OwnerUserID); err != nil {
		return nil, err
	}
	if selected, changed, mergeErr := MergeLegacyExternalSkillReferences(
		agentValue.OwnerUserID,
		agentValue.WorkspacePath,
		agentValue.Options.SkillIDs,
	); mergeErr != nil {
		return nil, mergeErr
	} else if changed {
		options := agentValue.Options
		options.SkillIDs = selected
		agentValue, err = s.agents.UpdateAgent(ctx, agentValue.AgentID, protocol.UpdateRequest{Options: &options})
		if err != nil {
			return nil, err
		}
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
	if err = EnsureExternalSkillWorkspaceClean(agentValue.OwnerUserID, agentValue.WorkspacePath); err != nil {
		return nil, err
	}
	return agentValue, nil
}
