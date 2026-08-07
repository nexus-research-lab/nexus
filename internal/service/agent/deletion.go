// INPUT: 删除前固化的 Agent 与全部 Session 快照。
// OUTPUT: 跨 Goal、Task、runtime、transcript、workspace 与 SQL 的幂等 Agent 删除。
// POS: Agent 领域的持久删除恢复边界。
package agent

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	deletionsvc "github.com/nexus-research-lab/nexus/internal/service/deletion"
)

type agentDeletionPayload struct {
	Agent    protocol.Agent     `json:"agent"`
	Sessions []protocol.Session `json:"sessions"`
}

func bindAgentDeletionOwner(payload *agentDeletionPayload, ownerUserID string) {
	if payload != nil {
		payload.Agent.OwnerUserID = strings.TrimSpace(ownerUserID)
	}
}

func (s *Service) applyAgentDeletion(
	ctx context.Context,
	job deletionsvc.Job,
	payload agentDeletionPayload,
) error {
	fail := func(err error) error {
		if s.deletion == nil || job.ID == "" {
			return err
		}
		return s.deletion.Fail(ctx, job, err)
	}
	agentValue := payload.Agent
	if s.sessions != nil {
		if err := s.sessions.DeleteAgentSessionArtifacts(ctx, agentValue, payload.Sessions); err != nil {
			return fail(err)
		}
	}
	if s.tasks != nil {
		if err := s.tasks.DeleteTasksForAgent(ctx, agentValue.OwnerUserID, agentValue.AgentID); err != nil {
			return fail(err)
		}
	}
	if s.goals != nil {
		if _, err := s.goals.DeleteGoalsForAgent(ctx, agentValue.AgentID); err != nil {
			return fail(err)
		}
	}
	if err := s.repository.DeleteAgent(ctx, agentValue.AgentID, agentValue.OwnerUserID); err != nil {
		return fail(err)
	}
	if s.history != nil {
		if _, err := s.history.ForOwner(agentValue.OwnerUserID).DeleteTranscriptProject(
			agentValue.WorkspacePath,
		); err != nil {
			return fail(err)
		}
	}
	if err := s.cleanupAgentWorkspace(ctx, agentValue); err != nil {
		return fail(err)
	}
	if s.deletion != nil && job.ID != "" {
		if err := s.deletion.Complete(ctx, job); err != nil {
			return s.deletion.Fail(ctx, job, err)
		}
	}
	return nil
}

// ReconcilePendingDeletions 重放尚未完成的 Agent 删除任务。
func (s *Service) ReconcilePendingDeletions(ctx context.Context) error {
	if s == nil || s.deletion == nil {
		return nil
	}
	jobs, err := s.deletion.ListPending(ctx, deletionsvc.KindAgent)
	if err != nil {
		return err
	}
	errList := make([]error, 0)
	for _, job := range jobs {
		var payload agentDeletionPayload
		if decodeErr := deletionsvc.DecodePayload(job, &payload); decodeErr != nil {
			errList = append(errList, s.deletion.Fail(ctx, job, decodeErr))
			continue
		}
		bindAgentDeletionOwner(&payload, job.OwnerUserID)
		ownerUserID := strings.TrimSpace(job.OwnerUserID)
		ownerCtx := authctx.WithPrincipal(ctx, &authctx.Principal{
			UserID: ownerUserID,
			Role:   authctx.RoleOwner,
		})
		if applyErr := s.applyAgentDeletion(ownerCtx, job, payload); applyErr != nil {
			errList = append(errList, applyErr)
		}
	}
	return errors.Join(errList...)
}
