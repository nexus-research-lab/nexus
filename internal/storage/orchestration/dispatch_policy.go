// INPUT: Coordinator-authorized policy replacement or a ready exact policy claim.
// OUTPUT: CAS policy receipts and atomic Assignment/Attempt/outbox admission.
// POS: Declarative routing persistence; waiting and queue delivery never replay business effects.
package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type ConfigureDispatchCommand struct {
	ExecutionID, PlanID, RequestDigest string
	ExpectedExecutionVersion           int64
	Policies                           []protocol.ExecutionDispatchPolicy
	Meta                               CommandMeta
}

type ActivateDispatchPolicyCommand struct {
	PolicyID   string
	Assignment AssignCommand
}

func (r *Repository) ConfigureDispatch(ctx context.Context, command ConfigureDispatchCommand) (*protocol.ExecutionSnapshot, error) {
	m, err := r.beginMutation(ctx, command.ExecutionID, command.ExpectedExecutionVersion, command.Meta, protocol.ExecutionEventDispatchConfigured)
	if err != nil {
		return nil, err
	}
	defer r.abortMutation(m)
	if m.replayed {
		var q sqlQueryer = r.db
		if m.tx != nil {
			q = m.tx
		}
		receipt, err := r.findEventByCommand(ctx, q, command.ExecutionID, command.Meta.CommandID)
		if err != nil {
			return nil, err
		}
		if receipt == nil || receipt.ActorID != command.Meta.ActorID || receipt.Payload["request_digest"] != command.RequestDigest {
			return nil, ErrCommandConflict
		}
		return r.finishMutation(ctx, m, command.Meta, protocol.ExecutionEvent{})
	}
	active, err := r.getActivePlan(ctx, m.tx, command.ExecutionID)
	if err != nil {
		return nil, err
	}
	if active == nil || active.ID != command.PlanID {
		return nil, ErrWorkNotReady
	}
	if len(command.Policies) > protocol.ExecutionProjectionCollectionLimit {
		return nil, ErrInvariant
	}
	if _, err = m.tx.ExecContext(ctx, `UPDATE execution_dispatch_policies SET state='cancelled', reason='policy_replaced' WHERE execution_id=`+r.bind(1)+` AND state='pending'`, command.ExecutionID); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, p := range command.Policies {
		if p.ID == "" || p.ExecutionID != command.ExecutionID || p.PlanID != command.PlanID || seen[p.WorkItemID] || p.TargetAgentID == "" || p.ReturnToAgentID == "" {
			return nil, ErrInvariant
		}
		seen[p.WorkItemID] = true
		var member int
		if err = m.tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM execution_plan_items WHERE plan_id=`+r.bind(1)+` AND work_item_id=`+r.bind(2)+` AND spec_id=`+r.bind(3), p.PlanID, p.WorkItemID, p.SpecID).Scan(&member); err != nil {
			return nil, err
		}
		if member != 1 {
			return nil, ErrInvariant
		}
		p.State = "pending"
		p.Reason = ""
		p.AssignmentID = ""
		p.AttemptID = ""
		raw, err := json.Marshal(p)
		if err != nil {
			return nil, err
		}
		now := r.timestamp(r.currentTime())
		_, err = m.tx.ExecContext(ctx, `INSERT INTO execution_dispatch_policies (policy_id,execution_id,plan_id,work_item_id,spec_id,intent_json,state,next_attempt_at,created_at) VALUES (`+r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+r.bind(6)+`,'pending',`+r.bind(7)+`,`+r.bind(8)+`)`, p.ID, p.ExecutionID, p.PlanID, p.WorkItemID, p.SpecID, string(raw), now, now)
		if err != nil {
			return nil, err
		}
	}
	command.Meta.Payload = map[string]any{"request_digest": command.RequestDigest}
	return r.finishMutation(ctx, m, command.Meta, protocol.ExecutionEvent{EntityType: protocol.ExecutionEntityExecution, EntityID: command.ExecutionID, EntityVersion: command.ExpectedExecutionVersion + 1, PlanID: command.PlanID})
}

const dispatchPolicySelect = `SELECT intent_json,state,reason,assignment_id,attempt_id FROM execution_dispatch_policies `

func scanDispatchPolicy(row interface{ Scan(...any) error }) (protocol.ExecutionDispatchPolicy, error) {
	var p protocol.ExecutionDispatchPolicy
	var raw, state, reason, assignment, attempt string
	err := row.Scan(&raw, &state, &reason, &assignment, &attempt)
	if err != nil {
		return p, err
	}
	if err = json.Unmarshal([]byte(raw), &p); err != nil {
		return p, err
	}
	p.State, p.Reason, p.AssignmentID, p.AttemptID = state, reason, assignment, attempt
	return p, nil
}
func (r *Repository) listDispatchPolicies(ctx context.Context, q sqlQueryer, planID string) ([]protocol.ExecutionDispatchPolicy, error) {
	rows, err := q.QueryContext(ctx, dispatchPolicySelect+`WHERE plan_id=`+r.bind(1)+` AND state!='cancelled' ORDER BY created_at,policy_id`, planID)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanDispatchPolicy)
}
func (r *Repository) ListDueDispatchPolicies(ctx context.Context, now time.Time, limit int) ([]protocol.ExecutionDispatchPolicy, error) {
	rows, err := r.db.QueryContext(ctx, dispatchPolicySelect+`WHERE state IN ('pending','assigned') AND next_attempt_at<=`+r.bind(1)+` ORDER BY next_attempt_at,policy_id LIMIT `+r.bind(2), r.timestamp(now), limit)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanDispatchPolicy)
}
func (r *Repository) DeferDispatchPolicy(ctx context.Context, id, expected, state, reason string, next time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE execution_dispatch_policies SET state=`+r.bind(1)+`,reason=`+r.bind(2)+`,next_attempt_at=`+r.bind(3)+` WHERE policy_id=`+r.bind(4)+` AND state=`+r.bind(5), state, reason, r.timestamp(next), id, expected)
	return err
}
func (r *Repository) ActivateDispatchPolicy(ctx context.Context, c ActivateDispatchPolicyCommand) (*protocol.ExecutionSnapshot, error) {
	command := c.Assignment
	if err := normalizeAssignment(&command.Assignment, r.currentTime()); err != nil {
		return nil, err
	}
	m, err := r.beginMutation(ctx, command.Assignment.ExecutionID, command.ExpectedExecutionVersion, command.Meta, protocol.ExecutionEventWorkAssigned)
	if err != nil {
		return nil, err
	}
	defer r.abortMutation(m)
	if m.replayed {
		return r.finishMutation(ctx, m, command.Meta, protocol.ExecutionEvent{})
	}
	var state, raw string
	if err = m.tx.QueryRowContext(ctx, `SELECT state,intent_json FROM execution_dispatch_policies WHERE policy_id=`+r.bind(1), c.PolicyID).Scan(&state, &raw); err != nil {
		return nil, err
	}
	var p protocol.ExecutionDispatchPolicy
	if err = json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, err
	}
	a := command.Assignment
	if state != "pending" || a.ExecutionID != p.ExecutionID || a.PlanID != p.PlanID || a.WorkItemID != p.WorkItemID || a.SpecID != p.SpecID || a.OwnerAgentID != p.TargetAgentID || a.ReturnToAgentID != p.ReturnToAgentID {
		return nil, fmt.Errorf("%w: dispatch policy changed", ErrWorkNotReady)
	}
	event, err := r.insertAssignmentChain(ctx, m.tx, command)
	if err != nil {
		return nil, err
	}
	state = "assigned"
	if command.Dispatch != nil {
		state = "queued"
	}
	if _, err = m.tx.ExecContext(ctx, `UPDATE execution_dispatch_policies SET state=`+r.bind(1)+`,reason='',assignment_id=`+r.bind(2)+`,attempt_id=`+r.bind(3)+` WHERE policy_id=`+r.bind(4), state, a.ID, command.RootAttempt.ID, p.ID); err != nil {
		return nil, err
	}
	return r.finishMutation(ctx, m, command.Meta, event)
}
