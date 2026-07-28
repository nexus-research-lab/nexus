package agent

import (
	"context"
	"os"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// EnsureReady 确保主智能体和 workspace 根目录存在。
func (s *Service) EnsureReady(ctx context.Context) error {
	s.readyMu.Lock()
	defer s.readyMu.Unlock()
	return s.ensureReady(ctx)
}

func (s *Service) ensureReady(ctx context.Context) error {
	workspaceBase := WorkspaceBasePath(s.config)
	if err := os.MkdirAll(workspaceBase, 0o700); err != nil {
		return err
	}
	ownerUserID := effectiveOwnerUserID(ctx)
	if err := appfs.EnsureUserRuntimeLayout(ownerUserID); err != nil {
		return err
	}
	if err := ensureDirectoryWithinRoot(
		workspaceBase,
		UserWorkspaceBasePath(s.config, ownerUserID),
		agentWorkspaceDirectoryMode(),
	); err != nil {
		return err
	}

	agent, err := s.repository.GetMainAgent(ctx, ownerUserID)
	if err != nil {
		return err
	}
	if agent == nil {
		record := BuildDefaultMainAgentRecord(s.config, ownerUserID)
		if err = ensureDirectoryWithinRoot(
			workspaceBase,
			record.WorkspacePath,
			agentWorkspaceDirectoryMode(),
		); err != nil {
			return err
		}
		recordRoot, openErr := s.openAgentWorkspace(protocol.Agent{
			AgentID:       record.AgentID,
			OwnerUserID:   record.OwnerUserID,
			WorkspacePath: record.WorkspacePath,
		}, false)
		if openErr != nil {
			return openErr
		}
		err = ensureRuntimeEmotionStateAt(recordRoot)
		closeErr := recordRoot.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
		agent, err = s.repository.CreateAgent(ctx, record)
		if err != nil {
			return err
		}
	}
	if err = ensureDirectoryWithinRoot(
		workspaceBase,
		agent.WorkspacePath,
		agentWorkspaceDirectoryMode(),
	); err != nil {
		return err
	}
	return s.ensureAgentRuntimeState(*agent)
}
