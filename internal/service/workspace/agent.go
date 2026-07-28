package workspace

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *Service) ensureAgentWorkspace(ctx context.Context, agentID string) (*protocol.Agent, error) {
	if err := EnsurePlatformSkillLibrary(); err != nil {
		return nil, err
	}
	agentValue, err := s.agents.GetAgent(ctx, strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	if err = EnsureUserSkillLibrary(s.config, agentValue.OwnerUserID); err != nil {
		return nil, err
	}
	root, err := s.openAgentWorkspace(agentValue, true)
	if err != nil {
		return nil, err
	}
	if err = EnsureInitializedAt(
		root,
		agentValue.AgentID,
		agentValue.Name,
		agentValue.IsMain,
		agentValue.CreatedAt,
	); err != nil {
		_ = root.Close()
		return nil, err
	}
	if err = root.Close(); err != nil {
		return nil, err
	}
	return agentValue, nil
}

// EnsureAgentWorkspace 校验 owner 归属并完成 Agent workspace 初始化。
func (s *Service) EnsureAgentWorkspace(
	ctx context.Context,
	agentID string,
) (*protocol.Agent, error) {
	return s.ensureAgentWorkspace(ctx, agentID)
}

func (s *Service) openAgentWorkspace(
	agentValue *protocol.Agent,
	create bool,
) (*confinedfs.Root, error) {
	if s == nil || agentValue == nil {
		return nil, errors.New("agent workspace is unavailable")
	}
	return workspacestore.New(s.config.WorkspacePath).OpenOwnerWorkspacePath(
		agentValue.OwnerUserID,
		agentValue.WorkspacePath,
		create,
	)
}

// EnsureInitializedForAgent 在 owner 绑定的 workspace fd 内完成初始化。
func EnsureInitializedForAgent(cfg config.Config, agentValue protocol.Agent) error {
	root, err := workspacestore.New(cfg.WorkspacePath).OpenOwnerWorkspacePath(
		agentValue.OwnerUserID,
		agentValue.WorkspacePath,
		true,
	)
	if err != nil {
		return err
	}
	defer root.Close()
	return EnsureInitializedAt(
		root,
		agentValue.AgentID,
		agentValue.Name,
		agentValue.IsMain,
		agentValue.CreatedAt,
	)
}
