// context_and_admission_test.go 覆盖编排上下文投影与子代理准入测试。
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

func TestRenderExecutionContextProjectsOnlyActionableRuntimeFacts(t *testing.T) {
	now := time.Date(2026, 8, 5, 11, 0, 0, 0, time.UTC)
	failed := protocol.ExecutionRuntimeNodeRun{
		ID: "runtime-tool-failed", GraphID: "execution:execution-context",
		Kind: protocol.ExecutionRuntimeNodeTool, SubjectID: "tool-search-failed",
		AgentRoundID: "agent-round-analyst", AgentID: "analyst", Name: "search",
		Status: protocol.ExecutionRuntimeNodeFailed, Failed: true,
		ErrorCode: "page_unavailable", ErrorSummary: "The requested page was unavailable",
		StartedAt: now, UpdatedAt: now,
	}
	retried := failed
	retried.ID = "runtime-tool-retried"
	retried.SubjectID = "tool-search-retried"
	retried.Status = protocol.ExecutionRuntimeNodeSucceeded
	retried.Failed = false
	retried.ErrorCode = ""
	retried.ErrorSummary = ""
	retried.ResultSummary = "The page was retrieved"
	retried.UpdatedAt = now.Add(time.Second)
	retried.Artifacts = []protocol.WorkspaceFileArtifactBlock{{
		ID: "artifact-result", Type: protocol.ContentBlockTypeWorkspaceFileArtifact,
		Path: "reports/source.md", SourceToolUseID: retried.SubjectID,
	}}
	otherAgent := failed
	otherAgent.ID = "runtime-tool-other-agent"
	otherAgent.SubjectID = "tool-private"
	otherAgent.AgentID = "lead"
	otherAgent.ErrorSummary = "OTHER AGENT PRIVATE INTERMEDIATE"
	commandNode := retried
	commandNode.ID = "runtime-command-inspect"
	commandNode.SubjectID = "tool-command-inspect"
	commandNode.ResultSummary = "RECURSIVE INSPECT CONTEXT"
	commandNode.Metadata = map[string]any{runtimeGraphCommandTransportMetadataKey: true}
	runtimeGraph := protocol.ExecutionRuntimeGraph{
		GraphID: "execution:execution-context",
		Nodes:   []protocol.ExecutionRuntimeNodeRun{failed, retried, otherAgent, commandNode},
		Edges: []protocol.ExecutionRuntimeEdgeRun{{
			ID: "runtime-retry", Kind: protocol.ExecutionRuntimeEdgeRetry,
			SourceNodeID: failed.ID, TargetNodeID: retried.ID, CreatedAt: now.Add(time.Second),
		}},
		NodeTotal: 40, EdgeTotal: 45, NodesTruncated: true,
	}
	rendered := RenderExecutionContext(
		func() *protocol.ExecutionSnapshot {
			snapshot := executionContextTestSnapshot()
			return &snapshot
		}(),
		ExecutionContextOptions{
			ActorAgentID:         "analyst",
			Role:                 ExecutionActorMember,
			RuntimeGraph:         &runtimeGraph,
			RuntimeGraphRelation: "current_execution",
		},
	)
	for _, expected := range []string{
		`<runtime_facts available="true" mode="observed_facts_only" relation="current_execution" graph_id="execution:execution-context" partial="true" node_total="40" edge_total="45" visible_node_total="2">`,
		`<attention_nodes>`,
		`code="page_unavailable">The requested page was unavailable</error>`,
		`tool_use_id="tool-search-retried" path="reports/source.md"`,
		`kind="retry" source_node_id="runtime-tool-failed"`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("runtime facts missing %q:\n%s", expected, rendered)
		}
	}
	for _, forbidden := range []string{
		"OTHER AGENT PRIVATE INTERMEDIATE",
		"RECURSIVE INSPECT CONTEXT",
		"The page was retrieved",
		"successful_nodes",
		"next_action",
		"retry this tool",
	} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("runtime facts leaked or prescribed %q:\n%s", forbidden, rendered)
		}
	}
}

func TestRenderExecutionContextDoesNotExposeUnacceptedDependencyPayload(t *testing.T) {
	snapshot := executionContextTestSnapshot()
	snapshot.Acceptances = nil
	snapshot.Submissions[0].ResultSummary = "UNREVIEWED SIBLING PAYLOAD"
	snapshot.Submissions[0].ResultRefs = []string{"secret://unreviewed"}
	rendered := RenderExecutionContext(&snapshot, ExecutionContextOptions{
		ActorAgentID: "analyst",
		Role:         ExecutionActorMember,
	})
	for _, expected := range []string{
		`<dependency kind="hard" status="pending_review">`,
		`<upstream work_item_id="work-research" logical_key="W1" spec_id="spec-research" />`,
		`<blocker>upstream Work Item current spec is not accepted</blocker>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("unaccepted dependency context missing %q:\n%s", expected, rendered)
		}
	}
	for _, forbidden := range []string{
		"UNREVIEWED SIBLING PAYLOAD",
		"secret://unreviewed",
		`<accepted_submission id="submission-research">`,
	} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("unaccepted dependency payload leaked %q:\n%s", forbidden, rendered)
		}
	}
}

func TestRenderExecutionContextShowsCanonicalObjectiveAndWaitingInputEvidence(t *testing.T) {
	snapshot := executionContextTestSnapshot()
	snapshot.Execution.Objective = "Deliver one verified comparison"
	snapshot.Execution.CompletionCriteria = []string{"terminal report accepted"}
	snapshot.WorkItemStates[1].Status = protocol.WorkItemStatusWaitingInput
	snapshot.WorkItemStates[1].BlockReason = "source access is missing"
	snapshot.WorkItemStates[1].NeededInput = "approved source credentials"

	rendered := RenderExecutionContext(&snapshot, ExecutionContextOptions{
		ActorAgentID: "analyst",
		Role:         ExecutionActorMember,
	})
	for _, expected := range []string{
		`<objective>Deliver one verified comparison</objective>`,
		`<criterion>terminal report accepted</criterion>`,
		`status="waiting_input"`,
		`<reason>source access is missing</reason>`,
		`<needed_input>approved source credentials</needed_input>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("context missing %q:\n%s", expected, rendered)
		}
	}
}

func TestRenderExecutionContextReturnsEmptyWithoutManagedExecution(t *testing.T) {
	if got := RenderExecutionContext(nil, ExecutionContextOptions{}); got != "" {
		t.Fatalf("nil snapshot context = %q, want empty", got)
	}
	if got := RenderExecutionContext(&protocol.ExecutionSnapshot{}, ExecutionContextOptions{}); got != "" {
		t.Fatalf("empty snapshot context = %q, want empty", got)
	}
}

func TestRenderUnmanagedExecutionContextUsesCompactRoundMarker(t *testing.T) {
	tests := []struct {
		name    string
		options ExecutionContextOptions
		want    string
	}{
		{
			name: "coordinator",
			options: ExecutionContextOptions{
				Role: ExecutionActorCoordinator,
			},
			want: `<nexus_round lane="coordination" role="coordinator" />`,
		},
		{
			name: "Room member",
			options: ExecutionContextOptions{
				ScopeKind: protocol.ExecutionScopeRoom,
			},
			want: `<nexus_round lane="conversation" role="member" />`,
		},
		{
			name: "Plan Mode",
			options: ExecutionContextOptions{
				Role:     ExecutionActorCoordinator,
				PlanMode: true,
			},
			want: `<nexus_round lane="coordination" role="coordinator" plan_only="true" />`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := RenderUnmanagedExecutionContext(test.options); got != test.want {
				t.Fatalf("round context = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRenderExecutionContextTerminalExecutionExposesInspectionOnly(t *testing.T) {
	snapshot := executionContextTestSnapshot()
	snapshot.Execution.GoalID = ""
	snapshot.Execution.Status = protocol.ExecutionStatusSuperseded

	rendered := RenderExecutionContext(&snapshot, ExecutionContextOptions{
		ActorAgentID: "lead",
	})
	if !strings.Contains(
		rendered,
		`<execution_transition replace_current_allowed="false" abandon_allowed="false" validation_only="false" reason_code="execution_terminal">`,
	) {
		t.Fatalf("terminal transition boundary = %s", rendered)
	}
	allowed := rendered[strings.Index(
		rendered,
		"<allowed_actions>",
	):strings.Index(rendered, "</allowed_actions>")]
	if !strings.Contains(allowed, "<action>get_execution</action>") {
		t.Fatalf("terminal Execution cannot be inspected:\n%s", rendered)
	}
	if !strings.Contains(allowed, "<action>subagent/spawn</action>") {
		t.Fatalf("terminal background Execution blocked subagent delegation:\n%s", rendered)
	}
	for _, action := range []string{
		"plan_execution",
		"abandon_execution",
		"assign_work",
		"submit_work",
		"review_work",
		"block_work",
		"resume_work",
		"take_over_work",
		"promote_execution_to_goal",
	} {
		if strings.Contains(allowed, "<action>"+action+"</action>") {
			t.Fatalf("terminal Execution exposed %q:\n%s", action, rendered)
		}
	}
}

func TestRenderExecutionContextReadyWorkCarriesCurrentSpecReviewAndResumeFacts(t *testing.T) {
	snapshot := executionContextTestSnapshot()
	snapshot.Assignments[1].Status = protocol.WorkAssignmentStatusReleased
	snapshot.Attempts = nil
	snapshot.WorkItemStates[1].Metadata = map[string]any{
		"last_resume_resolution": "source access approved",
		"last_resume_evidence":   []any{"approval-42"},
	}
	snapshot.Submissions = append(snapshot.Submissions, protocol.WorkSubmission{
		ID:               "submission-analysis",
		ExecutionID:      "execution-1",
		PlanID:           "plan-1",
		WorkItemID:       "work-analysis",
		SpecID:           "spec-analysis",
		AssignmentID:     "assignment-analysis",
		AttemptID:        "attempt-analysis",
		Sequence:         1,
		SubmitterAgentID: "analyst",
		ResultSummary:    "Initial comparison",
	})
	snapshot.Acceptances = append(snapshot.Acceptances, protocol.WorkAcceptance{
		ID:           "acceptance-analysis",
		ExecutionID:  "execution-1",
		PlanID:       "plan-1",
		WorkItemID:   "work-analysis",
		SpecID:       "spec-analysis",
		AssignmentID: "assignment-analysis",
		SubmissionID: "submission-analysis",
		Decision:     protocol.WorkAcceptanceChangesRequested,
		Feedback:     "Add the missing efficiency evidence",
		CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{
			Criterion: "Uses accepted sources",
			Passed:    false,
			Evidence:  []string{"review://gap-1"},
			Note:      "Efficiency claim is unsupported",
		}},
	})

	rendered := RenderExecutionContext(&snapshot, ExecutionContextOptions{
		ActorAgentID: "lead",
	})
	for _, expected := range []string{
		`<item id="work-analysis" logical_key="W2" spec_id="spec-analysis" kind="produce">`,
		`<objective>Synthesize accepted research</objective>`,
		`<deliverable>Compare &amp; verify</deliverable>`,
		`<criterion>Uses accepted sources</criterion>`,
		`<work_item_id>work-research</work_item_id>`,
		`<latest_review submission_id="submission-analysis" decision="changes_requested">`,
		`<feedback>Add the missing efficiency evidence</feedback>`,
		`<criterion passed="false">`,
		`<requirement>Uses accepted sources</requirement>`,
		`<item>review://gap-1</item>`,
		`<note>Efficiency claim is unsupported</note>`,
		`<resume_context>`,
		`<resolution>source access approved</resolution>`,
		`<item>approval-42</item>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("ready work context missing %q:\n%s", expected, rendered)
		}
	}
}

func TestRenderExecutionContextAssignedOwnerCarriesResumeFacts(t *testing.T) {
	snapshot := assignedExecutionSnapshot()
	snapshot.WorkItemStates[0].Metadata = map[string]any{
		"last_resume_resolution": "legal approval received",
		"last_resume_evidence":   []string{"approval://42"},
	}
	rendered := RenderExecutionContext(snapshot, ExecutionContextOptions{
		ActorAgentID: "agent-worker",
		Role:         ExecutionActorMember,
	})
	for _, expected := range []string{
		`assignment_id="assignment-1"`,
		`<resume_context>`,
		`<resolution>legal approval received</resolution>`,
		`<item>approval://42</item>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("assigned owner context missing %q:\n%s", expected, rendered)
		}
	}
}

func TestRenderExecutionContextProjectsManagedAndRuntimeOnlySubagentModes(t *testing.T) {
	unique := assignedExecutionSnapshot()
	rendered := RenderExecutionContext(unique, ExecutionContextOptions{
		ActorAgentID: "agent-worker",
		Role:         ExecutionActorMember,
	})
	for _, expected := range []string{
		`<subagent_admission eligible="true" command_domain="subagent" operation="spawn" candidate_assignment_count="1" binding_mode="managed" assignment_id="assignment-1" work_item_id="work-1" parent_attempt_id="attempt-1" />`,
		`<action>subagent/spawn</action>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("eligible subagent context missing %q:\n%s", expected, rendered)
		}
	}

	multiple := cloneExecutionSnapshot(unique)
	addSecondDelegableAssignment(multiple)
	rendered = RenderExecutionContext(multiple, ExecutionContextOptions{
		ActorAgentID: "agent-worker",
		Role:         ExecutionActorMember,
	})
	for _, expected := range []string{
		`<subagent_admission eligible="true" command_domain="subagent" operation="spawn" candidate_assignment_count="2" binding_mode="runtime_only" managed_binding_reason="ambiguous_assignment">`,
		`<note>subagent delegation is admissible, but this run is runtime observation only and does not claim managed Work Item evidence: the current Agent has multiple delegable Assignments; select one through the WorkGraph before launching a subagent</note>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("ambiguous subagent context missing %q:\n%s", expected, rendered)
		}
	}
	allowed := rendered[strings.Index(rendered, "<allowed_actions>"):strings.Index(rendered, "</allowed_actions>")]
	if !strings.Contains(allowed, "<action>subagent/spawn</action>") {
		t.Fatalf("runtime-only mode hid native Agent affordance:\n%s", rendered)
	}

	rendered = RenderExecutionContext(unique, ExecutionContextOptions{
		ActorAgentID: "agent-worker",
		Role:         ExecutionActorMember,
		PlanMode:     true,
	})
	if !strings.Contains(
		rendered,
		`<subagent_admission eligible="false" command_domain="subagent" operation="spawn" candidate_assignment_count="1" reason_code="plan_mode">`,
	) {
		t.Fatalf("Plan Mode subagent boundary is not explicit:\n%s", rendered)
	}
}

func TestRenderExecutionContextProjectsLatestTerminalSubagentEvidence(t *testing.T) {
	snapshot := assignedExecutionSnapshot()
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
		Status:          protocol.WorkAttemptStatusSucceeded,
		CreatedAt:       time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC),
		Metadata: map[string]any{
			"agent_transcript_path":      "/tmp/subagent.jsonl",
			"has_last_assistant_message": true,
		},
	})
	rendered := RenderExecutionContext(snapshot, ExecutionContextOptions{
		ActorAgentID: "agent-worker",
		Role:         ExecutionActorMember,
	})
	for _, expected := range []string{
		`assignment_id="assignment-1" kind="produce" status="pending" attempt_id="attempt-1"`,
		`<subagent_result attempt_id="attempt-child" parent_attempt_id="attempt-1" status="succeeded" transcript_ref="/tmp/subagent.jsonl" has_last_assistant_message="true">`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("terminal subagent evidence missing %q:\n%s", expected, rendered)
		}
	}
}

func executionContextTestSnapshot() protocol.ExecutionSnapshot {
	now := time.Date(2026, 7, 30, 8, 0, 0, 0, time.UTC)
	return protocol.ExecutionSnapshot{
		Execution: protocol.Execution{
			ID:                    "execution-1",
			SessionKey:            "room:group:conversation-1",
			ScopeKind:             protocol.ExecutionScopeRoom,
			CoordinatorAgentID:    "lead",
			GoalID:                "goal-1",
			GoalObjectiveRevision: 2,
			Status:                protocol.ExecutionStatusActive,
			Version:               7,
		},
		Plan: &protocol.ExecutionPlanRevision{
			ID:          "plan-1",
			ExecutionID: "execution-1",
			Revision:    3,
			Status:      protocol.PlanRevisionStatusActive,
		},
		WorkItems: []protocol.WorkItem{
			{ID: "work-research", ExecutionID: "execution-1", LogicalKey: "W1", Kind: protocol.WorkItemKindProduce},
			{ID: "work-analysis", ExecutionID: "execution-1", LogicalKey: "W2", Kind: protocol.WorkItemKindProduce},
			{ID: "work-integration", ExecutionID: "execution-1", LogicalKey: "W3", Kind: protocol.WorkItemKindIntegrate},
		},
		WorkItemStates: []protocol.WorkItemState{
			{WorkItemID: "work-research", CurrentSpecID: "spec-research", Status: protocol.WorkItemStatusOpen},
			{WorkItemID: "work-analysis", CurrentSpecID: "spec-analysis", Status: protocol.WorkItemStatusOpen},
			{WorkItemID: "work-integration", CurrentSpecID: "spec-integration", Status: protocol.WorkItemStatusOpen},
		},
		WorkItemSpecs: []protocol.WorkItemSpec{
			{
				ID:          "spec-research",
				WorkItemID:  "work-research",
				ExecutionID: "execution-1",
				Subject:     "Research",
				Deliverable: "Sources",
			},
			{
				ID:                 "spec-analysis",
				WorkItemID:         "work-analysis",
				ExecutionID:        "execution-1",
				Subject:            "Analyze",
				Objective:          "Synthesize accepted research",
				Deliverable:        "Compare & verify",
				AcceptanceCriteria: []string{"Uses accepted sources"},
				InputRefs:          []string{"brief://m4", "artifact://accepted-research"},
			},
			{
				ID:          "spec-integration",
				WorkItemID:  "work-integration",
				ExecutionID: "execution-1",
				Subject:     "Integrate",
				Deliverable: "Report",
			},
		},
		PlanItems: []protocol.ExecutionPlanItem{
			{PlanID: "plan-1", WorkItemID: "work-research", SpecID: "spec-research"},
			{PlanID: "plan-1", WorkItemID: "work-analysis", SpecID: "spec-analysis"},
			{PlanID: "plan-1", WorkItemID: "work-integration", SpecID: "spec-integration", Terminal: true},
		},
		Dependencies: []protocol.ExecutionPlanDependency{
			{
				PlanID:              "plan-1",
				WorkItemID:          "work-analysis",
				DependsOnWorkItemID: "work-research",
				Kind:                protocol.WorkDependencyHard,
			},
			{
				PlanID:              "plan-1",
				WorkItemID:          "work-integration",
				DependsOnWorkItemID: "work-analysis",
				Kind:                protocol.WorkDependencyHard,
			},
		},
		OutputClaims: []protocol.ExecutionPlanOutputClaim{
			{
				PlanID:      "plan-1",
				ExecutionID: "execution-1",
				WorkItemID:  "work-research",
				SpecID:      "spec-research",
				Scope:       "file:research/source-set.json",
				Mode:        protocol.WorkOutputScopeExclusive,
			},
			{
				PlanID:      "plan-1",
				ExecutionID: "execution-1",
				WorkItemID:  "work-analysis",
				SpecID:      "spec-analysis",
				Scope:       "file:reports/comparison.md",
				Mode:        protocol.WorkOutputScopeExclusive,
			},
		},
		Assignments: []protocol.WorkAssignment{
			{
				ID:              "assignment-research",
				ExecutionID:     "execution-1",
				PlanID:          "plan-1",
				WorkItemID:      "work-research",
				SpecID:          "spec-research",
				OwnerAgentID:    "researcher",
				ReturnToAgentID: "lead",
				Status:          protocol.WorkAssignmentStatusCompleted,
			},
			{
				ID:              "assignment-analysis",
				ExecutionID:     "execution-1",
				PlanID:          "plan-1",
				WorkItemID:      "work-analysis",
				SpecID:          "spec-analysis",
				OwnerAgentID:    "analyst",
				ReturnToAgentID: "lead",
				Status:          protocol.WorkAssignmentStatusActive,
			},
		},
		Attempts: []protocol.WorkAttempt{
			{
				ID:           "attempt-analysis",
				AssignmentID: "assignment-analysis",
				DispatchID:   "dispatch-analysis",
				Status:       protocol.WorkAttemptStatusRunning,
			},
		},
		Submissions: []protocol.WorkSubmission{
			{
				ID:               "submission-research",
				ExecutionID:      "execution-1",
				PlanID:           "plan-1",
				WorkItemID:       "work-research",
				SpecID:           "spec-research",
				AssignmentID:     "assignment-research",
				AttemptID:        "attempt-research",
				Sequence:         1,
				SubmitterAgentID: "researcher",
				ResultSummary:    "Accepted source set",
				ResultRefs:       []string{"artifact://source-set"},
				Evidence:         []string{"source://official"},
				CreatedAt:        now,
			},
		},
		Acceptances: []protocol.WorkAcceptance{
			{
				ID:           "acceptance-research",
				ExecutionID:  "execution-1",
				PlanID:       "plan-1",
				WorkItemID:   "work-research",
				SpecID:       "spec-research",
				AssignmentID: "assignment-research",
				SubmissionID: "submission-research",
				Decision:     protocol.WorkAcceptanceAccepted,
				CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{
					Criterion: "Official sources",
					Passed:    true,
					Evidence:  []string{"source://official"},
				}},
				CreatedAt: now,
			},
		},
		CompletionBlockers: []string{"terminal verification is not accepted"},
	}
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
	service := NewService(repository)
	result, err := service.ObserveSubagentParentRoundExit(
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
