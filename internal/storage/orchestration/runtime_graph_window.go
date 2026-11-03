// INPUT: Exact owner/session/runtime scope and conservative write-time read hints.
// OUTPUT: All possible canvas/ownership facts plus an independent recent detail window, using one control-endpoint set and indexed latest-success lookups.
// POS: SQL read reduction before Go visibility projection; never lets successful Read/Grep history displace canvas facts.
package orchestration

import (
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (r *Repository) workGraphRuntimeReadQuery(condition string) string {
	// The lightweight CTE omits summaries/metadata. Keep uncertain historical rows,
	// ancestors and late promotions; only proven detail leaves may be windowed.
	// Materialize control endpoints once: a correlated source-or-target lookup
	// otherwise scans the session's edge history for every successful detail.
	// Find the last success via an ordered index lookup. An EXISTS range may
	// choose repeated sequential scans when one recovery key dominates a round.
	return fmt.Sprintf(`WITH scoped AS MATERIALIZED (
 SELECT node_run_id, owner_user_id, session_key, node_kind, agent_round_id,
        subject_id, parent_subject_id, status, workgraph_read_class, recovery_key, started_at, updated_at
 FROM runtime_graph_node_runs
 WHERE owner_user_id = %s AND session_key = %s AND %s
), control_edges AS MATERIALIZED (
 SELECT source_node_run_id, target_node_run_id FROM runtime_graph_edge_runs
 WHERE owner_user_id = (SELECT owner_user_id FROM scoped LIMIT 1)
   AND session_key = (SELECT session_key FROM scoped LIMIT 1)
   AND edge_kind IN ('retry', 'loop_back')
), retained AS MATERIALIZED (
 SELECT n.node_run_id FROM scoped n
 WHERE n.workgraph_read_class <> 'detail' OR n.status <> 'succeeded'
 OR EXISTS (SELECT 1 FROM runtime_graph_artifact_refs a WHERE a.owner_user_id = n.owner_user_id AND a.session_key = n.session_key AND a.agent_round_id = n.agent_round_id AND a.tool_use_id = n.subject_id)
 OR n.node_run_id IN (SELECT source_node_run_id FROM control_edges UNION SELECT target_node_run_id FROM control_edges)
 OR EXISTS (SELECT 1 FROM runtime_graph_node_runs child WHERE child.owner_user_id = n.owner_user_id AND child.session_key = n.session_key AND child.agent_round_id = n.agent_round_id AND child.parent_subject_id = n.subject_id)
 OR (n.recovery_key <> ''
   AND n.node_run_id = (SELECT latest.node_run_id FROM runtime_graph_node_runs latest WHERE latest.owner_user_id = n.owner_user_id AND latest.session_key = n.session_key AND latest.recovery_key = n.recovery_key AND latest.status = 'succeeded' ORDER BY latest.started_at DESC, latest.node_run_id DESC LIMIT 1)
   AND EXISTS (SELECT 1 FROM runtime_graph_node_runs f WHERE f.owner_user_id = n.owner_user_id AND f.session_key = n.session_key AND f.recovery_key = n.recovery_key AND f.status IN ('failed', 'cancelled', 'interrupted') AND (f.started_at, f.node_run_id) < (n.started_at, n.node_run_id))
 )
), details AS MATERIALIZED (
 SELECT n.node_run_id FROM scoped n WHERE NOT EXISTS (SELECT 1 FROM retained r WHERE r.node_run_id = n.node_run_id)
 ORDER BY n.updated_at DESC, n.started_at DESC, n.node_run_id DESC LIMIT %d
)
%s
WHERE node_run_id IN (SELECT node_run_id FROM retained UNION ALL SELECT node_run_id FROM details)
ORDER BY updated_at DESC, started_at DESC, node_run_id DESC`, r.bind(1), r.bind(2), condition, protocol.ExecutionRuntimeGraphNodeProjectionLimit, runtimeGraphNodeSelectColumns)
}
