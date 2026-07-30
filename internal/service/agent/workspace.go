package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

const maxAgentIDWorkspaceAttempts = 8

func (s *Service) createAgentWorkspacePath(ownerUserID string) (string, string, error) {
	workspaceBase := WorkspaceBasePath(s.config)
	if err := os.MkdirAll(workspaceBase, agentWorkspaceDirectoryMode()); err != nil {
		return "", "", err
	}
	root, err := confinedfs.Open(workspaceBase)
	if err != nil {
		return "", "", err
	}
	defer root.Close()
	ownerRelative, err := filepath.Rel(
		workspaceBase,
		UserWorkspaceBasePath(s.config, ownerUserID),
	)
	if err != nil {
		return "", "", err
	}
	ownerRoot, err := root.OpenOrCreateRootNoSymlink(
		filepath.ToSlash(ownerRelative),
		agentWorkspaceDirectoryMode(),
	)
	if err != nil {
		return "", "", err
	}
	defer ownerRoot.Close()
	for range maxAgentIDWorkspaceAttempts {
		agentID := NewAgentID()
		workspaceName := BuildWorkspaceDirName(agentID)
		if err := ownerRoot.Mkdir(workspaceName, agentWorkspaceDirectoryMode()); err == nil {
			workspacePath := filepath.Join(
				UserWorkspaceBasePath(s.config, ownerUserID),
				workspaceName,
			)
			return agentID, workspacePath, nil
		} else {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return "", "", err
		}
	}
	return "", "", fmt.Errorf("无法生成可用的 agent 工作区目录")
}

func (s *Service) openAgentWorkspace(
	agentValue protocol.Agent,
	create bool,
) (*confinedfs.Root, error) {
	return workspacestore.New(s.config.WorkspacePath).OpenOwnerWorkspacePath(
		agentValue.OwnerUserID,
		agentValue.WorkspacePath,
		create,
	)
}

func (s *Service) removeAgentWorkspace(agentValue protocol.Agent) error {
	return workspacestore.New(s.config.WorkspacePath).RemoveOwnerWorkspacePath(
		agentValue.OwnerUserID,
		agentValue.WorkspacePath,
	)
}

func (s *Service) ensureAgentRuntimeState(agentValue protocol.Agent) error {
	root, err := s.openAgentWorkspace(agentValue, false)
	if err != nil {
		return err
	}
	defer root.Close()
	if err = ensureRuntimeEmotionStateAt(root); err != nil {
		return err
	}
	return ensureRuntimeSettingsProjectionAt(root, agentValue)
}
