// INPUT: exact ToolUse Artifact 事实与当前 Runtime Graph 节点集合。
// OUTPUT: 到达顺序无关的 Artifact upsert，以及分批读取后按 Agent round + ToolUse 精确回挂。
// POS: Runtime Graph Repository 的 Artifact 子域；不创建节点、不推断结果、不选择 Agent 路线。
package orchestration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// UpsertRuntimeGraphArtifact 先于或后于 Tool NodeRun 幂等记录 Artifact；它只
// 持久化 exact ToolUse 事实，不创建工具节点或推断执行结果。
func (r *Repository) UpsertRuntimeGraphArtifact(
	ctx context.Context,
	item protocol.ExecutionRuntimeArtifactRef,
) error {
	if err := validateRuntimeGraphArtifact(item); err != nil {
		return err
	}
	artifactJSON, err := json.Marshal(item.Artifact)
	if err != nil {
		return err
	}
	executionID := strings.TrimSpace(item.ExecutionID)
	if executionID == "" {
		var inherited sql.NullString
		err = r.db.QueryRowContext(ctx, `
SELECT execution_id
FROM runtime_graph_node_runs
WHERE owner_user_id = `+r.bind(1)+`
  AND session_key = `+r.bind(2)+`
  AND agent_round_id = `+r.bind(3)+`
  AND node_kind = 'tool'
  AND subject_id = `+r.bind(4)+`
ORDER BY updated_at DESC, node_run_id DESC
LIMIT 1`,
			item.OwnerUserID,
			item.SessionKey,
			item.AgentRoundID,
			item.ToolUseID,
		).Scan(&inherited)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if inherited.Valid {
			executionID = strings.TrimSpace(inherited.String)
		}
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO runtime_graph_artifact_refs (
    artifact_ref_id, graph_id, owner_user_id, session_key, execution_id,
    root_round_id, agent_round_id, tool_use_id, artifact_json,
    created_at, updated_at
) VALUES (`+
		r.bind(1)+`, `+r.bind(2)+`, `+r.bind(3)+`, `+r.bind(4)+`, `+r.bind(5)+`, `+
		r.bind(6)+`, `+r.bind(7)+`, `+r.bind(8)+`, `+r.jsonBind(9)+`, `+
		r.bind(10)+`, `+r.bind(11)+`
)
ON CONFLICT (artifact_ref_id) DO UPDATE SET
	execution_id = COALESCE(runtime_graph_artifact_refs.execution_id, excluded.execution_id),
    artifact_json = excluded.artifact_json,
    updated_at = CASE
        WHEN excluded.updated_at > runtime_graph_artifact_refs.updated_at
            THEN excluded.updated_at
        ELSE runtime_graph_artifact_refs.updated_at
    END`,
		item.ID,
		item.GraphID,
		item.OwnerUserID,
		item.SessionKey,
		nullString(executionID),
		item.RootRoundID,
		item.AgentRoundID,
		item.ToolUseID,
		string(artifactJSON),
		r.timestamp(item.CreatedAt),
		r.timestamp(item.UpdatedAt),
	)
	return err
}

func (r *Repository) attachRuntimeGraphArtifacts(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
	nodes []protocol.ExecutionRuntimeNodeRun,
) error {
	type exactTool struct {
		agentRoundID string
		toolUseID    string
	}
	toolNodeIndex := make(map[string]int)
	tools := make([]exactTool, 0)
	for index, node := range nodes {
		if node.Kind != protocol.ExecutionRuntimeNodeTool {
			continue
		}
		key := strings.TrimSpace(node.AgentRoundID) + "\x00" + strings.TrimSpace(node.SubjectID)
		if key == "\x00" {
			continue
		}
		if _, exists := toolNodeIndex[key]; exists {
			continue
		}
		toolNodeIndex[key] = index
		tools = append(tools, exactTool{
			agentRoundID: strings.TrimSpace(node.AgentRoundID),
			toolUseID:    strings.TrimSpace(node.SubjectID),
		})
	}
	if len(tools) == 0 {
		return nil
	}
	slices.SortFunc(tools, func(left, right exactTool) int {
		if order := strings.Compare(left.agentRoundID, right.agentRoundID); order != 0 {
			return order
		}
		return strings.Compare(left.toolUseID, right.toolUseID)
	})
	const toolBatchSize = 200
	for start := 0; start < len(tools); start += toolBatchSize {
		end := min(start+toolBatchSize, len(tools))
		args := []any{ownerUserID, sessionKey}
		conditions := make([]string, 0, end-start)
		for _, tool := range tools[start:end] {
			agentRoundBind := r.bind(len(args) + 1)
			args = append(args, tool.agentRoundID)
			toolUseBind := r.bind(len(args) + 1)
			args = append(args, tool.toolUseID)
			conditions = append(
				conditions,
				"(agent_round_id = "+agentRoundBind+" AND tool_use_id = "+toolUseBind+")",
			)
		}
		query := fmt.Sprintf(`
SELECT agent_round_id, tool_use_id, artifact_json
FROM runtime_graph_artifact_refs
WHERE owner_user_id = %s AND session_key = %s
  AND (%s)
ORDER BY updated_at DESC, artifact_ref_id DESC
LIMIT %d`,
			r.bind(1),
			r.bind(2),
			strings.Join(conditions, " OR "),
			(end-start)*protocol.ExecutionRuntimeGraphArtifactProjectionLimit,
		)
		rows, err := r.db.QueryContext(ctx, query, args...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var agentRoundID, toolUseID string
			var artifactJSON []byte
			if err = rows.Scan(&agentRoundID, &toolUseID, &artifactJSON); err != nil {
				rows.Close()
				return err
			}
			index, exists := toolNodeIndex[strings.TrimSpace(agentRoundID)+"\x00"+strings.TrimSpace(toolUseID)]
			if !exists || len(nodes[index].Artifacts) >= protocol.ExecutionRuntimeGraphArtifactProjectionLimit {
				continue
			}
			var artifact protocol.WorkspaceFileArtifactBlock
			if json.Unmarshal(artifactJSON, &artifact) != nil ||
				strings.TrimSpace(artifact.Path) == "" ||
				strings.TrimSpace(artifact.SourceToolUseID) != strings.TrimSpace(toolUseID) {
				continue
			}
			duplicate := false
			for _, current := range nodes[index].Artifacts {
				if current.ID != "" && current.ID == artifact.ID ||
					(current.SourceToolUseID == artifact.SourceToolUseID && current.Path == artifact.Path) {
					duplicate = true
					break
				}
			}
			if !duplicate {
				nodes[index].Artifacts = append(nodes[index].Artifacts, artifact)
			}
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return err
		}
		if err = rows.Close(); err != nil {
			return err
		}
	}
	return nil
}

func validateRuntimeGraphArtifact(item protocol.ExecutionRuntimeArtifactRef) error {
	path := strings.TrimSpace(item.Artifact.Path)
	if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.GraphID) == "" ||
		strings.TrimSpace(item.OwnerUserID) == "" || strings.TrimSpace(item.SessionKey) == "" ||
		strings.TrimSpace(item.RootRoundID) == "" || strings.TrimSpace(item.AgentRoundID) == "" ||
		strings.TrimSpace(item.ToolUseID) == "" || item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() ||
		path == "" || strings.TrimSpace(item.Artifact.SourceToolUseID) != strings.TrimSpace(item.ToolUseID) {
		return fmt.Errorf("%w: runtime graph artifact identity, exact ToolUse and timestamps are required", ErrInvariant)
	}
	cleaned := filepath.ToSlash(filepath.Clean(path))
	if filepath.IsAbs(path) || strings.HasPrefix(path, "~") || cleaned == "." ||
		cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") {
		return fmt.Errorf("%w: runtime graph artifact path must be workspace relative", ErrInvariant)
	}
	return nil
}
