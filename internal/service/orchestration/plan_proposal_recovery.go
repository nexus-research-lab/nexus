// INPUT: 已到 retry deadline 的 materializing proposal、materialized+pending Goal receipt 或被竞态阻塞后出现的 exact command receipt。
// OUTPUT: 有界重放、blocked receipt 收敛、confirmation retry 与可观测 recovery counters。
// POS: ExecutionPlanProposal saga 的进程重启恢复入口；不依赖模型再次提交文档。
package orchestration

import (
	"context"
	"errors"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

// PlanProposalRecoveryResult 汇总一次有界 recovery scan。
type PlanProposalRecoveryResult struct {
	Scanned      int
	Materialized int
	Confirmed    int
	Blocked      int
	Failed       int
}

// ReconcilePlanProposals 重放权威 command 或重试 Goal confirmation。
// Proposal 内保存的 identity 只用于 trusted background worker；所有模型调用仍须
// 通过 owner/session/scope/coordinator access fence。
func (s *Service) ReconcilePlanProposals(
	ctx context.Context,
	limit int,
) (PlanProposalRecoveryResult, error) {
	result := PlanProposalRecoveryResult{}
	if s == nil || s.planProposals == nil {
		return result, errors.New("execution plan proposal repository is unavailable")
	}
	if limit <= 0 {
		return result, errors.New("positive plan proposal recovery limit is required")
	}
	items, err := s.planProposals.ListRecoverablePlanProposals(
		ctx,
		orchestrationstore.ListRecoverablePlanProposalsQuery{
			Now:   s.currentTime(),
			Limit: limit,
		},
	)
	if err != nil {
		return result, err
	}
	result.Scanned = len(items)
	var itemErrors []error
	for index := range items {
		proposal := items[index]
		beforeStatus := proposal.Status
		beforeConfirmation := proposal.ConfirmationState
		mutation, reconcileErr := s.materializeLoadedPlanProposal(
			ctx,
			proposalActor(&proposal),
			&proposal,
		)
		if reconcileErr != nil {
			result.Failed++
			itemErrors = append(itemErrors, fmt.Errorf(
				"reconcile plan proposal %s: %w",
				proposal.ID,
				reconcileErr,
			))
			continue
		}
		if mutation.Outcome == MutationRejected {
			result.Blocked++
			continue
		}
		reloaded, reloadErr := s.planProposals.GetPlanProposal(
			ctx,
			orchestrationstore.GetPlanProposalQuery{
				Access: orchestrationstore.PlanProposalAccess{
					ProposalID:         proposal.ID,
					OwnerUserID:        proposal.OwnerUserID,
					SessionKey:         proposal.SessionKey,
					ScopeKind:          proposal.ScopeKind,
					RoomID:             proposal.RoomID,
					ConversationID:     proposal.ConversationID,
					CoordinatorAgentID: proposal.CoordinatorAgentID,
				},
			},
		)
		if reloadErr != nil || reloaded == nil {
			result.Failed++
			if reloadErr == nil {
				reloadErr = errors.New("proposal disappeared after recovery")
			}
			itemErrors = append(itemErrors, fmt.Errorf(
				"reload plan proposal %s: %w",
				proposal.ID,
				reloadErr,
			))
			continue
		}
		if (beforeStatus == protocol.ExecutionPlanProposalStatusMaterializing ||
			beforeStatus == protocol.ExecutionPlanProposalStatusBlocked) &&
			reloaded.Status == protocol.ExecutionPlanProposalStatusMaterialized {
			result.Materialized++
		}
		if beforeConfirmation == protocol.ExecutionPlanProposalConfirmationPending &&
			reloaded.ConfirmationState == protocol.ExecutionPlanProposalConfirmationConfirmed {
			result.Confirmed++
		}
	}
	return result, errors.Join(itemErrors...)
}
