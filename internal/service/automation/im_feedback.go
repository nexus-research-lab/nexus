// INPUT: Exact producer job/run/session of an IM delivery.
// OUTPUT: Fresh task tool restrictions for a new feedback turn, or rejection.
// POS: Automation feedback admission; never resumes or mutates the old run.
package automation

import (
	"context"
	"errors"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"slices"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/imdelivery"
)

func (s *Service) IMFeedbackPolicy(ctx context.Context, source imdelivery.Source) (*protocol.RuntimeToolPolicy, error) {
	owner := authctx.OwnerUserID(ctx)
	job, err := s.repository.GetScheduledTask(ctx, owner, source.JobID)
	if err != nil {
		return nil, err
	}
	if job == nil || job.AgentID != source.AgentID || job.DeletionState != "" || job.SessionBindingState == automationdomain.TaskSessionBindingStateRebindRequired || slices.Contains(job.InvalidatedSessionKeys, source.SessionKey) {
		return nil, errors.New("自动化来源已不可接收反馈")
	}
	run, err := s.repository.GetRun(ctx, owner, source.JobID, source.RunID)
	if err != nil {
		return nil, err
	}
	if run == nil || run.SessionKey != source.SessionKey || run.RoundID != source.RoundID || run.PermissionPolicyRevision != job.PermissionPolicy.Revision {
		return nil, errors.New("自动化来源或权限版本已改变")
	}
	policy := taskRuntimeToolPolicy(*job)
	if policy == nil {
		return nil, errors.New("自动化来源缺少可验证的工具权限快照")
	}
	return policy, nil
}
