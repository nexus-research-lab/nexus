// INPUT: 删除前固化的外部 Skill 名称与受限库路径。
// OUTPUT: 无 Agent 引用、无 registry 记录、无文件目录的幂等 Skill 删除。
// POS: 外部 Skill 跨数据库与文件系统的持久删除恢复边界。
package skills

import (
	"context"
	"errors"
	"os"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	deletionsvc "github.com/nexus-research-lab/nexus/internal/service/deletion"
	workspacesvc "github.com/nexus-research-lab/nexus/internal/service/workspace"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type skillDeletionPayload struct {
	Name       string `json:"name"`
	SourcePath string `json:"source_path"`
}

func (s *Service) applySkillDeletion(
	ctx context.Context,
	job deletionsvc.Job,
	payload skillDeletionPayload,
) error {
	fail := func(err error) error {
		if s.deletion == nil || job.ID == "" {
			return err
		}
		return s.deletion.Fail(ctx, job, err)
	}
	if s.agents != nil {
		agents, err := s.agents.ListAgentRecords(ctx)
		if err != nil {
			return fail(err)
		}
		for index := range agents {
			agentValue := agents[index]
			selected, selectedChanged := removeSkillReferences(
				agentValue.Options.SkillIDs,
				payload.Name,
			)
			disabled, disabledChanged := removeSkillReferences(
				agentValue.Options.DisabledSkillIDs,
				payload.Name,
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
				return fail(err)
			}
		}
	}
	ownerUserID := authctx.OwnerUserID(ctx)
	if s.skillStore != nil {
		if err := s.skillStore.DeleteImportedSkill(ctx, ownerUserID, payload.Name); err != nil {
			return fail(err)
		}
	}
	boundaryFS, err := workspacestore.New(s.config.WorkspacePath).OpenOwnerWorkspacePath(
		ownerUserID,
		workspacesvc.UserSkillLibraryRoot(s.config, ownerUserID),
		false,
	)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fail(err)
	}
	if boundaryFS != nil {
		relativePath, pathErr := relativeSkillPath(boundaryFS, payload.SourcePath)
		if pathErr == nil {
			pathErr = boundaryFS.RemoveAll(relativePath)
		}
		if err = errors.Join(pathErr, boundaryFS.Close()); err != nil {
			return fail(err)
		}
	}
	if err = workspacesvc.RefreshUserSkillLibrary(s.config, ownerUserID); err != nil {
		return fail(err)
	}
	if s.deletion != nil && job.ID != "" {
		if err = s.deletion.Complete(ctx, job); err != nil {
			return s.deletion.Fail(ctx, job, err)
		}
	}
	return nil
}

// ReconcilePendingDeletions 重放尚未完成的外部 Skill 删除任务。
func (s *Service) ReconcilePendingDeletions(ctx context.Context) error {
	if s == nil || s.deletion == nil {
		return nil
	}
	jobs, err := s.deletion.ListPending(ctx, deletionsvc.KindSkill)
	if err != nil {
		return err
	}
	errList := make([]error, 0)
	for _, job := range jobs {
		var payload skillDeletionPayload
		if decodeErr := deletionsvc.DecodePayload(job, &payload); decodeErr != nil {
			errList = append(errList, s.deletion.Fail(ctx, job, decodeErr))
			continue
		}
		ownerCtx := authctx.WithPrincipal(ctx, &authctx.Principal{
			UserID: job.OwnerUserID,
			Role:   authctx.RoleOwner,
		})
		if applyErr := s.applySkillDeletion(ownerCtx, job, payload); applyErr != nil {
			errList = append(errList, applyErr)
		}
	}
	return errors.Join(errList...)
}
