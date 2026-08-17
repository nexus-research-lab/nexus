package session

import (
	"context"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
)

// listWorkspaceSessions 只投影当前运行态，目录查询不得为展示状态写回 meta。
func (s *Service) listWorkspaceSessions(ctx context.Context) ([]protocol.Session, error) {
	workspacePaths, err := s.resolveWorkspacePaths(ctx, "")
	if err != nil {
		return nil, err
	}
	result := make([]protocol.Session, 0)
	for _, workspacePath := range workspacePaths {
		items, listErr := s.ownerFiles(ctx).ListSessions(workspacePath)
		if listErr != nil {
			return nil, listErr
		}
		for _, item := range items {
			projected := s.applyRuntimeStateToSession(item)
			if protocol.IsRoomSharedSessionKey(projected.SessionKey) {
				continue
			}
			result = append(result, projected)
		}
	}
	slices.SortFunc(result, func(left protocol.Session, right protocol.Session) int {
		return right.LastActivity.Compare(left.LastActivity)
	})
	return result, nil
}

// listMutableWorkspaceSessions 是配置 inspect 的纯读投影。它可以根据 Manager
// 展示实时 active/closed 状态，但绝不为了“修复”展示状态而写回 meta 或推进版本。
func (s *Service) listMutableWorkspaceSessions(ctx context.Context) ([]protocol.Session, error) {
	workspacePaths, err := s.resolveWorkspacePaths(ctx, "")
	if err != nil {
		return nil, err
	}
	result := make([]protocol.Session, 0)
	for _, workspacePath := range workspacePaths {
		items, listErr := s.ownerFiles(ctx).ListSessions(workspacePath)
		if listErr != nil {
			return nil, listErr
		}
		for _, item := range items {
			projected := s.applyRuntimeStateToSession(item)
			if shouldHideWorkspaceSession(projected) {
				continue
			}
			result = append(result, projected)
		}
	}
	slices.SortFunc(result, func(left protocol.Session, right protocol.Session) int {
		return right.LastActivity.Compare(left.LastActivity)
	})
	return result, nil
}

func (s *Service) listAgents(ctx context.Context, agentID string) ([]*protocol.Agent, error) {
	if strings.TrimSpace(agentID) != "" {
		agentValue, err := s.agentService.GetAgent(ctx, agentID)
		if err != nil {
			return nil, err
		}
		return []*protocol.Agent{agentValue}, nil
	}

	items, err := s.agentService.ListAgentRecords(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*protocol.Agent, 0, len(items))
	for index := range items {
		item := items[index]
		copyItem := item
		result = append(result, &copyItem)
	}
	return result, nil
}

func (s *Service) resolveWorkspacePaths(ctx context.Context, agentID string) ([]string, error) {
	agents, err := s.listAgents(ctx, agentID)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(agents))
	seen := make(map[string]struct{}, len(agents))
	for _, agentValue := range agents {
		workspacePath := strings.TrimSpace(agentValue.WorkspacePath)
		if workspacePath == "" {
			workspacePath = agentsvc.ResolveWorkspacePath(s.config, agentValue.OwnerUserID, agentValue.AgentID)
		}
		if _, exists := seen[workspacePath]; exists {
			continue
		}
		seen[workspacePath] = struct{}{}
		result = append(result, workspacePath)
	}
	return result, nil
}
