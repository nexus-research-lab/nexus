// INPUT: Migrated SQLite and explicitly configured, schema-isolated PostgreSQL.
// OUTPUT: Durable batch rollback/replay, concurrent revision fences, Draft binding and bounded runtime reads.
// POS: WorkGraph cross-driver integration checks; no application state or provider is accessed.
package storage_test

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage"
	orchestrationrepo "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
	workflowrepo "github.com/nexus-research-lab/nexus/internal/storage/workgraphworkflow"
)

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
