// workgraph_integration_test.go 覆盖 WorkGraph 数据库、检查点、续跑、派发策略、模板与升级集成测试。
package storage_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	svc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	workflowsvc "github.com/nexus-research-lab/nexus/internal/service/workgraphworkflow"
	"github.com/nexus-research-lab/nexus/internal/storage"
	orchestrationrepo "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
	workflowrepo "github.com/nexus-research-lab/nexus/internal/storage/workgraphworkflow"
	workspace "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type integrationPlanWriter interface {
	Create(context.Context, orchestrationrepo.CreateCommand) (*protocol.ExecutionSnapshot, error)
	WritePlan(context.Context, orchestrationrepo.WritePlanCommand) (*protocol.ExecutionSnapshot, error)
}
type legacyPlanWriter struct {
	*orchestrationrepo.Repository
	db     *sql.DB
	driver string
}

func (w legacyPlanWriter) WritePlan(ctx context.Context, c orchestrationrepo.WritePlanCommand) (*protocol.ExecutionSnapshot, error) {
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

func TestWorkGraphDatabaseIntegration(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			cfg := integrationDatabase(t, driver)
			migrateIntegrationDatabase(t, cfg, 0)
			db, otherDB := openIntegrationDatabase(t, cfg), openIntegrationDatabase(t, cfg)
			repo := orchestrationrepo.NewSQLRepository(driver, db)
			other := orchestrationrepo.NewSQLRepository(driver, otherDB)
			t.Run("batch_rollback_and_receipt", func(t *testing.T) { checkBatchRollback(t, db, driver, repo) })
			t.Run("concurrent_same_request", func(t *testing.T) { checkBatchCompetition(t, db, driver, repo, other, true) })
			t.Run("concurrent_different_requests", func(t *testing.T) { checkBatchCompetition(t, db, driver, repo, other, false) })
			t.Run("draft_revision_and_editor", func(t *testing.T) { checkDraftCompetition(t, cfg, db, otherDB) })
			t.Run("runtime_window_and_summary", func(t *testing.T) { checkWorkGraphReadWindow(t, repo) })
		})
	}
}

func checkBatchRollback(t *testing.T, db *sql.DB, driver string, repo *orchestrationrepo.Repository) {
	t.Helper()
	batch := prepareIntegrationBatch(t, repo, "rollback")
	invalid := batch
	invalid.Assignments = append([]orchestrationrepo.AssignCommand(nil), batch.Assignments...)
	// Fail at the second item's audit INSERT, after both responsibility chains
	// and the first audit event have already been written inside the transaction.
	invalid.Assignments[1].Meta.EventID = invalid.Assignments[0].Meta.EventID
	if _, err := repo.AssignBatch(t.Context(), invalid); err == nil {
		t.Fatal("duplicate audit event committed")
	}
	assertIntegrationBatchState(t, db, driver, repo, batch.ExecutionID, 2, 0, 2)
	if receipt, err := repo.FindAssignmentBatchReceipt(t.Context(), batch.ExecutionID, batch.Meta.CommandID); err != nil || receipt != nil {
		t.Fatalf("rolled-back receipt = %+v, %v", receipt, err)
	}
	for range 2 {
		if _, err := repo.AssignBatch(t.Context(), batch); err != nil {
			t.Fatal(err)
		}
		assertIntegrationBatchState(t, db, driver, repo, batch.ExecutionID, 3, 2, 5)
	}
	for _, change := range []string{"digest", "actor"} {
		conflict := batch
		if change == "digest" {
			conflict.RequestDigest = "different-intent"
		} else {
			conflict.Meta.ActorID = "other-actor"
		}
		if _, err := repo.AssignBatch(t.Context(), conflict); !errors.Is(err, orchestrationrepo.ErrCommandConflict) {
			t.Fatalf("changed %s replay = %v", change, err)
		}
	}
	assertIntegrationBatchState(t, db, driver, repo, batch.ExecutionID, 3, 2, 5)
}

func checkBatchCompetition(t *testing.T, db *sql.DB, driver string, repo, other *orchestrationrepo.Repository, sameRequest bool) {
	t.Helper()
	suffix := fmt.Sprintf("competition-%t", sameRequest)
	batch := prepareIntegrationBatch(t, repo, suffix)
	contender := batch
	if !sameRequest {
		contender.Meta = integrationCommandMeta("other-" + suffix)
		contender.RequestDigest = "different-intent"
	}
	results := raceIntegrationCalls(
		func() (*protocol.ExecutionSnapshot, error) { return repo.AssignBatch(t.Context(), batch) },
		func() (*protocol.ExecutionSnapshot, error) { return other.AssignBatch(t.Context(), contender) },
	)
	successes := 0
	for i, result := range results {
		if result.err == nil {
			successes++
			if result.value == nil || result.value.Execution.Version != 3 || len(result.value.Assignments) != 2 {
				t.Fatalf("concurrent receipt %d = %+v", i, result.value)
			}
		} else if sameRequest || !errors.Is(result.err, orchestrationrepo.ErrVersionConflict) {
			t.Fatalf("concurrent request %d: %v", i, result.err)
		}
	}
	wantSuccesses := 1
	if sameRequest {
		wantSuccesses = 2
	}
	if successes != wantSuccesses {
		t.Fatalf("successful callers = %d, want %d", successes, wantSuccesses)
	}
	assertIntegrationBatchState(t, db, driver, repo, batch.ExecutionID, 3, 2, 5)
	for i, command := range []orchestrationrepo.AssignBatchCommand{batch, contender} {
		receipt, err := repo.FindAssignmentBatchReceipt(t.Context(), command.ExecutionID, command.Meta.CommandID)
		if err != nil || (receipt != nil) != (results[i].err == nil) {
			t.Fatalf("caller %d durable receipt = %+v, %v", i, receipt, err)
		}
		if receipt != nil && (receipt.EntityVersion != 3 || receipt.Payload["request_digest"] != command.RequestDigest) {
			t.Fatalf("caller %d receipt lost intent/revision: %+v", i, receipt)
		}
	}
}

func assertIntegrationBatchState(t *testing.T, db *sql.DB, driver string, repo *orchestrationrepo.Repository, executionID string, version int64, chains, events int) {
	t.Helper()
	snapshot, err := repo.GetSnapshot(t.Context(), executionID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Execution.Version != version || len(snapshot.Assignments) != chains || len(snapshot.Attempts) != chains || len(snapshot.Dispatches) != chains {
		t.Fatalf("batch state: version=%d assignments=%d attempts=%d dispatches=%d; want %d/%d/%d/%d",
			snapshot.Execution.Version, len(snapshot.Assignments), len(snapshot.Attempts), len(snapshot.Dispatches), version, chains, chains, chains)
	}
	var count, minSequence, maxSequence int
	if err := db.QueryRowContext(t.Context(), `SELECT COUNT(*), MIN(sequence), MAX(sequence) FROM execution_events WHERE execution_id = `+storage.NewSQLDialect(driver).Bind(1), executionID).Scan(&count, &minSequence, &maxSequence); err != nil {
		t.Fatal(err)
	}
	if count != events || minSequence != 1 || maxSequence != events {
		t.Fatalf("event sequence: count=%d min=%d max=%d; want %d/1/%d", count, minSequence, maxSequence, events, events)
	}
}

func checkDraftCompetition(t *testing.T, cfg config.Config, db, otherDB *sql.DB) {
	t.Helper()
	repo, other := workflowrepo.NewRepository(cfg, db), workflowrepo.NewRepository(cfg, otherDB)
	now := time.Now().UTC().Truncate(time.Microsecond)
	preview := protocol.WorkGraphWorkflowPreview{PreviewID: "preview-concurrent", Title: "initial"}
	draft, err := repo.CreateDraft(t.Context(), protocol.WorkGraphWorkflowDraft{
		PreviewID: preview.PreviewID, OwnerUserID: "draft-owner", SourceSessionKey: "draft-session", SourceExecutionID: "draft-execution",
		HeadRevision: 1, SelectedRevision: 1, ExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now,
		Versions: []protocol.WorkGraphWorkflowPreviewVersion{{Revision: 1, Preview: preview, CreatedAt: now}},
	})
	if err != nil {
		t.Fatal(err)
	}
	appendVersion := func(repository *workflowrepo.Repository, title string) func() (*protocol.WorkGraphWorkflowDraft, error) {
		return func() (*protocol.WorkGraphWorkflowDraft, error) {
			next := preview
			next.Title = title
			return repository.AppendDraftVersion(t.Context(), draft.OwnerUserID, draft.PreviewID, 1, next, now, now.Add(time.Hour))
		}
	}
	results := raceIntegrationCalls(appendVersion(repo, "first contender"), appendVersion(other, "second contender"))
	successes, winner := 0, ""
	for _, result := range results {
		if result.err == nil {
			successes++
			if result.value == nil || result.value.HeadRevision != 2 || result.value.SelectedRevision != 2 || len(result.value.Versions) != 2 {
				t.Fatalf("draft write receipt = %+v", result.value)
			}
			winner = result.value.Preview.Title
		} else if result.err.Error() != "WorkGraph Draft revision changed" {
			t.Fatalf("draft contender: %v", result.err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful draft writers = %d, want 1", successes)
	}
	current, err := other.GetDraftByID(t.Context(), draft.OwnerUserID, draft.PreviewID)
	if err != nil || current == nil || current.HeadRevision != 2 || len(current.Versions) != 2 || current.Preview.Title != winner || current.Versions[0].Preview.Title != "initial" {
		t.Fatalf("durable draft = %+v, %v", current, err)
	}
	if hidden, err := repo.GetDraftByID(t.Context(), "other-owner", draft.PreviewID); err != nil || hidden != nil {
		t.Fatalf("draft owner isolation = %+v, %v", hidden, err)
	}
	bind := func(repository *workflowrepo.Repository, editor string) func() (*protocol.WorkGraphWorkflowDraft, error) {
		return func() (*protocol.WorkGraphWorkflowDraft, error) {
			return repository.BindDraftEditor(t.Context(), draft.OwnerUserID, draft.PreviewID, editor, "editor-agent", "session-"+editor, 0, now)
		}
	}
	bindings := raceIntegrationCalls(bind(repo, "editor-a"), bind(other, "editor-b"))
	for _, result := range bindings {
		if result.err != nil || result.value == nil || result.value.EditorID == "" {
			t.Fatalf("editor binding = %+v, %v", result.value, result.err)
		}
	}
	if bindings[0].value.EditorID != bindings[1].value.EditorID {
		t.Fatalf("concurrent openers received different editors: %s / %s", bindings[0].value.EditorID, bindings[1].value.EditorID)
	}
	oldEditor := bindings[0].value.EditorID
	if err := repo.ClearDraftEditor(t.Context(), draft.OwnerUserID, draft.PreviewID, oldEditor); err != nil {
		t.Fatal(err)
	}
	if old, err := other.GetDraftByEditorID(t.Context(), draft.OwnerUserID, oldEditor); err != nil || old != nil {
		t.Fatalf("discarded editor still authorized: %+v, %v", old, err)
	}
	if _, err := bind(other, "replacement-editor")(); err != nil {
		t.Fatal(err)
	}
	if err := repo.ClearDraftEditor(t.Context(), draft.OwnerUserID, draft.PreviewID, oldEditor); err != nil {
		t.Fatal(err)
	}
	if current, err := repo.GetDraftByID(t.Context(), draft.OwnerUserID, draft.PreviewID); err != nil || current == nil || current.EditorID != "replacement-editor" {
		t.Fatalf("stale discard revoked replacement: %+v, %v", current, err)
	}
}

func checkWorkGraphReadWindow(t *testing.T, repo *orchestrationrepo.Repository) {
	t.Helper()
	batch := prepareIntegrationBatch(t, repo, "read-window")
	execution, err := repo.Get(t.Context(), batch.ExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	count := protocol.ExecutionRuntimeGraphNodeProjectionLimit + 20
	for i := range count {
		node := integrationRuntimeNode(*execution, fmt.Sprintf("detail-%04d", i), now.Add(time.Duration(i)*time.Second))
		node.WorkGraphReadClass = "detail"
		if i == 0 {
			node.Status, node.Failed = protocol.ExecutionRuntimeNodeFailed, true
		}
		if err := repo.UpsertRuntimeGraphNode(t.Context(), node); err != nil {
			t.Fatal(err)
		}
	}
	graph, err := repo.GetWorkGraphRuntimeGraph(t.Context(), execution.OwnerUserID, execution.SessionKey, execution.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if graph.NodeTotal != count || len(graph.Nodes) != protocol.ExecutionRuntimeGraphNodeProjectionLimit+1 || graph.HistoryPending {
		t.Fatalf("runtime window: total=%d retained=%d pending=%t", graph.NodeTotal, len(graph.Nodes), graph.HistoryPending)
	}
	foundFailure, foundEvicted := false, false
	for _, node := range graph.Nodes {
		foundFailure = foundFailure || node.ID == "detail-0000"
		foundEvicted = foundEvicted || node.ID == "detail-0001"
	}
	if !foundFailure || foundEvicted {
		t.Fatalf("runtime window lost failure or retained old detail: failure=%t old_detail=%t", foundFailure, foundEvicted)
	}
	summaries, err := repo.ListManagedSummaries(t.Context(), execution.OwnerUserID, execution.SessionKey, 10)
	if err != nil || len(summaries) != 1 || summaries[0].ID != execution.ID || summaries[0].WorkItemCount != 2 || summaries[0].Version != 2 {
		t.Fatalf("managed summary = %+v, %v", summaries, err)
	}
	if summaries, err := repo.ListManagedSummaries(t.Context(), "other-owner", execution.SessionKey, 10); err != nil || len(summaries) != 0 {
		t.Fatalf("summary owner isolation = %+v, %v", summaries, err)
	}
}

type integrationCallResult[T any] struct {
	value T
	err   error
}

// Use distinct DB handles even on SQLite so the database, not one sql.DB's
// connection limit, arbitrates competing writers. Never retry a failed call.
func raceIntegrationCalls[T any](calls ...func() (T, error)) []integrationCallResult[T] {
	start := make(chan struct{})
	results := make([]integrationCallResult[T], len(calls))
	var wg sync.WaitGroup
	for i, call := range calls {
		wg.Go(func() {
			<-start
			results[i].value, results[i].err = call()
		})
	}
	close(start)
	wg.Wait()
	return results
}

func integrationCommandMeta(suffix string) orchestrationrepo.CommandMeta {
	return orchestrationrepo.CommandMeta{CommandID: "command-" + suffix, EventID: "event-" + suffix, ActorKind: protocol.ExecutionActorAgent, ActorID: "lead"}
}

func prepareIntegrationBatch(t testing.TB, repo integrationPlanWriter, suffix string) orchestrationrepo.AssignBatchCommand {
	return prepareIntegrationBatchScope(t, repo, suffix, protocol.ExecutionScopeDM)
}

func prepareIntegrationBatchScope(t testing.TB, repo integrationPlanWriter, suffix string, scope protocol.ExecutionScopeKind, dependencies ...bool) orchestrationrepo.AssignBatchCommand {
	t.Helper()
	execution := protocol.Execution{
		ID: "execution-" + suffix, OwnerUserID: "workgraph-owner", SessionKey: "session-" + suffix,
		ScopeKind: scope, CoordinatorAgentID: "lead", Origin: protocol.ExecutionOriginUserRequest,
		Objective: "verify " + suffix, CompletionCriteria: []string{"verified"}, Status: protocol.ExecutionStatusActive,
	}
	if scope == protocol.ExecutionScopeRoom {
		execution.RoomID = "room-" + suffix
		execution.ConversationID = "conversation-" + suffix
	}
	if _, err := repo.Create(t.Context(), orchestrationrepo.CreateCommand{Execution: execution, Meta: integrationCommandMeta("create-" + suffix)}); err != nil {
		t.Fatal(err)
	}
	planID := "plan-" + suffix
	plan := orchestrationrepo.WritePlanCommand{
		ExecutionID: execution.ID, ExpectedExecutionVersion: 1, Meta: integrationCommandMeta("plan-" + suffix),
		Plan: protocol.ExecutionPlanRevision{ID: planID, ExecutionID: execution.ID, Revision: 1, Status: protocol.PlanRevisionStatusActive, CreatedByAgentID: "lead", RevisionReason: "integration"},
	}
	batch := orchestrationrepo.AssignBatchCommand{ExecutionID: execution.ID, ExpectedExecutionVersion: 2, RequestDigest: "intent-" + suffix, Meta: integrationCommandMeta("batch-" + suffix)}
	for i := range 2 {
		id := fmt.Sprintf("%s-%d", suffix, i)
		workID, specID := "work-"+id, "spec-"+id
		plan.WorkItems = append(plan.WorkItems, orchestrationrepo.PlanWorkItem{
			WorkItem: protocol.WorkItem{ID: workID, ExecutionID: execution.ID, LogicalKey: workID, Kind: protocol.WorkItemKindProduce},
			Spec: protocol.WorkItemSpec{ID: specID, WorkItemID: workID, ExecutionID: execution.ID, Version: 1,
				Subject: id, Objective: "deliver " + id, Deliverable: "evidence", AcceptanceCriteria: []string{"verified"}, SpecHash: "hash-" + id, CreatedByAgentID: "lead"},
			State:        protocol.WorkItemState{WorkItemID: workID, ExecutionID: execution.ID, CurrentSpecID: specID, Status: protocol.WorkItemStatusOpen, Version: 1},
			Item:         protocol.ExecutionPlanItem{PlanID: planID, ExecutionID: execution.ID, WorkItemID: workID, SpecID: specID, Required: true, Position: i},
			OutputClaims: []protocol.ExecutionPlanOutputClaim{{PlanID: planID, ExecutionID: execution.ID, WorkItemID: workID, SpecID: specID, Scope: "dir:output/" + id, Mode: protocol.WorkOutputScopeExclusive}},
		})
		owner, assignmentID := "agent-"+id, "assignment-"+id
		batch.Assignments = append(batch.Assignments, orchestrationrepo.AssignCommand{
			ExpectedExecutionVersion: 2, Meta: integrationCommandMeta("assign-" + id),
			Assignment: protocol.WorkAssignment{ID: assignmentID, ExecutionID: execution.ID, PlanID: planID, WorkItemID: workID, SpecID: specID,
				OwnerAgentID: owner, AssignedByAgentID: "lead", ReturnToAgentID: "lead", Strategy: protocol.AssignmentStrategyRoomMember, Status: protocol.WorkAssignmentStatusAssigned},
			RootAttempt: &protocol.WorkAttempt{ID: "attempt-" + id, ExecutionID: execution.ID, PlanID: planID, WorkItemID: workID, SpecID: specID, AssignmentID: assignmentID,
				ExecutorKind: protocol.AttemptExecutorAgent, ExecutorAgentID: owner, Status: protocol.WorkAttemptStatusPending},
			Dispatch: &protocol.ExecutionDispatch{ID: "dispatch-" + id, DedupeKey: "dedupe-" + id, TargetAgentID: owner,
				Kind: protocol.ExecutionDispatchRoomDirected, Status: protocol.ExecutionDispatchStatusPending, Instruction: "deliver evidence"},
		})
	}
	if len(dependencies) > 0 && dependencies[0] {
		plan.Dependencies = []protocol.ExecutionPlanDependency{{PlanID: planID, ExecutionID: execution.ID, WorkItemID: plan.WorkItems[1].WorkItem.ID, DependsOnWorkItemID: plan.WorkItems[0].WorkItem.ID, Kind: protocol.WorkDependencyHard}}
	}
	if _, err := repo.WritePlan(t.Context(), plan); err != nil {
		t.Fatal(err)
	}
	return batch
}

func integrationRuntimeNode(e protocol.Execution, id string, now time.Time) protocol.ExecutionRuntimeNodeRun {
	return protocol.ExecutionRuntimeNodeRun{
		ID: id, GraphID: "execution:" + e.ID, OwnerUserID: e.OwnerUserID, SessionKey: e.SessionKey, ExecutionID: e.ID,
		Kind: protocol.ExecutionRuntimeNodeTool, SubjectID: "subject-" + id, RootRoundID: "round-" + e.ID,
		RuntimeRoundID: "round-" + e.ID, AgentRoundID: "agent-round-" + e.ID, AgentID: "lead",
		Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now, UpdatedAt: now,
	}
}

func TestWorkGraphCheckpointIntegration(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			cfg := integrationDatabase(t, driver)
			migrateIntegrationDatabase(t, cfg, 0)
			repository := orchestrationrepo.NewSQLRepository(driver, openIntegrationDatabase(t, cfg))
			service := svc.NewService(repository)
			actor, _ := runtimeResultFixture(t, repository, "checkpoint")
			before, _ := repository.GetSnapshot(t.Context(), actor.ExecutionID)
			input := svc.RecordCheckpointInput{ExecutionID: actor.ExecutionID, CommandID: "checkpoint-one", SnapshotRevision: before.Execution.Version, Data: protocol.WorkCheckpointData{Summary: "draft prepared", NextStep: "reconcile receipt before publish", ArtifactRefs: []string{"fixture:draft"}, Effects: []protocol.WorkCheckpointEffect{{Operation: "publish", Outcome: "unknown"}}}}
			saved, err := service.RecordCheckpoint(t.Context(), actor, input)
			if err != nil || saved.Outcome != svc.MutationApplied {
				t.Fatalf("checkpoint %+v %v", saved, err)
			}
			after := saved.Snapshot
			if len(after.Checkpoints) != 1 || len(after.Submissions) != 0 || len(after.Acceptances) != 0 || !reflect.DeepEqual(before.ReadyWorkItemIDs, after.ReadyWorkItemIDs) {
				t.Fatal("checkpoint changed completion or readiness")
			}
			input.SnapshotRevision = after.Execution.Version
			repeated, err := service.RecordCheckpoint(t.Context(), actor, input)
			if err != nil || repeated.Outcome == svc.MutationRejected || len(repeated.Snapshot.Checkpoints) != 1 {
				t.Fatalf("repeated %+v %v", repeated, err)
			}
			input.Data.Summary = "changed intent"
			rejected, err := service.RecordCheckpoint(t.Context(), actor, input)
			if err != nil || rejected.Outcome != svc.MutationRejected {
				t.Fatalf("conflicting intent %+v %v", rejected, err)
			}
			input.CommandID = "bad-receipt"
			input.Data.Effects[0].Outcome = "confirmed"
			rejected, err = service.RecordCheckpoint(t.Context(), actor, input)
			if err != nil || rejected.Outcome != svc.MutationRejected {
				t.Fatal("unproven receipt accepted")
			}
			coordinator := actor
			coordinator.WorkBinding = nil
			coordinator.AgentID = "lead"
			coordinator.Role = svc.ExecutionActorCoordinator
			if err = service.ActivateRuntimeCoordination(t.Context(), coordinator, after); err != nil {
				t.Fatal(err)
			}
			service.SetAssignmentTargetAuthorizer(policyTargetAuthorizer{})
			takeover, err := service.TakeOverWork(t.Context(), coordinator, svc.TakeOverWorkInput{ExecutionID: actor.ExecutionID, SnapshotRevision: after.Execution.Version, CommandID: "takeover-checkpoint", WorkItemID: actor.WorkBinding.WorkItemID, TargetAgentID: "replacement", ReturnToAgentID: "lead", Reason: "explicit recovery after inspecting checkpoint"})
			if err != nil || takeover.Outcome != svc.MutationApplied {
				t.Fatalf("takeover %+v %v", takeover, err)
			}
			current, _ := repository.GetSnapshot(t.Context(), actor.ExecutionID)
			if len(current.Checkpoints) != 1 || current.Checkpoints[0].AttemptID != actor.WorkBinding.AttemptID || len(current.Submissions) != 0 {
				t.Fatal("takeover lost evidence or invented delivery")
			}
			input.SnapshotRevision = current.Execution.Version
			input.CommandID = "stale-attempt"
			input.Data.Effects[0].Outcome = "unknown"
			stale, err := service.RecordCheckpoint(t.Context(), actor, input)
			if err == nil && stale.Outcome != svc.MutationRejected {
				t.Fatal("old owner wrote after takeover")
			}
			// A fresh service and queue-bound successor see the same immutable checkpoint.
			var assignment protocol.WorkAssignment
			var attempt protocol.WorkAttempt
			for _, a := range current.Assignments {
				if a.OwnerAgentID == "replacement" {
					assignment = a
				}
			}
			for _, a := range current.Attempts {
				if a.AssignmentID == assignment.ID {
					attempt = a
				}
			}
			successor := actor
			successor.AgentID = "replacement"
			successor.WorkBinding = &protocol.ExecutionWorkBinding{ExecutionID: actor.ExecutionID, PlanID: assignment.PlanID, WorkItemID: assignment.WorkItemID, SpecID: assignment.SpecID, AssignmentID: assignment.ID, AttemptID: attempt.ID, DispatchID: attempt.DispatchID}
			resumed, err := svc.NewService(repository).GetSnapshot(t.Context(), successor, actor.ExecutionID)
			if err != nil || len(resumed.Checkpoints) != 1 || resumed.Checkpoints[0].Effects[0].Outcome != "unknown" {
				t.Fatalf("resume %+v %v", resumed, err)
			}
		})
	}
}

type continuationDelivery struct {
	policyAdmission
	binding         *protocol.ExecutionWorkBinding
	target          string
	continuationErr error
}

func (*continuationDelivery) ContinuationPauseRevision(context.Context, protocol.Execution, string) (int64, error) {
	return 3, nil
}
func (p *continuationDelivery) QueueContinuedWork(_ context.Context, _ protocol.Execution, b protocol.ExecutionWorkBinding, _, key string, revision int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.continuationErr != nil {
		return p.continuationErr
	}
	p.binding = &b
	p.receipts[key] = true
	return nil
}
func (p *continuationDelivery) DeliverExecutionDispatch(_ context.Context, d svc.ExecutionDispatchDelivery) (svc.ExecutionDispatchReceipt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.continuationErr != nil {
		return svc.ExecutionDispatchReceipt{}, svc.PermanentExecutionDispatchDelivery(p.continuationErr)
	}
	p.binding = &d.Binding
	p.target = d.TargetAgentID
	p.receipts[d.DispatchDedupeKey] = true
	if !d.UserContinuation || d.ContinuationPauseRevision != 3 {
		panic("missing explicit continuation admission")
	}
	return svc.ExecutionDispatchReceipt{QueueItemID: "continued"}, nil
}

func TestWorkGraphSameOwnerContinuationIntegration(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			cfg := integrationDatabase(t, driver)
			migrateIntegrationDatabase(t, cfg, 0)
			db := openIntegrationDatabase(t, cfg)
			repository := orchestrationrepo.NewSQLRepository(driver, db)
			for _, scope := range []protocol.ExecutionScopeKind{protocol.ExecutionScopeDM, protocol.ExecutionScopeRoom} {
				t.Run(string(scope), func(t *testing.T) {
					batch := prepareIntegrationBatchScope(t, repository, "continue-"+string(scope), scope)
					assignment := batch.Assignments[0]
					if scope == protocol.ExecutionScopeDM {
						assignment.Assignment.OwnerAgentID = "lead"
						assignment.Assignment.Strategy = protocol.AssignmentStrategySelf
						assignment.RootAttempt.ExecutorAgentID = "lead"
						assignment.Dispatch = nil
					}
					snapshot, err := repository.Assign(t.Context(), assignment)
					if err != nil {
						t.Fatal(err)
					}
					old := snapshot.Attempts[0]
					old.Status = protocol.WorkAttemptStatusInterrupted
					old.FailureReason = protocol.InterruptHostShutdown
					snapshot, err = repository.FinishAttempt(t.Context(), orchestrationrepo.FinishAttemptCommand{ExpectedExecutionVersion: snapshot.Execution.Version, ExpectedAttemptVersion: old.Version, Attempt: old, Meta: integrationCommandMeta("stop-" + string(scope))})
					if err != nil {
						t.Fatal(err)
					}
					delivery := &continuationDelivery{policyAdmission: policyAdmission{paused: true, receipts: map[string]bool{}}}
					service := svc.NewService(repository)
					service.SetAssignmentTargetAuthorizer(policyTargetAuthorizer{})
					service.SetScheduledWorkConsumer(delivery)
					input := protocol.RequestExecutionRecovery{SessionKey: snapshot.Execution.SessionKey, RequestID: "continue", WorkItemID: old.WorkItemID, AttemptID: old.ID}
					var wg sync.WaitGroup
					for range 4 {
						wg.Go(func() {
							_, _ = service.RequestWorkRecovery(t.Context(), snapshot.Execution.OwnerUserID, snapshot.Execution.ID, input)
						})
					}
					wg.Wait()
					receipt, err := service.GetWorkRecoveryReceipt(t.Context(), snapshot.Execution.OwnerUserID, snapshot.Execution.ID, input.SessionKey, input.RequestID)
					if err != nil || !receipt.Queued {
						t.Fatalf("receipt %+v %v", receipt, err)
					}
					current, err := repository.GetSnapshot(t.Context(), snapshot.Execution.ID)
					if err != nil {
						t.Fatal(err)
					}
					if len(current.Assignments) != 1 || len(current.Attempts) != 2 || len(current.Submissions) != 0 || len(current.Acceptances) != 0 {
						t.Fatalf("unexpected projection: assignments=%d attempts=%d submissions=%d acceptances=%d", len(current.Assignments), len(current.Attempts), len(current.Submissions), len(current.Acceptances))
					}
					var replacement protocol.WorkAssignment
					for _, a := range current.Assignments {
						if a.ID != assignment.Assignment.ID {
							replacement = a
						}
					}
					if replacement.OwnerAgentID != assignment.Assignment.OwnerAgentID || replacement.ReturnToAgentID != assignment.Assignment.ReturnToAgentID {
						t.Fatal("continuation changed owner/reviewer")
					}
					// A fresh service drains the durable outbox, even though automatic work remains paused.
					restarted := svc.NewService(repository)
					restarted.SetAssignmentTargetAuthorizer(policyTargetAuthorizer{})
					restarted.SetScheduledWorkConsumer(delivery)
					restarted.SetExecutionDispatchConsumer(delivery)
					if scope == protocol.ExecutionScopeDM {
						_, err = restarted.ReconcileDispatchPolicies(t.Context(), 32)
					} else {
						_, err = restarted.DispatchPending(t.Context(), "restart-worker", 32)
					}
					if err != nil || delivery.binding == nil || delivery.binding.AttemptID == old.ID || delivery.binding.WorkItemID != old.WorkItemID {
						t.Fatalf("restart delivery %+v %v", delivery.binding, err)
					}
					if scope == protocol.ExecutionScopeRoom && delivery.target != assignment.Assignment.OwnerAgentID {
						t.Fatal("delivered to coordinator instead of original worker")
					}
					input.RequestID = "different-click"
					if _, err = restarted.RequestWorkRecovery(t.Context(), snapshot.Execution.OwnerUserID, snapshot.Execution.ID, input); err == nil {
						t.Fatal("old attempt admitted twice")
					}
					{
						current, _ = repository.GetSnapshot(t.Context(), snapshot.Execution.ID)
						var stopped protocol.WorkAttempt
						for _, candidate := range current.Attempts {
							if candidate.ID == delivery.binding.AttemptID {
								stopped = candidate
							}
						}
						stopped.Status = protocol.WorkAttemptStatusInterrupted
						_, err = repository.FinishAttempt(t.Context(), orchestrationrepo.FinishAttemptCommand{ExpectedExecutionVersion: current.Execution.Version, ExpectedAttemptVersion: stopped.Version, Attempt: stopped, Meta: integrationCommandMeta("second-stop-" + string(scope))})
						if err != nil {
							t.Fatal(err)
						}
						input.AttemptID, input.RequestID = stopped.ID, "continue-before-later-stop"
						if _, err = restarted.RequestWorkRecovery(t.Context(), snapshot.Execution.OwnerUserID, snapshot.Execution.ID, input); err != nil {
							t.Fatal(err)
						}
						delivery.continuationErr = workspace.ErrContinuationStopped
						if scope == protocol.ExecutionScopeDM {
							_, err = restarted.ReconcileDispatchPolicies(t.Context(), 32)
						} else {
							_, err = restarted.DispatchPending(t.Context(), "late-stop-worker", 32)
						}
						if err != nil {
							t.Fatal(err)
						}
						current, _ = repository.GetSnapshot(t.Context(), snapshot.Execution.ID)
						for _, candidate := range current.Attempts {
							if candidate.ID != old.ID && candidate.ID != stopped.ID {
								if candidate.Status != protocol.WorkAttemptStatusInterrupted {
									t.Fatalf("late stop stranded attempt: %+v", candidate)
								}
								input.AttemptID = candidate.ID
							}
						}
						input.RequestID = "fresh-explicit-continuation"
						if _, err = restarted.RequestWorkRecovery(t.Context(), snapshot.Execution.OwnerUserID, snapshot.Execution.ID, input); err != nil {
							t.Fatal(err)
						}
					}
				})
			}
		})
	}
}

type policyAdmission struct {
	mu       sync.Mutex
	paused   bool
	receipts map[string]bool
}

func (p *policyAdmission) AutomaticDispatchAllowed(context.Context, protocol.Execution, string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return !p.paused, nil
}
func (p *policyAdmission) QueueScheduledWork(_ context.Context, _ protocol.Execution, _ protocol.ExecutionWorkBinding, _, key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.receipts[key] = true
	return nil
}

type policyTargetAuthorizer struct{}

func (policyTargetAuthorizer) AuthorizeAssignmentTarget(context.Context, svc.AssignmentTargetRequest) error {
	return nil
}

func TestWorkGraphDispatchPolicyIntegration(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			cfg := integrationDatabase(t, driver)
			migrateIntegrationDatabase(t, cfg, 0)
			db := openIntegrationDatabase(t, cfg)
			repository := orchestrationrepo.NewSQLRepository(driver, db)
			for _, scope := range []protocol.ExecutionScopeKind{protocol.ExecutionScopeDM, protocol.ExecutionScopeRoom} {
				t.Run(string(scope), func(t *testing.T) {
					batch := prepareIntegrationBatchScope(t, repository, "policy-"+string(scope), scope, true)
					snapshot, _ := repository.GetSnapshot(t.Context(), batch.ExecutionID)
					actor := svc.ActorContext{OwnerUserID: snapshot.Execution.OwnerUserID, SessionKey: snapshot.Execution.SessionKey, AgentID: "lead", ScopeKind: scope, RoomID: snapshot.Execution.RoomID, ConversationID: snapshot.Execution.ConversationID, ExecutionID: snapshot.Execution.ID, Role: svc.ExecutionActorCoordinator, RootRoundID: "policy-round", RuntimeRoundID: "policy-round", AgentRoundID: "policy-agent-round"}
					admission := &policyAdmission{receipts: map[string]bool{}}
					service := svc.NewService(repository)
					service.SetScheduledWorkConsumer(admission)
					service.SetAssignmentTargetAuthorizer(policyTargetAuthorizer{})
					if err := service.ActivateRuntimeCoordination(t.Context(), actor, snapshot); err != nil {
						t.Fatal(err)
					}
					input := svc.ConfigureDispatchInput{ExecutionID: batch.ExecutionID, SnapshotRevision: snapshot.Execution.Version, CommandID: "configure-" + string(scope)}
					for _, work := range snapshot.WorkItems {
						input.Policies = append(input.Policies, svc.DispatchPolicyInput{WorkItemID: work.ID, TargetAgentID: "lead", ReturnToAgentID: "lead"})
					}
					configured, err := service.ConfigureDispatch(t.Context(), actor, input)
					if err != nil || configured.Outcome != svc.MutationApplied {
						t.Fatalf("configure=%+v %v", configured, err)
					}
					replay, err := service.ConfigureDispatch(t.Context(), actor, input)
					if err != nil || replay.Outcome != svc.MutationNoOp {
						t.Fatalf("stale exact replay=%+v %v", replay, err)
					}
					input.Policies[0].Instruction = "different intent"
					changed, err := service.ConfigureDispatch(t.Context(), actor, input)
					if err != nil || changed.Outcome != svc.MutationRejected {
						t.Fatalf("changed intent=%+v %v", changed, err)
					}
					// A reconstructed scheduler uses only durable policy authority, not the old round map.
					service = svc.NewService(repository)
					service.SetScheduledWorkConsumer(admission)
					service.SetAssignmentTargetAuthorizer(policyTargetAuthorizer{})
					var group sync.WaitGroup
					for range 2 {
						group.Go(func() { _, _ = service.ReconcileDispatchPolicies(t.Context(), 32) })
					}
					group.Wait()
					snapshot, _ = repository.GetSnapshot(t.Context(), batch.ExecutionID)
					if len(snapshot.Assignments) != 1 || len(snapshot.Attempts) != 1 {
						t.Fatalf("dependency/concurrency admitted %d assignments", len(snapshot.Assignments))
					}
					a, attempt := snapshot.Assignments[0], snapshot.Attempts[0]
					bound := actor
					bound.Role = svc.ExecutionActorMember
					bound.WorkBinding = &protocol.ExecutionWorkBinding{ExecutionID: a.ExecutionID, PlanID: a.PlanID, WorkItemID: a.WorkItemID, SpecID: a.SpecID, AssignmentID: a.ID, AttemptID: attempt.ID, DispatchID: attempt.DispatchID}
					bound.AgentRoundID = "worker-round"
					bound.RuntimeRoundID = "worker-runtime"
					if scope == protocol.ExecutionScopeDM {
						if _, err = service.PrepareScheduledDMRound(t.Context(), bound); err != nil {
							t.Fatal(err)
						}
					}
					snapshot, _ = repository.GetSnapshot(t.Context(), batch.ExecutionID)
					submitted, err := service.SubmitWork(t.Context(), bound, svc.SubmitWorkInput{ExecutionID: batch.ExecutionID, SnapshotRevision: snapshot.Execution.Version, CommandID: "deliver-" + string(scope), ResultSummary: "verified", Evidence: []string{"checked"}})
					if err != nil || submitted.Outcome != svc.MutationApplied {
						t.Fatalf("submit=%+v %v", submitted, err)
					}
					if len(submitted.Snapshot.ReviewDispatches) != 1 {
						t.Fatal("automatic self work has no review outbox")
					}
					if _, err = service.ReconcileDispatchPolicies(t.Context(), 32); err != nil {
						t.Fatal(err)
					}
					still, _ := repository.GetSnapshot(t.Context(), batch.ExecutionID)
					if len(still.Assignments) != 1 {
						t.Fatal("submission unlocked dependency before acceptance")
					}
					reviewer, _ := runtimeReviewFixture(submitted.Snapshot)
					reviewed, err := service.ReviewWork(t.Context(), reviewer, svc.ReviewWorkInput{ExecutionID: batch.ExecutionID, SnapshotRevision: submitted.Snapshot.Execution.Version, CommandID: "review-" + string(scope), Decision: protocol.WorkAcceptanceAccepted, CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{Criterion: "verified", Passed: true, Evidence: []string{"checked"}}}})
					if err != nil || reviewed.Outcome != svc.MutationApplied {
						t.Fatalf("review=%+v %v", reviewed, err)
					}
					admission.paused = true
					if _, err = service.ReconcileDispatchPolicies(t.Context(), 32); err != nil {
						t.Fatal(err)
					}
					paused, _ := repository.GetSnapshot(t.Context(), batch.ExecutionID)
					if len(paused.Assignments) != 1 {
						t.Fatal("paused queue was automatically resumed")
					}
					admission.paused = false
					// Move only this fixture's deferred deadline, without sleeping or mutating domain state.
					stmt := `UPDATE execution_dispatch_policies SET next_attempt_at=created_at WHERE execution_id=?`
					if driver == "postgres" {
						stmt = `UPDATE execution_dispatch_policies SET next_attempt_at=created_at WHERE execution_id=$1`
					}
					if _, err = db.ExecContext(t.Context(), stmt, batch.ExecutionID); err != nil {
						t.Fatal(err)
					}
					if _, err = service.ReconcileDispatchPolicies(t.Context(), 32); err != nil {
						t.Fatal(err)
					}
					final, _ := repository.GetSnapshot(t.Context(), batch.ExecutionID)
					if len(final.Assignments) != 2 || len(final.Attempts) != 2 {
						t.Fatalf("accepted dependency did not dispatch: %+v", final.DispatchPolicies)
					}
					if scope == protocol.ExecutionScopeRoom && len(final.Dispatches) != 2 {
						t.Fatal("Room self policy did not use durable outbox")
					}
					if scope == protocol.ExecutionScopeDM && len(admission.receipts) != 2 {
						t.Fatal("DM policy did not enter durable queue port")
					}

				})
			}
		})
	}
}

func TestWorkGraphTemplateIntegration(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			cfg := integrationDatabase(t, driver)
			migrateIntegrationDatabase(t, cfg, 0)
			db := openIntegrationDatabase(t, cfg)
			templates := workflowrepo.NewRepository(cfg, db)
			repository := orchestrationrepo.NewSQLRepository(driver, db)
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

func TestWorkGraphUpgradeIntegration(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			cfg := integrationDatabase(t, driver)
			migrateIntegrationDatabase(t, cfg, 158)
			oldDB := openIntegrationDatabase(t, cfg)
			oldRepo := orchestrationrepo.NewSQLRepository(driver, oldDB)
			batch := prepareIntegrationBatch(t, legacyPlanWriter{oldRepo, oldDB, driver}, "pre-upgrade")
			if err := oldDB.Close(); err != nil {
				t.Fatal(err)
			}
			migrateIntegrationDatabase(t, cfg, 0)
			db := openIntegrationDatabase(t, cfg)
			repo := orchestrationrepo.NewSQLRepository(driver, db)
			assertIntegrationBatchState(t, db, driver, repo, batch.ExecutionID, 2, 0, 2)
			prepareIntegrationBatch(t, repo, "post-upgrade")
			pending, err := repo.ListRuntimeHistoryBackfills(t.Context(), protocol.Execution{}, 10)
			if err != nil || len(pending) != 1 || pending[0].ID != batch.ExecutionID {
				t.Fatalf("frozen backlog must contain only pre-upgrade execution: %+v, %v", pending, err)
			}
			e := pending[0]
			if next, err := repo.ListRuntimeHistoryBackfills(t.Context(), e, 10); err != nil || len(next) != 0 {
				t.Fatalf("backlog keyset cursor: %+v, %v", next, err)
			}
			now := time.Now().UTC().Truncate(time.Microsecond)
			live := integrationRuntimeNode(e, "live-node", now.Add(time.Second))
			live.ResultSummary = "live result"
			if err := repo.UpsertRuntimeGraphNode(t.Context(), live); err != nil {
				t.Fatal(err)
			}
			historical := live
			historical.UpdatedAt = now
			historical.ResultSummary, historical.WorkGraphReadClass = "obsolete result", "detail"
			recovered := integrationRuntimeNode(e, "recovered-node", now)
			recovered.WorkGraphReadClass = "detail"
			edge := protocol.ExecutionRuntimeEdgeRun{
				ID: "recovered-edge", GraphID: live.GraphID, OwnerUserID: e.OwnerUserID, SessionKey: e.SessionKey,
				SourceNodeID: live.ID, TargetNodeID: recovered.ID, Kind: protocol.ExecutionRuntimeEdgeInvoke, CreatedAt: now,
			}
			badEdge := edge
			badEdge.ID, badEdge.OwnerUserID = "cross-owner-edge", "other-owner"
			graph := protocol.ExecutionRuntimeGraph{Nodes: []protocol.ExecutionRuntimeNodeRun{historical, recovered}, Edges: []protocol.ExecutionRuntimeEdgeRun{edge, badEdge}}
			if err := repo.CompleteRuntimeHistoryBackfill(t.Context(), e, graph); !errors.Is(err, orchestrationrepo.ErrInvariant) {
				t.Fatalf("cross-owner backfill error = %v", err)
			}
			current, err := repo.GetWorkGraphRuntimeGraph(t.Context(), e.OwnerUserID, e.SessionKey, e.ID, e.RootRoundID)
			if err != nil || !current.HistoryPending || len(current.Nodes) != 1 || len(current.Edges) != 0 {
				t.Fatalf("failed backfill must roll back facts and marker: %+v, %v", current, err)
			}
			graph.Edges = graph.Edges[:1]
			for range 2 {
				if err := repo.CompleteRuntimeHistoryBackfill(t.Context(), e, graph); err != nil {
					t.Fatal(err)
				}
			}
			current, err = repo.GetWorkGraphRuntimeGraph(t.Context(), e.OwnerUserID, e.SessionKey, e.ID, e.RootRoundID)
			if err != nil || current.HistoryPending || len(current.Nodes) != 2 || len(current.Edges) != 1 {
				t.Fatalf("backfill retry: %+v, %v", current, err)
			}
			for _, node := range current.Nodes {
				if node.ID == live.ID && (node.ResultSummary != live.ResultSummary || !node.UpdatedAt.Equal(live.UpdatedAt) || node.WorkGraphReadClass != live.WorkGraphReadClass) {
					t.Fatalf("backfill overwrote a live observation: %+v", node)
				}
				if node.ID == recovered.ID && node.WorkGraphReadClass != "detail" {
					t.Fatalf("recovered read classification = %q", node.WorkGraphReadClass)
				}
			}
			if pending, err := repo.ListRuntimeHistoryBackfills(t.Context(), protocol.Execution{}, 10); err != nil || len(pending) != 0 {
				t.Fatalf("completed backlog = %+v, %v", pending, err)
			}
			// Responsibility and CAS data must remain usable after the history upgrade.
			if _, err := repo.AssignBatch(t.Context(), batch); err != nil {
				t.Fatal(err)
			}
			assertIntegrationBatchState(t, db, driver, repo, batch.ExecutionID, 3, 2, 5)
		})
	}
}
