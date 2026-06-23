package room

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
)

func (s *Service) resolveAgentWorkspacePath(ctx context.Context, agentID string) (string, error) {
	agentValue, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		return "", err
	}
	if workspacePath := strings.TrimSpace(agentValue.WorkspacePath); workspacePath != "" {
		return workspacePath, nil
	}
	return agentsvc.ResolveWorkspacePath(s.config, agentValue.OwnerUserID, agentValue.AgentID), nil
}

func (s *Service) normalizeDirectAgentIDs(ctx context.Context, agentIDs []string) ([]string, error) {
	normalizedIDs := make([]string, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		agentValue, err := s.resolveRoomAgent(ctx, agentID)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(normalizedIDs, agentValue.AgentID) {
			normalizedIDs = append(normalizedIDs, agentValue.AgentID)
		}
	}
	if len(normalizedIDs) == 0 {
		return nil, errors.New("DM room 需要一个 agent 成员")
	}
	if len(normalizedIDs) > 1 {
		return nil, errors.New("DM room 仅支持一个 agent 成员")
	}
	return normalizedIDs, nil
}

func (s *Service) normalizeGroupAgentIDs(ctx context.Context, agentIDs []string) ([]string, error) {
	if len(agentIDs) == 0 {
		return nil, errors.New("room 至少需要一个普通成员 agent，主智能体不能作为 room 成员")
	}
	// Deduplicate and trim input IDs before batch fetch.
	seen := make(map[string]struct{}, len(agentIDs))
	cleaned := make([]string, 0, len(agentIDs))
	for _, id := range agentIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, errors.New("agent_id 不能为空")
		}
		if _, dup := seen[id]; !dup {
			seen[id] = struct{}{}
			cleaned = append(cleaned, id)
		}
	}
	fetched, err := s.agents.GetAgentsByIDs(ctx, cleaned)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]protocol.Agent, len(fetched))
	for _, a := range fetched {
		byID[a.AgentID] = a
	}
	normalizedIDs := make([]string, 0, len(cleaned))
	for _, id := range cleaned {
		agentValue, ok := byID[id]
		if !ok || agentValue.Status != "active" {
			return nil, fmt.Errorf("%w: %s", agentsvc.ErrAgentNotFound, id)
		}
		if agentValue.IsMain {
			return nil, fmt.Errorf("主智能体（%s）不能作为 room 成员", agentValue.Name)
		}
		normalizedIDs = append(normalizedIDs, agentValue.AgentID)
	}
	if len(normalizedIDs) == 0 {
		return nil, errors.New("room 至少需要一个普通成员 agent，主智能体不能作为 room 成员")
	}
	return normalizedIDs, nil
}

func (s *Service) loadAgentRefs(ctx context.Context, agentIDs []string) ([]roomrepo.AgentRuntimeRef, error) {
	refs, err := s.repository.LoadAgentRuntimeRefs(ctx, authctx.OwnerUserID(ctx), agentIDs)
	if err != nil {
		return nil, err
	}
	refByID := make(map[string]roomrepo.AgentRuntimeRef, len(refs))
	for _, ref := range refs {
		refByID[ref.AgentID] = ref
	}

	result := make([]roomrepo.AgentRuntimeRef, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		ref, ok := refByID[agentID]
		if !ok || ref.Status != "active" || strings.TrimSpace(ref.RuntimeID) == "" {
			return nil, fmt.Errorf("%w: %s", agentsvc.ErrAgentNotFound, agentID)
		}
		result = append(result, ref)
	}
	return result, nil
}

func (s *Service) resolveRoomAgent(ctx context.Context, agentID string) (*protocol.Agent, error) {
	cleaned := strings.TrimSpace(agentID)
	if cleaned == "" {
		return nil, errors.New("agent_id 不能为空")
	}
	if cleaned == strings.TrimSpace(s.config.DefaultAgentID) {
		agentValue, err := s.agents.GetDefaultAgent(ctx)
		if err == nil {
			return agentValue, nil
		}
		if !errors.Is(err, agentsvc.ErrAgentNotFound) {
			return nil, err
		}
	}
	return s.agents.GetAgent(ctx, cleaned)
}

func (s *Service) ensureGroupMemberAgent(ctx context.Context, agentID string) (*protocol.Agent, error) {
	agentValue, err := s.resolveRoomAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	if agentValue.IsMain {
		return nil, fmt.Errorf("主智能体（%s）不能作为 room 成员", agentValue.Name)
	}
	return agentValue, nil
}
