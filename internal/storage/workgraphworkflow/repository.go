// INPUT: 已验证的 WorkGraph Workflow aggregate、owner/name 查询与 SQL 方言。
// OUTPUT: 原子持久化的 workflow、节点和依赖；owner scope 外一律不可见。
// POS: 可复用 WorkGraph Workflow 的关系数据库边界。
package workgraphworkflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// Repository 持久化 WorkGraph Workflow aggregate。
type Repository struct {
	db      *sql.DB
	dialect storage.SQLDialect
}

// NewRepository 创建跨 SQLite/PostgreSQL 的 Workflow repository。
func NewRepository(cfg config.Config, db *sql.DB) *Repository {
	return &Repository{db: db, dialect: storage.NewSQLDialect(cfg.DatabaseDriver)}
}

// Create 原子写入 Workflow、节点与依赖。
func (r *Repository) Create(
	ctx context.Context,
	workflow protocol.WorkGraphWorkflow,
) (*protocol.WorkGraphWorkflow, error) {
	criteriaJSON, err := marshalJSON(workflow.CompletionCriteria)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `
INSERT INTO workgraph_workflows (
    workflow_id, owner_user_id, slash_name, title, description,
    source_execution_id, source_session_key, objective,
    completion_criteria_json, version, created_at, updated_at
) VALUES (`+
		r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+
		r.bind(6)+`,`+r.bind(7)+`,`+r.bind(8)+`,`+r.jsonBind(9)+`,`+r.bind(10)+`,`+
		r.bind(11)+`,`+r.bind(12)+`)`,
		workflow.ID,
		workflow.OwnerUserID,
		workflow.SlashName,
		workflow.Title,
		nullString(workflow.Description),
		workflow.SourceExecutionID,
		workflow.SourceSessionKey,
		workflow.Objective,
		criteriaJSON,
		workflow.Version,
		r.timestamp(workflow.CreatedAt),
		r.timestamp(workflow.UpdatedAt),
	)
	if err != nil {
		return nil, err
	}
	for _, node := range workflow.Nodes {
		acceptanceJSON, marshalErr := marshalJSON(node.AcceptanceCriteria)
		if marshalErr != nil {
			return nil, marshalErr
		}
		_, err = tx.ExecContext(ctx, `
INSERT INTO workgraph_workflow_nodes (
    workflow_id, logical_key, source_work_item_id, role, kind,
    subject, objective, deliverable, acceptance_criteria_json,
    is_required, is_terminal, parent_logical_key, position
) VALUES (`+
			r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+
			r.bind(6)+`,`+r.bind(7)+`,`+r.bind(8)+`,`+r.jsonBind(9)+`,`+
			r.bind(10)+`,`+r.bind(11)+`,`+r.bind(12)+`,`+r.bind(13)+`)`,
			workflow.ID,
			node.LogicalKey,
			node.SourceWorkItemID,
			node.Role,
			node.Kind,
			node.Subject,
			node.Objective,
			node.Deliverable,
			acceptanceJSON,
			node.Required,
			node.Terminal,
			nil,
			node.Position,
		)
		if err != nil {
			return nil, err
		}
	}
	for _, node := range workflow.Nodes {
		if strings.TrimSpace(node.ParentLogicalKey) == "" {
			continue
		}
		_, err = tx.ExecContext(ctx, `
UPDATE workgraph_workflow_nodes
SET parent_logical_key = `+r.bind(1)+`
WHERE workflow_id = `+r.bind(2)+` AND logical_key = `+r.bind(3),
			node.ParentLogicalKey, workflow.ID, node.LogicalKey,
		)
		if err != nil {
			return nil, err
		}
	}
	for _, dependency := range workflow.Dependencies {
		_, err = tx.ExecContext(ctx, `
INSERT INTO workgraph_workflow_dependencies (
    workflow_id, logical_key, depends_on_logical_key, dependency_kind
) VALUES (`+r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`)`,
			workflow.ID,
			dependency.LogicalKey,
			dependency.DependsOnLogicalKey,
			dependency.Kind,
		)
		if err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, workflow.OwnerUserID, workflow.ID)
}

// List 返回 owner 的完整 Workflow aggregates，供 UI 和命令目录共同消费。
func (r *Repository) List(
	ctx context.Context,
	ownerUserID string,
) ([]protocol.WorkGraphWorkflow, error) {
	rows, err := r.db.QueryContext(ctx, r.workflowSelect()+`
WHERE owner_user_id = `+r.bind(1)+`
ORDER BY updated_at DESC, workflow_id ASC`, strings.TrimSpace(ownerUserID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]protocol.WorkGraphWorkflow, 0)
	for rows.Next() {
		workflow, scanErr := scanWorkflow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		if loadErr := r.loadGraph(ctx, &workflow); loadErr != nil {
			return nil, loadErr
		}
		items = append(items, workflow)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// GetByID 按 owner + opaque ID 读取 Workflow。
func (r *Repository) GetByID(
	ctx context.Context,
	ownerUserID string,
	workflowID string,
) (*protocol.WorkGraphWorkflow, error) {
	workflow, err := scanWorkflow(r.db.QueryRowContext(ctx, r.workflowSelect()+`
WHERE owner_user_id = `+r.bind(1)+` AND workflow_id = `+r.bind(2),
		strings.TrimSpace(ownerUserID), strings.TrimSpace(workflowID)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = r.loadGraph(ctx, &workflow); err != nil {
		return nil, err
	}
	return &workflow, nil
}

// GetBySlashName 按 owner + canonical Slash name 读取 Workflow。
func (r *Repository) GetBySlashName(
	ctx context.Context,
	ownerUserID string,
	slashName string,
) (*protocol.WorkGraphWorkflow, error) {
	workflow, err := scanWorkflow(r.db.QueryRowContext(ctx, r.workflowSelect()+`
WHERE owner_user_id = `+r.bind(1)+` AND slash_name = `+r.bind(2),
		strings.TrimSpace(ownerUserID), strings.TrimSpace(slashName)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = r.loadGraph(ctx, &workflow); err != nil {
		return nil, err
	}
	return &workflow, nil
}

// Delete 删除 owner scope 内的 Workflow；不存在时返回 false。
func (r *Repository) Delete(
	ctx context.Context,
	ownerUserID string,
	workflowID string,
) (bool, error) {
	result, err := r.db.ExecContext(ctx, `
DELETE FROM workgraph_workflows
WHERE owner_user_id = `+r.bind(1)+` AND workflow_id = `+r.bind(2),
		strings.TrimSpace(ownerUserID), strings.TrimSpace(workflowID))
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (r *Repository) loadGraph(
	ctx context.Context,
	workflow *protocol.WorkGraphWorkflow,
) error {
	if workflow == nil {
		return nil
	}
	nodeRows, err := r.db.QueryContext(ctx, `
SELECT logical_key, source_work_item_id, role, kind, subject, objective,
       deliverable, `+r.dialect.JSONText("acceptance_criteria_json")+`,
       is_required, is_terminal, parent_logical_key, position
FROM workgraph_workflow_nodes
WHERE workflow_id = `+r.bind(1)+`
ORDER BY position ASC, logical_key ASC`, workflow.ID)
	if err != nil {
		return err
	}
	for nodeRows.Next() {
		var node protocol.WorkGraphWorkflowNode
		var acceptanceJSON string
		var parent sql.NullString
		if err = nodeRows.Scan(
			&node.LogicalKey,
			&node.SourceWorkItemID,
			&node.Role,
			&node.Kind,
			&node.Subject,
			&node.Objective,
			&node.Deliverable,
			&acceptanceJSON,
			&node.Required,
			&node.Terminal,
			&parent,
			&node.Position,
		); err != nil {
			_ = nodeRows.Close()
			return err
		}
		node.WorkflowID = workflow.ID
		node.ParentLogicalKey = parent.String
		if err = unmarshalJSON(acceptanceJSON, &node.AcceptanceCriteria); err != nil {
			_ = nodeRows.Close()
			return err
		}
		workflow.Nodes = append(workflow.Nodes, node)
	}
	if err = nodeRows.Close(); err != nil {
		return err
	}
	if err = nodeRows.Err(); err != nil {
		return err
	}

	dependencyRows, err := r.db.QueryContext(ctx, `
SELECT logical_key, depends_on_logical_key, dependency_kind
FROM workgraph_workflow_dependencies
WHERE workflow_id = `+r.bind(1)+`
ORDER BY logical_key ASC, depends_on_logical_key ASC`, workflow.ID)
	if err != nil {
		return err
	}
	defer dependencyRows.Close()
	for dependencyRows.Next() {
		dependency := protocol.WorkGraphWorkflowDependency{WorkflowID: workflow.ID}
		if err = dependencyRows.Scan(
			&dependency.LogicalKey,
			&dependency.DependsOnLogicalKey,
			&dependency.Kind,
		); err != nil {
			return err
		}
		workflow.Dependencies = append(workflow.Dependencies, dependency)
	}
	return dependencyRows.Err()
}

func (r *Repository) workflowSelect() string {
	return `SELECT workflow_id, owner_user_id, slash_name, title, description,
       source_execution_id, source_session_key, objective,
       ` + r.dialect.JSONText("completion_criteria_json") + `,
       version, created_at, updated_at
FROM workgraph_workflows`
}

type rowScanner interface {
	Scan(...any) error
}

func scanWorkflow(scanner rowScanner) (protocol.WorkGraphWorkflow, error) {
	var workflow protocol.WorkGraphWorkflow
	var description sql.NullString
	var criteriaJSON string
	err := scanner.Scan(
		&workflow.ID,
		&workflow.OwnerUserID,
		&workflow.SlashName,
		&workflow.Title,
		&description,
		&workflow.SourceExecutionID,
		&workflow.SourceSessionKey,
		&workflow.Objective,
		&criteriaJSON,
		&workflow.Version,
		&workflow.CreatedAt,
		&workflow.UpdatedAt,
	)
	if err != nil {
		return protocol.WorkGraphWorkflow{}, err
	}
	workflow.Description = description.String
	if err = unmarshalJSON(criteriaJSON, &workflow.CompletionCriteria); err != nil {
		return protocol.WorkGraphWorkflow{}, err
	}
	workflow.CreatedAt = workflow.CreatedAt.UTC()
	workflow.UpdatedAt = workflow.UpdatedAt.UTC()
	return workflow, nil
}

func (r *Repository) bind(index int) string {
	return r.dialect.Bind(index)
}

func (r *Repository) jsonBind(index int) string {
	return r.dialect.JSONValue(index)
}

func (r *Repository) timestamp(value time.Time) any {
	return r.dialect.TimestampValue(value.UTC())
}

func marshalJSON(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode workflow JSON: %w", err)
	}
	return string(payload), nil
}

func unmarshalJSON(raw string, target any) error {
	if strings.TrimSpace(raw) == "" {
		raw = "[]"
	}
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		return fmt.Errorf("decode workflow JSON: %w", err)
	}
	return nil
}

func nullString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}
