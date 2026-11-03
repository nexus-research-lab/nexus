// INPUT: Version 160's frozen execution backlog and recovered exact runtime identities.
// OUTPUT: Paginated migration input and insert-only facts committed with the completion marker.
// POS: Historical WorkGraph migration persistence; never overwrites live observations.
package orchestration

import (
	"context"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (r *Repository) ListRuntimeHistoryBackfills(ctx context.Context, after protocol.Execution, limit int) ([]protocol.Execution, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT e.execution_id, e.owner_user_id, e.session_key, COALESCE(e.root_round_id, '')
FROM executions e JOIN runtime_graph_history_backfills b ON b.execution_id = e.execution_id
WHERE b.completed_at IS NULL AND (e.owner_user_id, e.session_key, e.execution_id) > (`+r.bind(1)+`, `+r.bind(2)+`, `+r.bind(3)+`) ORDER BY e.owner_user_id, e.session_key, e.execution_id LIMIT `+r.bind(4), after.OwnerUserID, after.SessionKey, after.ID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []protocol.Execution
	for rows.Next() {
		var e protocol.Execution
		if err := rows.Scan(&e.ID, &e.OwnerUserID, &e.SessionKey, &e.RootRoundID); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

// RuntimeGraphForMigration is deliberately unbounded. Only the frozen upgrade
// backlog uses this read; public history and views have separate read paths.
func (r *Repository) RuntimeGraphForMigration(ctx context.Context, e protocol.Execution) (protocol.ExecutionRuntimeGraph, error) {
	return r.getRuntimeGraph(ctx, e.OwnerUserID, e.SessionKey, e.ID, e.RootRoundID, false, false)
}

func (r *Repository) CompleteRuntimeHistoryBackfill(ctx context.Context, e protocol.Execution, graph protocol.ExecutionRuntimeGraph) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, node := range graph.Nodes {
		if node.OwnerUserID != e.OwnerUserID || node.SessionKey != e.SessionKey || (node.ExecutionID != "" && node.ExecutionID != e.ID) {
			return fmt.Errorf("%w: historical runtime node scope mismatch", ErrInvariant)
		}
		if err = r.writeRuntimeGraphNode(ctx, tx, node, false); err != nil {
			return err
		}
		// A concurrently updated row keeps its live classification and content.
		if _, err = tx.ExecContext(ctx, `UPDATE runtime_graph_node_runs SET workgraph_read_class = `+r.bind(1)+`, recovery_key = `+r.bind(2)+` WHERE node_run_id = `+r.bind(3)+` AND updated_at = `+r.bind(4)+` AND workgraph_read_class = ''`, node.WorkGraphReadClass, node.RecoveryKey, node.ID, r.timestamp(node.UpdatedAt)); err != nil {
			return err
		}
	}
	for _, edge := range graph.Edges {
		if edge.OwnerUserID != e.OwnerUserID || edge.SessionKey != e.SessionKey {
			return fmt.Errorf("%w: historical runtime edge scope mismatch", ErrInvariant)
		}
		if err = r.writeRuntimeGraphEdge(ctx, tx, edge); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE runtime_graph_history_backfills SET completed_at = `+r.bind(1)+` WHERE execution_id = `+r.bind(2)+` AND completed_at IS NULL`, r.timestamp(r.currentTime()), e.ID); err != nil {
		return err
	}
	return tx.Commit()
}
