// INPUT: Owner-scoped durable template usage receipts, Execution lifecycle and Attempts.
// OUTPUT: Observed per-version run, failure and completed elapsed-wall-time aggregates.
// POS: Read-only template improvement evidence; no automatic template edits or runtime cost estimate.
package workgraphworkflow

import (
	"context"
	"database/sql"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"sort"
	"time"
)

func (r *Repository) Statistics(ctx context.Context, owner string) (map[string][]protocol.WorkGraphWorkflowStatistics, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT u.workflow_id,u.workflow_version,e.status,e.created_at,e.completed_at,
 (SELECT COUNT(*) FROM execution_attempts a WHERE a.execution_id=e.execution_id),
 (SELECT COUNT(*) FROM execution_attempts a WHERE a.execution_id=e.execution_id AND a.status='failed')
 FROM execution_workflow_usage u JOIN executions e ON e.execution_id=u.execution_id
 WHERE u.owner_user_id=`+r.bind(1)+` AND e.owner_user_id=`+r.bind(2), owner, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type key struct {
		id      string
		version int64
	}
	counts := map[key]*protocol.WorkGraphWorkflowStatistics{}
	for rows.Next() {
		var id string
		var version int64
		var status protocol.ExecutionStatus
		var created time.Time
		var ended sql.NullTime
		var attempts, failed int
		if err = rows.Scan(&id, &version, &status, &created, &ended, &attempts, &failed); err != nil {
			return nil, err
		}
		k := key{id, version}
		stat := counts[k]
		if stat == nil {
			stat = &protocol.WorkGraphWorkflowStatistics{Version: version}
			counts[k] = stat
		}
		stat.Runs++
		stat.Attempts += attempts
		stat.FailedAttempts += failed
		switch {
		case status == protocol.ExecutionStatusCompleted:
			stat.Completed++
			if ended.Valid && !ended.Time.Before(created) {
				stat.ElapsedSamples++
				stat.ElapsedMilliseconds += ended.Time.Sub(created).Milliseconds()
			}
		case status.Terminal():
			stat.ClosedWithoutCompletion++
		default:
			stat.Active++
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	result := map[string][]protocol.WorkGraphWorkflowStatistics{}
	for k, v := range counts {
		result[k.id] = append(result[k.id], *v)
	}
	for id := range result {
		sort.Slice(result[id], func(i, j int) bool { return result[id][i].Version > result[id][j].Version })
	}
	return result, nil
}
