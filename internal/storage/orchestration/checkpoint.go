// INPUT: Current root-owner checkpoint, immutable intent digest and Execution CAS.
// OUTPUT: Append-only progress receipt and bounded latest-per-work resume projection.
// POS: Durable observations; no completion, acceptance or retry transition.
package orchestration

import (
	"context"
	"encoding/json"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type RecordCheckpointCommand struct {
	Checkpoint               protocol.WorkCheckpoint
	ExpectedExecutionVersion int64
	Digest                   string
	Meta                     CommandMeta
}

func (r *Repository) RecordCheckpoint(ctx context.Context, c RecordCheckpointCommand) (*protocol.ExecutionSnapshot, error) {
	p := c.Checkpoint
	m, err := r.beginMutation(ctx, p.ExecutionID, c.ExpectedExecutionVersion, c.Meta, protocol.ExecutionEventCheckpointRecorded)
	if err != nil {
		return nil, err
	}
	defer r.abortMutation(m)
	if m.replayed {
		var q sqlQueryer = r.db
		if m.tx != nil {
			q = m.tx
		}
		receipt, err := r.findEventByCommand(ctx, q, p.ExecutionID, c.Meta.CommandID)
		if err != nil {
			return nil, err
		}
		if receipt == nil || receipt.ActorID != p.AgentID || receipt.Payload["checkpoint_digest"] != c.Digest {
			return nil, ErrCommandConflict
		}
		return r.finishMutation(ctx, m, c.Meta, protocol.ExecutionEvent{})
	}
	var eligible int
	err = m.tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM execution_attempts a
 JOIN execution_work_assignments w ON w.assignment_id=a.assignment_id
 JOIN execution_plan_revisions p ON p.plan_id=a.plan_id
 JOIN execution_work_item_states s ON s.work_item_id=a.work_item_id
 WHERE a.attempt_id=`+r.bind(1)+` AND a.execution_id=`+r.bind(2)+` AND a.work_item_id=`+r.bind(3)+` AND a.spec_id=`+r.bind(4)+` AND a.assignment_id=`+r.bind(5)+` AND a.executor_agent_id=`+r.bind(6)+`
 AND a.status IN ('pending','running') AND a.parent_attempt_id IS NULL
 AND w.status IN ('assigned','active') AND p.status='active' AND s.status='open'
 AND NOT EXISTS (SELECT 1 FROM execution_submissions d WHERE d.attempt_id=a.attempt_id)`, p.AttemptID, p.ExecutionID, p.WorkItemID, p.SpecID, p.AssignmentID, p.AgentID).Scan(&eligible)
	if err != nil {
		return nil, err
	}
	if eligible != 1 {
		return nil, ErrWorkNotReady
	}
	var count int
	if err = m.tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM execution_work_checkpoints WHERE attempt_id=`+r.bind(1), p.AttemptID).Scan(&count); err != nil {
		return nil, err
	}
	if count >= 32 {
		return nil, ErrInvariant
	}
	p.CreatedAt = r.currentTime()
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	_, err = m.tx.ExecContext(ctx, `INSERT INTO execution_work_checkpoints(checkpoint_id,execution_id,work_item_id,spec_id,attempt_id,payload_json,created_at,execution_version) VALUES (`+r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+r.bind(6)+`,`+r.bind(7)+`,`+r.bind(8)+`)`, p.ID, p.ExecutionID, p.WorkItemID, p.SpecID, p.AttemptID, string(raw), r.timestamp(p.CreatedAt), c.ExpectedExecutionVersion+1)
	if err != nil {
		return nil, err
	}
	c.Meta.Payload = map[string]any{"checkpoint_digest": c.Digest}
	return r.finishMutation(ctx, m, c.Meta, protocol.ExecutionEvent{EntityType: protocol.ExecutionEntityExecution, EntityID: p.ExecutionID, EntityVersion: c.ExpectedExecutionVersion + 1, PlanID: p.PlanID, WorkItemID: p.WorkItemID, SpecID: p.SpecID, AssignmentID: p.AssignmentID, AttemptID: p.AttemptID})
}
func (r *Repository) listLatestCheckpoints(ctx context.Context, q sqlQueryer, execution, plan string) ([]protocol.WorkCheckpoint, error) {
	rows, err := q.QueryContext(ctx, `SELECT checkpoint.payload_json FROM execution_plan_items item JOIN execution_work_checkpoints checkpoint ON checkpoint.checkpoint_id=(SELECT c.checkpoint_id FROM execution_work_checkpoints c WHERE c.execution_id=`+r.bind(1)+` AND c.work_item_id=item.work_item_id AND c.spec_id=item.spec_id ORDER BY c.execution_version DESC LIMIT 1) WHERE item.plan_id=`+r.bind(2)+` ORDER BY item.position`, execution, plan)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, func(row interface{ Scan(...any) error }) (protocol.WorkCheckpoint, error) {
		var p protocol.WorkCheckpoint
		var raw string
		if err := row.Scan(&raw); err != nil {
			return p, err
		}
		err := json.Unmarshal([]byte(raw), &p)
		return p, err
	})
}
