// INPUT: Frozen pre-164 database and a synthetic first Plan.
// OUTPUT: Historical responsibility facts without executing today's snapshot queries on an old schema.
// POS: Upgrade-test fixture only; production always migrates before opening the current repository.
package storage_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage"
	repo "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
	"strings"
)

type integrationPlanWriter interface {
	Create(context.Context, repo.CreateCommand) (*protocol.ExecutionSnapshot, error)
	WritePlan(context.Context, repo.WritePlanCommand) (*protocol.ExecutionSnapshot, error)
}
type legacyPlanWriter struct {
	*repo.Repository
	db     *sql.DB
	driver string
}

func (w legacyPlanWriter) WritePlan(ctx context.Context, c repo.WritePlanCommand) (*protocol.ExecutionSnapshot, error) {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	dialect := storage.NewSQLDialect(w.driver)
	insert := func(table, columns string, values ...any) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO "+table+" ("+columns+") VALUES ("+dialect.BindList(len(values))+")", values...)
		return err
	}
	p := c.Plan
	if err = insert("execution_plan_revisions", "plan_id,execution_id,revision,status,created_by_agent_id,revision_reason", p.ID, p.ExecutionID, p.Revision, p.Status, p.CreatedByAgentID, p.RevisionReason); err != nil {
		return nil, err
	}
	for _, work := range c.WorkItems {
		a, s := work.WorkItem, work.Spec
		if err = insert("execution_work_items", "work_item_id,execution_id,logical_key,kind", a.ID, a.ExecutionID, a.LogicalKey, a.Kind); err != nil {
			return nil, err
		}
		criteria, _ := json.Marshal(s.AcceptanceCriteria)
		if err = insert("execution_work_item_specs", "spec_id,work_item_id,execution_id,spec_version,subject,objective,deliverable,acceptance_criteria_json,spec_hash,created_by_agent_id", s.ID, s.WorkItemID, s.ExecutionID, s.Version, s.Subject, s.Objective, s.Deliverable, string(criteria), s.SpecHash, s.CreatedByAgentID); err != nil {
			return nil, err
		}
		if err = insert("execution_work_item_states", "work_item_id,execution_id,current_spec_id,status", a.ID, a.ExecutionID, s.ID, "open"); err != nil {
			return nil, err
		}
		if err = insert("execution_plan_items", "plan_id,execution_id,work_item_id,spec_id,is_required,is_terminal,position", p.ID, a.ExecutionID, a.ID, s.ID, work.Item.Required, work.Item.Terminal, work.Item.Position); err != nil {
			return nil, err
		}
		for _, claim := range work.OutputClaims {
			if err = insert("execution_plan_output_claims", "plan_id,execution_id,work_item_id,spec_id,claim_key,mode", p.ID, a.ExecutionID, a.ID, s.ID, claim.Scope, claim.Mode); err != nil {
				return nil, err
			}
		}
	}
	_, err = tx.ExecContext(ctx, "UPDATE executions SET version=2 WHERE execution_id="+dialect.Bind(1), p.ExecutionID)
	if err != nil {
		return nil, err
	}
	if err = insert("execution_events", strings.Join([]string{"event_id", "execution_id", "sequence", "command_id", "event_type", "entity_type", "entity_id", "entity_version", "actor_kind", "actor_id", "plan_id"}, ","), c.Meta.EventID, p.ExecutionID, 2, c.Meta.CommandID, protocol.ExecutionEventPlanActivated, protocol.ExecutionEntityPlan, p.ID, 1, c.Meta.ActorKind, c.Meta.ActorID, p.ID); err != nil {
		return nil, err
	}
	return nil, tx.Commit()
}
