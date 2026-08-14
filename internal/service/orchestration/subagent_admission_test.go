package orchestration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

func TestSubagentAdmissionAllowsRuntimeOnlyWithoutManagedAssignment(t *testing.T) {
	snapshot := assignedExecutionSnapshot()
	snapshot.Assignments = nil
	snapshot.Attempts = nil
	result := admitSubagentWithSnapshot(t, snapshot, subagentActor(), "tool-1")
	assertRuntimeOnlySubagentAdmission(t, result)
}

func TestSubagentAdmissionAllowsRuntimeOnlyWithMultipleManagedCandidates(t *testing.T) {
	snapshot := assignedExecutionSnapshot()
	addSecondDelegableAssignment(snapshot)
	result := admitSubagentWithSnapshot(t, snapshot, subagentActor(), "tool-1")
	assertRuntimeOnlySubagentAdmission(t, result)
}

func TestSubagentAdmissionAllowsRuntimeOnlyWithoutExecutionOrToolCorrelation(t *testing.T) {
	assertRuntimeOnlySubagentAdmission(
		t,
		admitSubagentWithSnapshot(t, nil, subagentActor(), "tool-1"),
	)
	assertRuntimeOnlySubagentAdmission(
		t,
		admitSubagentWithSnapshot(t, assignedExecutionSnapshot(), subagentActor(), ""),
	)
}

func TestSubagentAdmissionRejectsWrongOwnerOrSession(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		actor ActorContext
	}{
		{
			name: "owner",
			actor: ActorContext{
				OwnerUserID: "owner-other",
				SessionKey:  "session-1",
				ExecutionID: "execution-1",
				AgentID:     "agent-worker",
			},
		},
		{
			name: "session",
			actor: ActorContext{
				OwnerUserID: "owner-1",
				SessionKey:  "session-other",
				ExecutionID: "execution-1",
				AgentID:     "agent-worker",
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			result := admitSubagentWithSnapshot(
				t,
				assignedExecutionSnapshot(),
				testCase.actor,
				"tool-1",
			)
			assertSubagentAdmissionRejected(t, result, ErrorCodeWrongOwner)
		})
	}
}

func TestSubagentAdmissionRejectsPlanMode(t *testing.T) {
	actor := subagentActor()
	actor.PlanMode = true
	result := admitSubagentWithSnapshot(t, assignedExecutionSnapshot(), actor, "tool-1")
	assertSubagentAdmissionRejected(t, result, ErrorCodePlanMode)
}

func TestSubagentLifecycleAllowsRuntimeOnlyWithoutManagedBinding(t *testing.T) {
	service := NewService(&fakeRepository{})
	actor := subagentActor()
	start, err := service.ObserveSubagentStart(
		context.Background(),
		actor,
		SubagentLifecycleInput{SDKAgentID: "sdk-child"},
	)
	if err != nil {
		t.Fatal(err)
	}
	assertRuntimeOnlySubagentAdmission(t, start)
	stop, err := service.ObserveSubagentStop(
		context.Background(),
		actor,
		SubagentLifecycleInput{SDKAgentID: "sdk-child"},
	)
	if err != nil {
		t.Fatal(err)
	}
	assertRuntimeOnlySubagentAdmission(t, stop)
	exit, err := service.ObserveSubagentParentRoundExit(
		context.Background(),
		actor,
		SubagentParentRoundExitInput{},
	)
	if err != nil {
		t.Fatal(err)
	}
	assertRuntimeOnlySubagentAdmission(t, exit)
}

func TestSubagentAdmissionAllowsDistinctConcurrentLaunch(t *testing.T) {
	snapshot := assignedExecutionSnapshot()
	snapshot.Assignments[0].Status = protocol.WorkAssignmentStatusActive
	snapshot.Assignments[0].Version = 2
	snapshot.Attempts[0].Status = protocol.WorkAttemptStatusRunning
	snapshot.Attempts[0].Version = 2
	snapshot.Attempts = append(snapshot.Attempts, protocol.WorkAttempt{
		ID:              "attempt-child",
		ExecutionID:     snapshot.Execution.ID,
		PlanID:          "plan-1",
		WorkItemID:      "work-1",
		SpecID:          "spec-1",
		AssignmentID:    "assignment-1",
		ParentAttemptID: "attempt-1",
		ExecutorKind:    protocol.AttemptExecutorSubagent,
		ParentAgentID:   "agent-worker",
		ToolUseID:       "tool-first",
		Status:          protocol.WorkAttemptStatusRunning,
		Version:         1,
	})
	repository := &fakeRepository{snapshot: snapshot}
	repository.startAttempt = func(
		_ context.Context,
		command orchestrationstore.StartAttemptCommand,
	) (*protocol.ExecutionSnapshot, error) {
		updated := cloneExecutionSnapshot(repository.snapshot)
		updated.Execution.Version++
		updated.Assignments[0].Version++
		child := command.Attempt
		child.Status = protocol.WorkAttemptStatusRunning
		child.Version = 1
		updated.Attempts = append(updated.Attempts, child)
		repository.snapshot = updated
		return updated, nil
	}
	service := NewService(repository)
	service.newID = func(string) string { return "attempt-child-second" }
	result, err := service.AdmitSubagentLaunch(
		context.Background(),
		subagentActor(),
		SubagentLaunchInput{ToolUseID: "tool-second"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Allowed || result.Binding == nil ||
		result.Binding.AttemptID != "attempt-child-second" ||
		len(repository.snapshot.Attempts) != 3 {
		t.Fatalf("concurrent admission = %#v, attempts=%#v", result, repository.snapshot.Attempts)
	}
}

func TestSubagentAdmissionRetriesTransientPersistenceFailure(t *testing.T) {
	snapshot := assignedExecutionSnapshot()
	snapshot.Assignments[0].Status = protocol.WorkAssignmentStatusActive
	snapshot.Assignments[0].Version = 2
	snapshot.Attempts[0].Status = protocol.WorkAttemptStatusRunning
	snapshot.Attempts[0].Version = 2
	repository := &fakeRepository{snapshot: snapshot}
	startCalls := 0
	repository.startAttempt = func(
		_ context.Context,
		command orchestrationstore.StartAttemptCommand,
	) (*protocol.ExecutionSnapshot, error) {
		startCalls++
		if startCalls <= 2 {
			return nil, errors.New("database is locked (SQLITE_BUSY)")
		}
		updated := cloneExecutionSnapshot(repository.snapshot)
		updated.Execution.Version++
		updated.Assignments[0].Version++
		child := command.Attempt
		child.Status = protocol.WorkAttemptStatusRunning
		child.Version = 1
		updated.Attempts = append(updated.Attempts, child)
		repository.snapshot = updated
		return updated, nil
	}
	service := NewService(repository)
	service.newID = func(string) string { return "attempt-child-after-lock" }

	result, err := service.AdmitSubagentLaunch(
		context.Background(),
		subagentActor(),
		SubagentLaunchInput{ToolUseID: "tool-after-lock"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Allowed || result.Binding == nil ||
		result.Binding.AttemptID != "attempt-child-after-lock" ||
		startCalls != 3 {
		t.Fatalf("retried admission = %#v, start calls = %d", result, startCalls)
	}
}

func TestSubagentAdmissionReusesExactToolBinding(t *testing.T) {
	snapshot := runningSubagentSnapshot()
	child := snapshot.Attempts[len(snapshot.Attempts)-1]
	result := admitSubagentWithSnapshot(t, snapshot, subagentActor(), child.ToolUseID)
	if !result.Allowed || result.Binding == nil || result.Binding.AttemptID != child.ID {
		t.Fatalf("replayed admission = %#v, want existing child %s", result, child.ID)
	}
}

func TestSubagentAdmissionFallsBackToRuntimeOnlyOnIncompleteManagedBinding(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		mutate func(*protocol.ExecutionSnapshot)
	}{
		{
			name: "missing current state",
			mutate: func(snapshot *protocol.ExecutionSnapshot) {
				snapshot.WorkItemStates = nil
			},
		},
		{
			name: "stale plan item",
			mutate: func(snapshot *protocol.ExecutionSnapshot) {
				snapshot.PlanItems[0].SpecID = "spec-stale"
			},
		},
		{
			name: "unreviewed submission",
			mutate: func(snapshot *protocol.ExecutionSnapshot) {
				snapshot.Submissions = []protocol.WorkSubmission{{
					ID:           "submission-1",
					ExecutionID:  snapshot.Execution.ID,
					PlanID:       "plan-1",
					WorkItemID:   "work-1",
					SpecID:       "spec-1",
					AssignmentID: "assignment-1",
					AttemptID:    "attempt-1",
					Sequence:     1,
				}}
			},
		},
		{
			name: "parent executor mismatch",
			mutate: func(snapshot *protocol.ExecutionSnapshot) {
				snapshot.Attempts[0].ExecutorAgentID = "agent-other"
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			snapshot := assignedExecutionSnapshot()
			testCase.mutate(snapshot)
			result := admitSubagentWithSnapshot(t, snapshot, subagentActor(), "tool-1")
			assertRuntimeOnlySubagentAdmission(t, result)
		})
	}
}

func TestSubagentAdmissionAllowsUniqueCandidateAndPersistsBinding(t *testing.T) {
	snapshot := assignedExecutionSnapshot()
	repository := &fakeRepository{snapshot: snapshot}
	repository.startAttempt = func(
		_ context.Context,
		command orchestrationstore.StartAttemptCommand,
	) (*protocol.ExecutionSnapshot, error) {
		result := cloneExecutionSnapshot(repository.snapshot)
		result.Execution.Version++
		result.Assignments[0].Status = protocol.WorkAssignmentStatusActive
		result.Assignments[0].Version++
		if command.Attempt.ID == "attempt-1" {
			result.Attempts[0] = command.Attempt
			result.Attempts[0].Status = protocol.WorkAttemptStatusRunning
			result.Attempts[0].Version++
		} else {
			child := command.Attempt
			child.Status = protocol.WorkAttemptStatusRunning
			child.Version = 1
			result.Attempts = append(result.Attempts, child)
		}
		repository.snapshot = result
		return result, nil
	}
	service := NewService(repository)
	service.newID = func(kind string) string {
		if kind == "attempt" {
			return "attempt-child"
		}
		return kind + "-generated"
	}
	result, err := service.AdmitSubagentLaunch(context.Background(), subagentActor(), SubagentLaunchInput{
		ToolUseID:         "tool-agent-1",
		RuntimeSessionKey: "runtime-session-1",
		SDKSessionID:      "sdk-session-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Allowed || result.Mode != SubagentAdmissionManaged || result.Binding == nil {
		t.Fatalf("admission = %#v, want allowed durable binding", result)
	}
	if result.Binding.AttemptID != "attempt-child" ||
		result.Binding.ParentAttemptID != "attempt-1" ||
		result.Binding.AssignmentID != "assignment-1" ||
		result.Binding.ToolUseID != "tool-agent-1" {
		t.Fatalf("binding = %#v", result.Binding)
	}
	child := repository.snapshot.Attempts[len(repository.snapshot.Attempts)-1]
	if child.ExecutorKind != protocol.AttemptExecutorSubagent ||
		child.RuntimeSessionKey != "runtime-session-1" ||
		child.SDKSessionID != "sdk-session-1" {
		t.Fatalf("persisted child Attempt = %#v", child)
	}
}

func TestSubagentStopTerminatesOnlyTheBoundChildAttempt(t *testing.T) {
	snapshot := assignedExecutionSnapshot()
	snapshot.Execution.Version = 20
	snapshot.Assignments[0].Status = protocol.WorkAssignmentStatusActive
	snapshot.Assignments[0].Version = 3
	snapshot.Attempts[0].Status = protocol.WorkAttemptStatusRunning
	snapshot.Attempts[0].Version = 2
	snapshot.Attempts = append(snapshot.Attempts, protocol.WorkAttempt{
		ID:                "attempt-child",
		ExecutionID:       snapshot.Execution.ID,
		PlanID:            "plan-1",
		WorkItemID:        "work-1",
		SpecID:            "spec-1",
		AssignmentID:      "assignment-1",
		ParentAttemptID:   "attempt-1",
		ExecutorKind:      protocol.AttemptExecutorSubagent,
		ParentAgentID:     "agent-worker",
		RuntimeSessionKey: "runtime-session-1",
		ToolUseID:         "tool-agent-1",
		Status:            protocol.WorkAttemptStatusRunning,
		Version:           1,
	})
	repository := &fakeRepository{snapshot: snapshot}
	repository.finishAttempt = func(
		_ context.Context,
		command orchestrationstore.FinishAttemptCommand,
	) (*protocol.ExecutionSnapshot, error) {
		result := cloneExecutionSnapshot(repository.snapshot)
		result.Execution.Version++
		for index := range result.Attempts {
			if result.Attempts[index].ID == command.Attempt.ID {
				result.Attempts[index] = command.Attempt
				result.Attempts[index].Version++
			}
		}
		repository.snapshot = result
		return result, nil
	}
	result, err := NewService(repository).ObserveSubagentStop(
		context.Background(),
		subagentActor(),
		SubagentLifecycleInput{
			ToolUseID:           "tool-agent-1",
			SDKSessionID:        "sdk-session-1",
			SDKTaskID:           "sdk-task-1",
			SDKAgentID:          "sdk-agent-1",
			ChildSessionID:      "child-session-1",
			AgentType:           "researcher",
			AgentTranscriptPath: "/tmp/subagent.jsonl",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Allowed || result.Binding == nil ||
		result.Binding.AttemptID != "attempt-child" {
		t.Fatalf("stop result = %#v", result)
	}
	child := repository.snapshot.Attempts[1]
	if child.Status != protocol.WorkAttemptStatusSucceeded ||
		child.Metadata["sdk_agent_id"] != "sdk-agent-1" ||
		child.ExecutorAgentID != "sdk-agent-1" ||
		child.SDKSessionID != "sdk-session-1" ||
		child.ChildSessionID != "child-session-1" ||
		child.SDKTaskID != "sdk-task-1" {
		t.Fatalf("terminal child Attempt = %#v", child)
	}
}

func TestSubagentParentRoundExitPersistsDurableReconciliationDeadline(t *testing.T) {
	snapshot := runningSubagentSnapshot()
	exitedAt := time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC)
	reconcileAfter := exitedAt.Add(30 * time.Second)
	repository := &fakeRepository{snapshot: snapshot}
	repository.scheduleSubagent = func(
		_ context.Context,
		command orchestrationstore.ScheduleSubagentReconciliationCommand,
	) (*protocol.ExecutionSnapshot, error) {
		if command.ExecutionID != "execution-1" ||
			command.AttemptID != "attempt-child" ||
			command.ExpectedExecutionVersion != snapshot.Execution.Version ||
			command.ExpectedAttemptVersion != 1 ||
			!command.ParentRoundExitedAt.Equal(exitedAt) ||
			!command.ReconcileAfter.Equal(reconcileAfter) {
			t.Fatalf("schedule command = %#v", command)
		}
		updated := cloneExecutionSnapshot(repository.snapshot)
		updated.Execution.Version++
		for index := range updated.Attempts {
			if updated.Attempts[index].ID == command.AttemptID {
				updated.Attempts[index].Version++
				updated.Attempts[index].ParentRoundExitedAt = &exitedAt
				updated.Attempts[index].ReconcileAfter = &reconcileAfter
			}
		}
		repository.snapshot = updated
		return updated, nil
	}
	result, err := NewService(repository).ObserveSubagentParentRoundExit(
		context.Background(),
		subagentActor(),
		SubagentParentRoundExitInput{
			ToolUseID:           "tool-agent-1",
			SDKSessionID:        "sdk-session-1",
			SDKAgentID:          "sdk-agent-1",
			ParentRoundExitedAt: exitedAt,
			ReconcileAfter:      reconcileAfter,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Allowed || result.Binding == nil ||
		result.Binding.AttemptID != "attempt-child" {
		t.Fatalf("parent round exit result = %#v", result)
	}
}

func TestSubagentParentRoundExitRejectsNonExactReconciliationDeadline(t *testing.T) {
	exitedAt := time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC)
	result, err := NewService(&fakeRepository{
		snapshot: runningSubagentSnapshot(),
	}).ObserveSubagentParentRoundExit(
		context.Background(),
		subagentActor(),
		SubagentParentRoundExitInput{
			ToolUseID:           "tool-agent-1",
			ParentRoundExitedAt: exitedAt,
			ReconcileAfter: exitedAt.Add(
				protocol.SubagentReconciliationGrace + time.Nanosecond,
			),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Allowed || result.ReasonCode != ErrorCodeInvalidInput {
		t.Fatalf("non-exact deadline result = %#v", result)
	}
}

func TestReconcileExpiredSubagentsTerminalizesDurableChild(t *testing.T) {
	snapshot := runningSubagentSnapshot()
	now := time.Date(2030, time.January, 2, 3, 5, 0, 0, time.UTC)
	exitedAt := now.Add(-time.Minute)
	reconcileAfter := now.Add(-30 * time.Second)
	snapshot.Attempts[1].ParentRoundExitedAt = &exitedAt
	snapshot.Attempts[1].ReconcileAfter = &reconcileAfter
	repository := &fakeRepository{snapshot: snapshot}
	repository.listExpired = func(
		_ context.Context,
		observed time.Time,
		limit int,
	) ([]protocol.WorkAttempt, error) {
		if !observed.Equal(now) || limit != 8 {
			t.Fatalf("expired query now=%s limit=%d", observed, limit)
		}
		return []protocol.WorkAttempt{snapshot.Attempts[1]}, nil
	}
	repository.finishAttempt = func(
		_ context.Context,
		command orchestrationstore.FinishAttemptCommand,
	) (*protocol.ExecutionSnapshot, error) {
		if command.Attempt.ID != "attempt-child" ||
			command.Attempt.Status != protocol.WorkAttemptStatusInterrupted ||
			command.ExpectedAttemptVersion != 1 {
			t.Fatalf("finish command = %#v", command)
		}
		updated := cloneExecutionSnapshot(repository.snapshot)
		updated.Execution.Version++
		updated.Attempts[1] = command.Attempt
		updated.Attempts[1].Version++
		repository.snapshot = updated
		return updated, nil
	}
	service := NewService(repository)
	service.now = func() time.Time { return now }
	result, err := service.ReconcileExpiredSubagents(context.Background(), 8)
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanned != 1 || result.Reconciled != 1 || result.Deferred != 0 {
		t.Fatalf("reconciliation result = %#v", result)
	}
}

func TestReconcileOrphanedSubagentsOnlyClosesPreRestartChildren(t *testing.T) {
	processStartedAt := time.Date(2030, time.January, 2, 3, 5, 0, 0, time.UTC)
	snapshot := runningSubagentSnapshot()
	snapshot.Attempts[1].CreatedAt = processStartedAt.Add(-time.Minute)
	repository := &fakeRepository{snapshot: snapshot}
	repository.listOrphaned = func(
		_ context.Context,
		cutoff time.Time,
		limit int,
	) ([]protocol.WorkAttempt, error) {
		if !cutoff.Equal(processStartedAt) || limit != 8 {
			t.Fatalf("orphan query cutoff=%s limit=%d", cutoff, limit)
		}
		return []protocol.WorkAttempt{snapshot.Attempts[1]}, nil
	}
	repository.finishAttempt = func(
		_ context.Context,
		command orchestrationstore.FinishAttemptCommand,
	) (*protocol.ExecutionSnapshot, error) {
		if command.Attempt.ID != "attempt-child" ||
			command.Attempt.Status != protocol.WorkAttemptStatusInterrupted ||
			!strings.Contains(command.Attempt.FailureReason, "server restarted") {
			t.Fatalf("finish command = %#v", command)
		}
		updated := cloneExecutionSnapshot(repository.snapshot)
		updated.Execution.Version++
		updated.Attempts[1] = command.Attempt
		updated.Attempts[1].Version++
		repository.snapshot = updated
		return updated, nil
	}
	result, err := NewService(repository).ReconcileOrphanedSubagents(
		context.Background(),
		processStartedAt,
		8,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanned != 1 || result.Reconciled != 1 || result.Deferred != 0 {
		t.Fatalf("orphan reconciliation result = %#v", result)
	}
}

func TestReconcileOrphanedSubagentsRechecksImmutableStartupCutoff(t *testing.T) {
	processStartedAt := time.Date(2030, time.January, 2, 3, 5, 0, 0, time.UTC)
	snapshot := runningSubagentSnapshot()
	snapshot.Attempts[1].CreatedAt = processStartedAt.Add(time.Second)
	repository := &fakeRepository{snapshot: snapshot}
	repository.listOrphaned = func(
		context.Context,
		time.Time,
		int,
	) ([]protocol.WorkAttempt, error) {
		// Simulate a stale/overbroad storage result: the service must still
		// protect a child created by this process.
		return []protocol.WorkAttempt{snapshot.Attempts[1]}, nil
	}
	result, err := NewService(repository).ReconcileOrphanedSubagents(
		context.Background(),
		processStartedAt,
		8,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanned != 1 || result.Reconciled != 0 || result.Deferred != 0 {
		t.Fatalf("current-process child result = %#v", result)
	}
}

func admitSubagentWithSnapshot(
	t *testing.T,
	snapshot *protocol.ExecutionSnapshot,
	actor ActorContext,
	toolUseID string,
) SubagentAdmissionResult {
	t.Helper()
	result, err := NewService(&fakeRepository{snapshot: snapshot}).AdmitSubagentLaunch(
		context.Background(),
		actor,
		SubagentLaunchInput{ToolUseID: toolUseID},
	)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertSubagentAdmissionRejected(
	t *testing.T,
	result SubagentAdmissionResult,
	reason ErrorCode,
) {
	t.Helper()
	if result.Allowed || result.ReasonCode != reason || result.Binding != nil {
		t.Fatalf("admission = %#v, want rejected %s", result, reason)
	}
}

func assertRuntimeOnlySubagentAdmission(
	t *testing.T,
	result SubagentAdmissionResult,
) {
	t.Helper()
	if !result.Allowed ||
		result.Mode != SubagentAdmissionRuntimeOnly ||
		result.Binding != nil ||
		result.ReasonCode != "" {
		t.Fatalf("admission = %#v, want runtime-only allow", result)
	}
}

func subagentActor() ActorContext {
	return ActorContext{
		OwnerUserID:  "owner-1",
		SessionKey:   "session-1",
		ExecutionID:  "execution-1",
		AgentID:      "agent-worker",
		Role:         ExecutionActorMember,
		ActorKind:    protocol.ExecutionActorAgent,
		ScopeKind:    protocol.ExecutionScopeDM,
		RootRoundID:  "round-root",
		AgentRoundID: "round-agent",
	}
}

func runningSubagentSnapshot() *protocol.ExecutionSnapshot {
	snapshot := assignedExecutionSnapshot()
	snapshot.Execution.Version = 20
	snapshot.Assignments[0].Status = protocol.WorkAssignmentStatusActive
	snapshot.Assignments[0].Version = 3
	snapshot.Attempts[0].Status = protocol.WorkAttemptStatusRunning
	snapshot.Attempts[0].Version = 2
	snapshot.Attempts = append(snapshot.Attempts, protocol.WorkAttempt{
		ID:                "attempt-child",
		ExecutionID:       snapshot.Execution.ID,
		PlanID:            "plan-1",
		WorkItemID:        "work-1",
		SpecID:            "spec-1",
		AssignmentID:      "assignment-1",
		ParentAttemptID:   "attempt-1",
		ExecutorKind:      protocol.AttemptExecutorSubagent,
		ParentAgentID:     "agent-worker",
		RuntimeSessionKey: "runtime-session-1",
		SDKSessionID:      "sdk-session-1",
		ToolUseID:         "tool-agent-1",
		Status:            protocol.WorkAttemptStatusRunning,
		Version:           1,
	})
	return snapshot
}

func addSecondDelegableAssignment(snapshot *protocol.ExecutionSnapshot) {
	snapshot.WorkItems = append(snapshot.WorkItems, protocol.WorkItem{
		ID:          "work-2",
		ExecutionID: snapshot.Execution.ID,
		LogicalKey:  "analysis",
		Kind:        protocol.WorkItemKindProduce,
	})
	snapshot.WorkItemStates = append(snapshot.WorkItemStates, protocol.WorkItemState{
		WorkItemID:    "work-2",
		ExecutionID:   snapshot.Execution.ID,
		CurrentSpecID: "spec-2",
		Status:        protocol.WorkItemStatusOpen,
		Version:       1,
	})
	snapshot.WorkItemSpecs = append(snapshot.WorkItemSpecs, protocol.WorkItemSpec{
		ID:                 "spec-2",
		WorkItemID:         "work-2",
		ExecutionID:        snapshot.Execution.ID,
		Version:            1,
		Subject:            "Analyze",
		Objective:          "Analyze evidence",
		Deliverable:        "Analysis",
		AcceptanceCriteria: []string{"claims supported"},
		SpecHash:           "hash-2",
	})
	snapshot.PlanItems = append(snapshot.PlanItems, protocol.ExecutionPlanItem{
		PlanID:      "plan-1",
		ExecutionID: snapshot.Execution.ID,
		WorkItemID:  "work-2",
		SpecID:      "spec-2",
		Required:    true,
	})
	snapshot.Assignments = append(snapshot.Assignments, protocol.WorkAssignment{
		ID:           "assignment-2",
		ExecutionID:  snapshot.Execution.ID,
		PlanID:       "plan-1",
		WorkItemID:   "work-2",
		SpecID:       "spec-2",
		OwnerAgentID: "agent-worker",
		Strategy:     protocol.AssignmentStrategySelf,
		Status:       protocol.WorkAssignmentStatusAssigned,
		Version:      1,
	})
	snapshot.Attempts = append(snapshot.Attempts, protocol.WorkAttempt{
		ID:              "attempt-2",
		ExecutionID:     snapshot.Execution.ID,
		PlanID:          "plan-1",
		WorkItemID:      "work-2",
		SpecID:          "spec-2",
		AssignmentID:    "assignment-2",
		ExecutorKind:    protocol.AttemptExecutorAgent,
		ExecutorAgentID: "agent-worker",
		Status:          protocol.WorkAttemptStatusPending,
		Version:         1,
	})
}
