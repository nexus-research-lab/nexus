// INPUT: Validated workflow provenance on an immutable first Plan and its transaction.
// OUTPUT: One durable template usage receipt per Execution, atomic with materialization.
// POS: Template statistics attribution; preview and unsuccessful Plan writes cannot count.
package orchestration

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (r *Repository) recordWorkflowUsage(ctx context.Context, tx *sql.Tx, plan protocol.ExecutionPlanRevision) error {
	value, ok := plan.Metadata["workflow_source"]
	if !ok {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var source protocol.WorkGraphWorkflowSource
	if err = json.Unmarshal(raw, &source); err != nil {
		return err
	}
	if source.ID == "" || source.Version < 1 || plan.Revision != 1 {
		return ErrInvariant
	}
	parameters, err := json.Marshal(source.Parameters)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO execution_workflow_usage(execution_id,owner_user_id,workflow_id,workflow_version,parameters_json,created_at)
 SELECT execution_id,owner_user_id,`+r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+` FROM executions WHERE execution_id=`+r.bind(5), source.ID, source.Version, string(parameters), r.timestamp(plan.CreatedAt), plan.ExecutionID)
	return err
}
