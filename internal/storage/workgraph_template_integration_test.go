// INPUT: Saved parameterized templates and sealed Plans on migrated SQLite/PostgreSQL.
// OUTPUT: Version/owner validation, transactional attribution, and per-version elapsed samples.
// POS: Template reuse statistics use committed domain state, never slash-preview counts.
package storage_test

import (
	"fmt"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	svc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	workflowsvc "github.com/nexus-research-lab/nexus/internal/service/workgraphworkflow"
	"github.com/nexus-research-lab/nexus/internal/storage"
	repo "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
	workflowrepo "github.com/nexus-research-lab/nexus/internal/storage/workgraphworkflow"
	"testing"
	"time"
)

func TestWorkGraphTemplateIntegration(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			cfg := integrationDatabase(t, driver)
			migrateIntegrationDatabase(t, cfg, 0)
			db := openIntegrationDatabase(t, cfg)
			templates := workflowrepo.NewRepository(cfg, db)
			repository := repo.NewSQLRepository(driver, db)
			service := svc.NewService(repository)
			templateService := workflowsvc.NewService(templates, service)
			service.SetWorkflowSourceValidator(templateService)
			template, err := templates.Create(t.Context(), protocol.WorkGraphWorkflow{ID: "template-inputs", OwnerUserID: "owner", SlashName: "inputs", Title: "Inputs", SourceExecutionID: "source", SourceSessionKey: "source-session", Objective: "calculate", Version: 1, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Parameters: []protocol.WorkGraphParameter{{Name: "quantity", Type: "number", Required: true}, {Name: "format", Type: "string", Default: "md"}}})
			if err != nil || len(template.Parameters) != 2 {
				t.Fatalf("persist template %+v %v", template, err)
			}
			doc := `nexus_plan: 1
operation: create
objective: Verify template inputs
completion_criteria: [verified]
workflow:
  id: template-inputs
  version: 1
  parameters: {quantity: 1.5}
items:
  - logical_key: calculate
    kind: produce
    subject: Calculate
    objective: Calculate
    deliverable: Evidence
    acceptance_criteria: [verified]
    required: true
    terminal: true
    output_scopes: ["semantic:calculate"]`
			actor := svc.ActorContext{OwnerUserID: "owner", SessionKey: "template-session", AgentID: "lead", Role: svc.ExecutionActorCoordinator, ScopeKind: protocol.ExecutionScopeDM, RootRoundID: "template-round", AgentRoundID: "template-agent-round"}
			wrong := actor
			wrong.OwnerUserID = "another"
			if _, err = service.PreparePlanExecution(t.Context(), wrong, svc.PreparePlanExecutionInput{CommandID: "cross-owner", PlanDocument: doc}); err == nil {
				t.Fatal("cross-owner provenance")
			}
			proposal, err := service.PreparePlanExecution(t.Context(), actor, svc.PreparePlanExecutionInput{CommandID: "seal-template", PlanDocument: doc})
			if err != nil {
				t.Fatal(err)
			}
			if proposal.Document.WorkflowSource.Parameters["format"] != "md" {
				t.Fatal("default not sealed")
			}
			stats, err := templates.Statistics(t.Context(), "owner")
			if err != nil || len(stats) != 0 {
				t.Fatal("preview counted as run")
			}
			materialized, err := service.MaterializePlanExecution(t.Context(), actor, svc.MaterializePlanExecutionInput{ProposalID: proposal.ID, ProposalDigest: proposal.ContentDigest})
			if err != nil || materialized.Outcome != svc.MutationApplied {
				t.Fatalf("materialization %+v %v", materialized, err)
			}
			if _, err = service.MaterializePlanExecution(t.Context(), actor, svc.MaterializePlanExecutionInput{ProposalID: proposal.ID, ProposalDigest: proposal.ContentDigest}); err != nil {
				t.Fatal(err)
			}
			stats, err = templates.Statistics(t.Context(), "owner")
			if err != nil || len(stats[template.ID]) != 1 || stats[template.ID][0].Runs != 1 || stats[template.ID][0].Active != 1 {
				t.Fatalf("counted once %+v %v", stats, err)
			}
			template.Version = 2
			template.Parameters[0].Type = "integer"
			template, err = templates.Update(t.Context(), *template)
			if err != nil || template.Parameters[0].Type != "integer" {
				t.Fatal("parameter revision not persisted", err)
			}
			next := actor
			next.SessionKey = "template-next"
			next.RootRoundID = "next-round"
			if _, err = service.PreparePlanExecution(t.Context(), next, svc.PreparePlanExecutionInput{CommandID: "stale-version", PlanDocument: doc}); err == nil {
				t.Fatal("stale saved version accepted")
			}
			// Exact timestamps exercise the aggregation, including closed vs completed semantics.
			dialect := storage.NewSQLDialect(driver)
			created := time.Now().UTC().Add(-2 * time.Minute)
			if _, err = db.ExecContext(t.Context(), `UPDATE executions SET status='completed',created_at=`+dialect.Bind(1)+`,completed_at=`+dialect.Bind(2)+` WHERE execution_id=`+dialect.Bind(3), dialect.TimestampValue(created), dialect.TimestampValue(created.Add(90*time.Second)), materialized.Snapshot.Execution.ID); err != nil {
				t.Fatal(err)
			}
			stats, err = templates.Statistics(t.Context(), "owner")
			if err != nil {
				t.Fatal(err)
			}
			stat := stats[template.ID][0]
			if stat.Version != 1 || stat.Runs != 1 || stat.Completed != 1 || stat.ElapsedSamples != 1 || stat.ElapsedMilliseconds != 90000 {
				t.Fatal(fmt.Sprintf("wrong version/timing %+v", stat))
			}
			other, _ := templates.Statistics(t.Context(), "other")
			if len(other) != 0 {
				t.Fatal("stats leaked across owners")
			}
		})
	}
}
