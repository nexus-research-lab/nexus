// INPUT: deterministic proposal ID 或 durable binding、exact owner/session/scope/coordinator access、recovery deadline 与 command receipt。
// OUTPUT: typed canonical ExecutionPlanProposal 或有界 recoverable/persisted-receipt proposal 集合。
// POS: 非权威 Plan proposal aggregate 的 SQL read/scan 边界；active pointer 读取见 plan_proposal_binding.go。
package orchestration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// FindPlanMaterializationReceipt returns the exact Plan activated by the stable
// proposal materialization command. A semantically equal current graph is not
// a receipt for a different command.
func (r *Repository) FindPlanMaterializationReceipt(
	ctx context.Context,
	executionID string,
	materializationCommandID string,
) (string, error) {
	executionID = strings.TrimSpace(executionID)
	materializationCommandID = strings.TrimSpace(materializationCommandID)
	if executionID == "" || materializationCommandID == "" {
		return "", fmt.Errorf("%w: materialization receipt identity is required", ErrInvariant)
	}
	event, err := r.findEventByCommand(
		ctx,
		r.db,
		executionID,
		materializationCommandID+":plan",
	)
	if err != nil || event == nil {
		return "", err
	}
	if event.Type != protocol.ExecutionEventPlanActivated ||
		event.EntityType != protocol.ExecutionEntityPlan ||
		strings.TrimSpace(event.PlanID) == "" {
		return "", fmt.Errorf(
			"%w: materialization command has a non-Plan receipt",
			ErrCommandConflict,
		)
	}
	return strings.TrimSpace(event.PlanID), nil
}

// GetPlanProposal 按 exact access fence 读取 proposal；ID 存在但权限不匹配时
// 返回 ErrPlanProposalAccess，不把 proposal 当作 bearer capability。
func (r *Repository) GetPlanProposal(
	ctx context.Context,
	query GetPlanProposalQuery,
) (*protocol.ExecutionPlanProposal, error) {
	access, err := normalizePlanProposalAccess(query.Access)
	if err != nil {
		return nil, err
	}
	item, err := r.getPlanProposalScoped(ctx, r.db, access)
	if err != nil || item != nil {
		return item, err
	}
	existing, err := r.getPlanProposal(ctx, r.db, access.ProposalID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrPlanProposalAccess
	}
	return nil, nil
}

// ListRecoverablePlanProposals 返回未完成 materialization 或 Goal confirmation
// 已到重试时间的 proposal。该入口只供 trusted background reconciler 使用。
func (r *Repository) ListRecoverablePlanProposals(
	ctx context.Context,
	query ListRecoverablePlanProposalsQuery,
) ([]protocol.ExecutionPlanProposal, error) {
	if query.Limit <= 0 {
		return nil, fmt.Errorf("%w: positive proposal recovery limit is required", ErrInvariant)
	}
	now := query.Now.UTC()
	if now.IsZero() {
		now = r.currentTime()
	}
	rows, err := r.db.QueryContext(ctx, r.planProposalSelect()+`
WHERE (
        status = 'materializing'
        OR (status = 'materialized' AND confirmation_state = 'pending')
        OR (
          status = 'blocked'
          AND EXISTS (
            SELECT 1
            FROM execution_events AS receipt
            WHERE receipt.execution_id = execution_plan_proposals.reserved_execution_id
              AND receipt.command_id = execution_plan_proposals.materialization_command_id || ':plan'
              AND receipt.event_type = 'plan_activated'
              AND receipt.entity_type = 'plan'
              AND receipt.plan_id IS NOT NULL
          )
        )
      )
  AND (next_attempt_at IS NULL OR next_attempt_at <= `+r.bind(1)+`)
ORDER BY COALESCE(next_attempt_at, updated_at), proposal_id
LIMIT `+r.bind(2),
		r.timestamp(now),
		query.Limit,
	)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanPlanProposal)
}

func (r *Repository) planProposalSelect() string {
	return `SELECT
    proposal_id, owner_user_id, session_key, scope_kind,
    room_id, conversation_id, coordinator_agent_id,
    root_round_id, runtime_round_id, agent_round_id,
    target_execution_id, target_execution_version, base_plan_id,
    goal_id, goal_objective_revision, goal_activation_origin,
    goal_activation_reason, goal_reserved_execution_id, replaces_execution_id,
    ` + r.dialect.JSONText("document_json") + `, content_digest, status, version,
    reserved_execution_id, materialization_command_id,
    materialized_execution_id, materialized_plan_id,
    confirmation_state, attempt_count, next_attempt_at, last_error,
    created_at, updated_at, materialized_at
FROM execution_plan_proposals
`
}

func (r *Repository) getPlanProposal(
	ctx context.Context,
	queryer sqlQueryer,
	proposalID string,
) (*protocol.ExecutionPlanProposal, error) {
	item, err := scanPlanProposal(queryer.QueryRowContext(
		ctx,
		r.planProposalSelect()+`WHERE proposal_id = `+r.bind(1),
		strings.TrimSpace(proposalID),
	))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) getPlanProposalScoped(
	ctx context.Context,
	queryer sqlQueryer,
	access PlanProposalAccess,
) (*protocol.ExecutionPlanProposal, error) {
	item, err := scanPlanProposal(queryer.QueryRowContext(
		ctx,
		r.planProposalSelect()+`
WHERE proposal_id = `+r.bind(1)+`
  AND owner_user_id = `+r.bind(2)+`
  AND session_key = `+r.bind(3)+`
  AND scope_kind = `+r.bind(4)+`
  AND COALESCE(room_id, '') = `+r.bind(5)+`
  AND COALESCE(conversation_id, '') = `+r.bind(6)+`
  AND coordinator_agent_id = `+r.bind(7),
		access.ProposalID,
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

func scanPlanProposal(
	scanner interface{ Scan(...any) error },
) (protocol.ExecutionPlanProposal, error) {
	var item protocol.ExecutionPlanProposal
	var scopeKind, status, confirmationState, documentJSON string
	var roomID, conversationID, rootRoundID, runtimeRoundID, agentRoundID sql.NullString
	var targetExecutionID, basePlanID sql.NullString
	var goalID, goalActivationOrigin, goalActivationReason sql.NullString
	var goalReservedExecutionID, replacesExecutionID sql.NullString
	var reservedExecutionID, materializationCommandID sql.NullString
	var materializedExecutionID, materializedPlanID, lastError sql.NullString
	var nextAttemptAt, materializedAt sql.NullTime
	err := scanner.Scan(
		&item.ID, &item.OwnerUserID, &item.SessionKey, &scopeKind,
		&roomID, &conversationID, &item.CoordinatorAgentID,
		&rootRoundID, &runtimeRoundID, &agentRoundID,
		&targetExecutionID, &item.TargetExecutionVersion, &basePlanID,
		&goalID, &item.GoalObjectiveRevision, &goalActivationOrigin,
		&goalActivationReason, &goalReservedExecutionID, &replacesExecutionID,
		&documentJSON, &item.ContentDigest, &status, &item.Version,
		&reservedExecutionID, &materializationCommandID,
		&materializedExecutionID, &materializedPlanID,
		&confirmationState, &item.AttemptCount, &nextAttemptAt, &lastError,
		&item.CreatedAt, &item.UpdatedAt, &materializedAt,
	)
	if err != nil {
		return protocol.ExecutionPlanProposal{}, err
	}
	decoder := json.NewDecoder(strings.NewReader(documentJSON))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&item.Document); err != nil {
		return protocol.ExecutionPlanProposal{}, fmt.Errorf(
			"%w: decode execution plan proposal document: %v",
			ErrInvariant,
			err,
		)
	}
	if err = decoder.Decode(&struct{}{}); err != io.EOF {
		return protocol.ExecutionPlanProposal{}, fmt.Errorf(
			"%w: execution plan proposal document has trailing JSON",
			ErrInvariant,
		)
	}
	item.ScopeKind = protocol.ExecutionScopeKind(scopeKind)
	item.RoomID = nullStringValue(roomID)
	item.ConversationID = nullStringValue(conversationID)
	item.RootRoundID = nullStringValue(rootRoundID)
	item.RuntimeRoundID = nullStringValue(runtimeRoundID)
	item.AgentRoundID = nullStringValue(agentRoundID)
	item.TargetExecutionID = nullStringValue(targetExecutionID)
	item.BasePlanID = nullStringValue(basePlanID)
	item.GoalID = nullStringValue(goalID)
	item.GoalActivationOrigin = protocol.GoalActivationOrigin(nullStringValue(goalActivationOrigin))
	item.GoalActivationReason = protocol.GoalActivationReason(nullStringValue(goalActivationReason))
	item.GoalReservedExecutionID = nullStringValue(goalReservedExecutionID)
	item.ReplacesExecutionID = nullStringValue(replacesExecutionID)
	item.Status = protocol.ExecutionPlanProposalStatus(status)
	item.ReservedExecutionID = nullStringValue(reservedExecutionID)
	item.MaterializationCommandID = nullStringValue(materializationCommandID)
	item.MaterializedExecutionID = nullStringValue(materializedExecutionID)
	item.MaterializedPlanID = nullStringValue(materializedPlanID)
	item.ConfirmationState = protocol.ExecutionPlanProposalConfirmationState(confirmationState)
	item.NextAttemptAt = nullTimePointer(nextAttemptAt)
	item.LastError = nullStringValue(lastError)
	item.MaterializedAt = nullTimePointer(materializedAt)
	item = item.Normalized()
	digest, err := protocol.DigestExecutionPlanProposalImmutable(item)
	if err != nil {
		return protocol.ExecutionPlanProposal{}, fmt.Errorf("digest immutable execution plan proposal: %w", err)
	}
	if digest != item.ContentDigest {
		return protocol.ExecutionPlanProposal{}, fmt.Errorf(
			"%w: execution plan proposal immutable digest mismatch",
			ErrInvariant,
		)
	}
	return item, nil
}

func normalizePlanProposalAccess(access PlanProposalAccess) (PlanProposalAccess, error) {
	access.ProposalID = strings.TrimSpace(access.ProposalID)
	if access.ProposalID == "" {
		return PlanProposalAccess{}, fmt.Errorf(
			"%w: proposal id is required",
			ErrInvariant,
		)
	}
	binding, err := normalizePlanProposalBindingAccess(PlanProposalBindingAccess{
		OwnerUserID:        access.OwnerUserID,
		SessionKey:         access.SessionKey,
		ScopeKind:          access.ScopeKind,
		RoomID:             access.RoomID,
		ConversationID:     access.ConversationID,
		CoordinatorAgentID: access.CoordinatorAgentID,
	})
	if err != nil {
		return PlanProposalAccess{}, err
	}
	access.OwnerUserID = binding.OwnerUserID
	access.SessionKey = binding.SessionKey
	access.ScopeKind = binding.ScopeKind
	access.RoomID = binding.RoomID
	access.ConversationID = binding.ConversationID
	access.CoordinatorAgentID = binding.CoordinatorAgentID
	if err = validatePlanProposalStringWidth("proposal id", access.ProposalID, 64); err != nil {
		return PlanProposalAccess{}, err
	}
	return access, nil
}

func normalizePlanProposalBindingAccess(
	access PlanProposalBindingAccess,
) (PlanProposalBindingAccess, error) {
	access.OwnerUserID = strings.TrimSpace(access.OwnerUserID)
	access.SessionKey = strings.TrimSpace(access.SessionKey)
	access.RoomID = strings.TrimSpace(access.RoomID)
	access.ConversationID = strings.TrimSpace(access.ConversationID)
	access.CoordinatorAgentID = strings.TrimSpace(access.CoordinatorAgentID)
	if access.OwnerUserID == "" || access.SessionKey == "" || access.CoordinatorAgentID == "" {
		return PlanProposalBindingAccess{}, fmt.Errorf(
			"%w: proposal binding owner, session and coordinator are required",
			ErrInvariant,
		)
	}
	for _, field := range []struct {
		name  string
		value string
		limit int
	}{
		{name: "owner user id", value: access.OwnerUserID, limit: 128},
		{name: "session key", value: access.SessionKey, limit: 512},
		{name: "room id", value: access.RoomID, limit: 64},
		{name: "conversation id", value: access.ConversationID, limit: 64},
		{name: "coordinator agent id", value: access.CoordinatorAgentID, limit: 128},
	} {
		if err := validatePlanProposalStringWidth(field.name, field.value, field.limit); err != nil {
			return PlanProposalBindingAccess{}, err
		}
	}
	switch access.ScopeKind {
	case protocol.ExecutionScopeDM:
		if access.RoomID != "" || access.ConversationID != "" {
			return PlanProposalBindingAccess{}, fmt.Errorf("%w: DM proposal cannot carry Room identity", ErrInvariant)
		}
	case protocol.ExecutionScopeRoom:
		if access.RoomID == "" || access.ConversationID == "" {
			return PlanProposalBindingAccess{}, fmt.Errorf("%w: Room proposal requires room and conversation identity", ErrInvariant)
		}
	default:
		return PlanProposalBindingAccess{}, fmt.Errorf("%w: proposal scope %q is invalid", ErrInvariant, access.ScopeKind)
	}
	return access, nil
}

func validatePlanProposalStringWidth(name, value string, limit int) error {
	if len(value) > limit {
		return fmt.Errorf("%w: %s exceeds %d bytes", ErrInvariant, name, limit)
	}
	return nil
}

func planProposalAccessFor(item protocol.ExecutionPlanProposal) PlanProposalAccess {
	return PlanProposalAccess{
		ProposalID:         item.ID,
		OwnerUserID:        item.OwnerUserID,
		SessionKey:         item.SessionKey,
		ScopeKind:          item.ScopeKind,
		RoomID:             item.RoomID,
		ConversationID:     item.ConversationID,
		CoordinatorAgentID: item.CoordinatorAgentID,
	}
}

func planProposalAccessMatches(
	item protocol.ExecutionPlanProposal,
	access PlanProposalAccess,
) bool {
	return item.ID == access.ProposalID &&
		item.OwnerUserID == access.OwnerUserID &&
		item.SessionKey == access.SessionKey &&
		item.ScopeKind == access.ScopeKind &&
		item.RoomID == access.RoomID &&
		item.ConversationID == access.ConversationID &&
		item.CoordinatorAgentID == access.CoordinatorAgentID
}
