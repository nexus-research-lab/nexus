// INPUT: 外部 Skill 名称与受限库路径。
// OUTPUT: 无 Agent 引用、无 registry 记录、无文件目录的 Skill 删除。
// POS: 外部 Skill 的完整删除边界。
package skills

import (
	"context"
	"errors"
	"os"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	workspacesvc "github.com/nexus-research-lab/nexus/internal/service/workspace"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *Service) applySkillDeletion(
	ctx context.Context,
	name string,
	sourcePath string,
) error {
	if s.agents != nil {
		agents, err := s.agents.ListAgentRecords(ctx)
		if err != nil {
			return err
		}
		for index := range agents {
			agentValue := agents[index]
			selected, selectedChanged := removeSkillReferences(
				agentValue.Options.SkillIDs,
				name,
			)
			disabled, disabledChanged := removeSkillReferences(
				agentValue.Options.DisabledSkillIDs,
				name,
			)
			if !selectedChanged && !disabledChanged {
				continue
			}
			if _, err = s.agents.UpdateAgentSkillSelection(
				ctx,
				agentValue.AgentID,
				selected,
				disabled,
			); err != nil {
				return err
			}
		}
	}
	ownerUserID := authctx.OwnerUserID(ctx)
	boundaryFS, err := workspacestore.New(s.config.WorkspacePath).OpenOwnerWorkspacePath(
		ownerUserID,
		workspacesvc.UserSkillLibraryRoot(s.config, ownerUserID),
		false,
	)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if boundaryFS != nil {
		relativePath, pathErr := relativeSkillPath(boundaryFS, sourcePath)
		if pathErr == nil {
			pathErr = boundaryFS.RemoveAll(relativePath)
		}
		if err = errors.Join(pathErr, boundaryFS.Close()); err != nil {
			return err
		}
	}
	if err = workspacesvc.RefreshUserSkillLibrary(s.config, ownerUserID); err != nil {
		return err
	}
	if s.skillStore != nil {
		return s.skillStore.DeleteImportedSkill(ctx, ownerUserID, name)
	}
	return nil
}
