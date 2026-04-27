package agent

import (
	"context"
	"os"
)

// EnsureReady 确保主智能体和 workspace 根目录存在。
func (s *Service) EnsureReady(ctx context.Context) error {
	s.readyMu.Lock()
	defer s.readyMu.Unlock()
	return s.ensureReady(ctx)
}

func (s *Service) ensureReady(ctx context.Context) error {
	workspaceBase := WorkspaceBasePath(s.config)
	if err := os.MkdirAll(workspaceBase, 0o755); err != nil {
		return err
	}
	ownerUserID := effectiveOwnerUserID(ctx)
	if err := os.MkdirAll(UserWorkspaceBasePath(s.config, ownerUserID), 0o755); err != nil {
		return err
	}

	agent, err := s.repository.GetMainAgent(ctx, ownerUserID)
	if err != nil {
		return err
	}
	if agent == nil {
		record := BuildDefaultMainAgentRecord(s.config, ownerUserID)
		if err = os.MkdirAll(record.WorkspacePath, 0o755); err != nil {
			return err
		}
		agent, err = s.repository.CreateAgent(ctx, record)
		if err != nil {
			return err
		}
	}
	return os.MkdirAll(agent.WorkspacePath, 0o755)
}
