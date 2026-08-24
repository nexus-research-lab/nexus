// INPUT: prepare 已创建/重放的 exact proposal 与 owner/session/scope/coordinator binding access。
// OUTPUT: 单一 durable active proposal pointer、旧 sealed proposal 的显式 supersede 与 exact bound read。
// POS: Plan proposal 选择权威边界；禁止按时间猜“最新”，迟到 replay 不能改写已推进的 binding。
package orchestration

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// GetBoundPlanProposal 返回 prepare 在 exact trusted scope 中最后明确选择的 proposal。
// Proposal id 不来自调用方，且读取仍同时验证 proposal 自身的完整 access fence。
func (r *Repository) GetBoundPlanProposal(
	ctx context.Context,
	query GetBoundPlanProposalQuery,
) (*protocol.ExecutionPlanProposal, error) {
	access, err := normalizePlanProposalBindingAccess(query.Access)
	if err != nil {
		return nil, err
	}
	return r.getBoundPlanProposal(ctx, r.db, access)
}

func (r *Repository) bindPreparedPlanProposal(
	ctx context.Context,
	tx *sql.Tx,
	item protocol.ExecutionPlanProposal,
	inserted bool,
) error {
	access := planProposalBindingAccessFor(item)
	current, err := r.getBoundPlanProposal(ctx, tx, access)
	if err != nil {
		return err
	}
	if current != nil {
		if current.ID == item.ID {
			return nil
		}
		if !inserted {
			return fmt.Errorf(
				"%w: proposal %q was superseded by the current prepared proposal",
				ErrCommandConflict,
				item.ID,
			)
		}
		if current.Status == protocol.ExecutionPlanProposalStatusMaterializing {
			return fmt.Errorf(
				"%w: proposal %q is already materializing and cannot be superseded",
				ErrCommandConflict,
				current.ID,
			)
		}
		if current.Status == protocol.ExecutionPlanProposalStatusSealed &&
			samePlanProposalPhysicalRound(*current, item) {
			return fmt.Errorf(
				"%w: physical round already owns sealed proposal %q",
				ErrCommandConflict,
				current.ID,
			)
		}
	}
	if item.Status == protocol.ExecutionPlanProposalStatusDiscarded {
		return fmt.Errorf("%w: discarded proposal %q cannot become active", ErrCommandConflict, item.ID)
	}
	if err = r.discardOtherSealedPlanProposals(ctx, tx, access, item.ID); err != nil {
		return err
	}
	now := r.currentTime()
	result, err := tx.ExecContext(ctx, `
INSERT INTO execution_plan_proposal_bindings (
    owner_user_id, session_key, scope_kind, room_id, conversation_id,
    coordinator_agent_id, proposal_id,
    root_round_id, runtime_round_id, agent_round_id,
    created_at, updated_at
) VALUES (`+
		r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+
		r.bind(6)+`,`+r.bind(7)+`,`+r.bind(8)+`,`+r.bind(9)+`,`+r.bind(10)+`,`+
		r.bind(11)+`,`+r.bind(12)+`)
ON CONFLICT (owner_user_id, session_key, scope_kind, room_id, conversation_id, coordinator_agent_id)
DO UPDATE SET
    proposal_id = excluded.proposal_id,
    root_round_id = excluded.root_round_id,
    runtime_round_id = excluded.runtime_round_id,
    agent_round_id = excluded.agent_round_id,
    updated_at = excluded.updated_at
WHERE NOT EXISTS (
      SELECT 1
      FROM execution_plan_proposals AS active
      WHERE active.proposal_id = execution_plan_proposal_bindings.proposal_id
        AND (
            active.status = 'materializing'
            OR (
                active.status = 'sealed'
                AND execution_plan_proposal_bindings.root_round_id = excluded.root_round_id
                AND execution_plan_proposal_bindings.runtime_round_id = excluded.runtime_round_id
                AND execution_plan_proposal_bindings.agent_round_id = excluded.agent_round_id
            )
        )
  )`,
		access.OwnerUserID,
		access.SessionKey,
		access.ScopeKind,
		access.RoomID,
		access.ConversationID,
		access.CoordinatorAgentID,
		item.ID,
		strings.TrimSpace(item.RootRoundID),
		strings.TrimSpace(item.RuntimeRoundID),
		strings.TrimSpace(item.AgentRoundID),
		r.timestamp(now),
		r.timestamp(now),
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 1 {
		return nil
	}
	current, err = r.getBoundPlanProposal(ctx, tx, access)
	if err != nil {
		return err
	}
	if current != nil && current.ID == item.ID {
		return nil
	}
	return fmt.Errorf(
		"%w: active proposal binding cannot be replaced in the same round or during materialization",
		ErrCommandConflict,
	)
}

func samePlanProposalPhysicalRound(left, right protocol.ExecutionPlanProposal) bool {
	return strings.TrimSpace(left.RootRoundID) == strings.TrimSpace(right.RootRoundID) &&
		strings.TrimSpace(left.RuntimeRoundID) == strings.TrimSpace(right.RuntimeRoundID) &&
		strings.TrimSpace(left.AgentRoundID) == strings.TrimSpace(right.AgentRoundID)
}

func (r *Repository) discardOtherSealedPlanProposals(
	ctx context.Context,
	tx *sql.Tx,
	access PlanProposalBindingAccess,
	keepProposalID string,
) error {
	_, err := tx.ExecContext(ctx, `
UPDATE execution_plan_proposals
SET status = 'discarded', version = version + 1, updated_at = `+r.bind(1)+`
WHERE owner_user_id = `+r.bind(2)+`
  AND session_key = `+r.bind(3)+`
  AND scope_kind = `+r.bind(4)+`
  AND COALESCE(room_id, '') = `+r.bind(5)+`
  AND COALESCE(conversation_id, '') = `+r.bind(6)+`
  AND coordinator_agent_id = `+r.bind(7)+`
  AND proposal_id <> `+r.bind(8)+`
  AND status = 'sealed'`,
		r.timestamp(r.currentTime()),
		access.OwnerUserID,
		access.SessionKey,
		access.ScopeKind,
		access.RoomID,
		access.ConversationID,
		access.CoordinatorAgentID,
		strings.TrimSpace(keepProposalID),
	)
	return err
}

func (r *Repository) getBoundPlanProposal(
	ctx context.Context,
	queryer sqlQueryer,
	access PlanProposalBindingAccess,
) (*protocol.ExecutionPlanProposal, error) {
	item, err := scanPlanProposal(queryer.QueryRowContext(
		ctx,
		r.planProposalSelect()+`
WHERE proposal_id = (
    SELECT proposal_id
    FROM execution_plan_proposal_bindings
    WHERE owner_user_id = `+r.bind(1)+`
      AND session_key = `+r.bind(2)+`
      AND scope_kind = `+r.bind(3)+`
      AND room_id = `+r.bind(4)+`
      AND conversation_id = `+r.bind(5)+`
      AND coordinator_agent_id = `+r.bind(6)+`
)
  AND owner_user_id = `+r.bind(7)+`
  AND session_key = `+r.bind(8)+`
  AND scope_kind = `+r.bind(9)+`
  AND COALESCE(room_id, '') = `+r.bind(10)+`
  AND COALESCE(conversation_id, '') = `+r.bind(11)+`
  AND coordinator_agent_id = `+r.bind(12),
		access.OwnerUserID,
		access.SessionKey,
		access.ScopeKind,
		access.RoomID,
		access.ConversationID,
		access.CoordinatorAgentID,
		access.OwnerUserID,
		access.SessionKey,
		access.ScopeKind,
		access.RoomID,
		access.ConversationID,
		access.CoordinatorAgentID,
	))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func planProposalBindingAccessFor(item protocol.ExecutionPlanProposal) PlanProposalBindingAccess {
	return PlanProposalBindingAccess{
		OwnerUserID:        item.OwnerUserID,
		SessionKey:         item.SessionKey,
		ScopeKind:          item.ScopeKind,
		RoomID:             item.RoomID,
		ConversationID:     item.ConversationID,
		CoordinatorAgentID: item.CoordinatorAgentID,
	}
}
