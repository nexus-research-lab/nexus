// INPUT: owner 作用域与待删除的规范化 Session keys。
// OUTPUT: 绑定任务、Goal、投递路由、Execution 与 runtime graph 的幂等级联清理。
// POS: 各领域删除操作复用的 Session 次级数据收口边界。
package deletion

import (
	"context"
	"database/sql"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

type sessionGoalCleaner interface {
	DeleteGoalsForSessions(context.Context, []string) (int, error)
}

type sessionTaskCleaner interface {
	DeleteTasksForSessions(context.Context, string, []string) error
	InvalidateTasksForDeletedSessions(context.Context, string, []string) error
}

// Coordinator 统一清理 Session 作用域的次级数据。
type Coordinator struct {
	db      *sql.DB
	dialect storage.SQLDialect
	goals   sessionGoalCleaner
	tasks   sessionTaskCleaner
}

// NewCoordinator 创建共享删除协调器。
func NewCoordinator(cfg config.Config, db *sql.DB) *Coordinator {
	return &Coordinator{
		db:      db,
		dialect: storage.NewSQLDialect(cfg.DatabaseDriver),
	}
}

// SetGoalCleaner 注入 Session 作用域 Goal 清理器。
func (c *Coordinator) SetGoalCleaner(cleaner sessionGoalCleaner) {
	if c != nil {
		c.goals = cleaner
	}
}

// SetTaskCleaner 注入绑定到 Session 的定时任务清理器。
func (c *Coordinator) SetTaskCleaner(cleaner sessionTaskCleaner) {
	if c != nil {
		c.tasks = cleaner
	}
}

// CleanupSessionReferences 删除不属于历史审计的数据；token usage、task run/event
// 与 ingress 历史明确保留，不参与该级联。
func (c *Coordinator) CleanupSessionReferences(
	ctx context.Context,
	ownerUserID string,
	sessionKeys []string,
) error {
	return c.cleanupSessionReferences(ctx, ownerUserID, sessionKeys, true)
}

// CleanupSessionReferencesPreservingTasks 清理已删除 Session 的可重建引用，并把引用
// 该 Session 的 Automation 任务停用、标记为等待重绑。Agent/Room 整体删除仍使用
// 完整级联入口，因为其任务本身也失去所属执行主体。
func (c *Coordinator) CleanupSessionReferencesPreservingTasks(
	ctx context.Context,
	ownerUserID string,
	sessionKeys []string,
) error {
	return c.cleanupSessionReferences(ctx, ownerUserID, sessionKeys, false)
}

func (c *Coordinator) cleanupSessionReferences(
	ctx context.Context,
	ownerUserID string,
	sessionKeys []string,
	deleteTasks bool,
) error {
	if c == nil || c.db == nil {
		return nil
	}
	ownerUserID = strings.TrimSpace(ownerUserID)
	sessionKeys = normalizedSessionKeys(sessionKeys)
	if ownerUserID == "" || len(sessionKeys) == 0 {
		return nil
	}
	if c.tasks != nil {
		if deleteTasks {
			if err := c.tasks.DeleteTasksForSessions(ctx, ownerUserID, sessionKeys); err != nil {
				return err
			}
		} else if err := c.tasks.InvalidateTasksForDeletedSessions(
			ctx,
			ownerUserID,
			sessionKeys,
		); err != nil {
			return err
		}
	}
	if c.goals != nil {
		if _, err := c.goals.DeleteGoalsForSessions(ctx, sessionKeys); err != nil {
			return err
		}
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = c.deleteSessionScopedRows(ctx, tx, ownerUserID, sessionKeys); err != nil {
		return err
	}
	return tx.Commit()
}

func (c *Coordinator) deleteSessionScopedRows(
	ctx context.Context,
	tx *sql.Tx,
	ownerUserID string,
	sessionKeys []string,
) error {
	ownerBind := c.dialect.Bind(1)
	keyBinds := make([]string, 0, len(sessionKeys))
	args := make([]any, 0, len(sessionKeys)+1)
	args = append(args, ownerUserID)
	for _, sessionKey := range sessionKeys {
		args = append(args, sessionKey)
		keyBinds = append(keyBinds, c.dialect.Bind(len(args)))
	}
	inKeys := strings.Join(keyBinds, ",")
	statements := []string{
		`DELETE FROM automation_delivery_routes
WHERE agent_id IN (SELECT id FROM agents WHERE owner_user_id = ` + ownerBind + `)
  AND session_key IN (` + inKeys + `)`,
		`DELETE FROM runtime_graph_artifact_refs
WHERE owner_user_id = ` + ownerBind + ` AND session_key IN (` + inKeys + `)`,
		`DELETE FROM runtime_graph_edge_runs
WHERE owner_user_id = ` + ownerBind + ` AND session_key IN (` + inKeys + `)`,
		`DELETE FROM runtime_graph_node_runs
WHERE owner_user_id = ` + ownerBind + ` AND session_key IN (` + inKeys + `)`,
		`DELETE FROM execution_plan_proposals
WHERE owner_user_id = ` + ownerBind + ` AND session_key IN (` + inKeys + `)`,
		`DELETE FROM executions
WHERE owner_user_id = ` + ownerBind + ` AND session_key IN (` + inKeys + `)`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement, args...); err != nil {
			return err
		}
	}
	return nil
}

func normalizedSessionKeys(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
