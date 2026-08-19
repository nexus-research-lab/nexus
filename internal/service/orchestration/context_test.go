package orchestration

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRenderExecutionContextMarksAbnormalHistoricalProjectionTruncation(t *testing.T) {
	snapshot := executionContextTestSnapshot()
	values := make([]string, protocol.ExecutionProjectionCollectionLimit+1)
	for index := range values {
		values[index] = fmt.Sprintf("criterion-%02d", index)
	}
	snapshot.Execution.CompletionCriteria = values

	rendered := RenderExecutionContext(&snapshot, ExecutionContextOptions{
		ActorAgentID: "coordinator",
		Role:         ExecutionActorCoordinator,
	})
	marker := `<completion_criteria truncated="true" total="33">`
	if !strings.Contains(rendered, marker) {
		t.Fatalf("context lacks explicit truncation marker %q: %s", marker, rendered)
	}
	if strings.Contains(rendered, "criterion-32") {
		t.Fatalf("context rendered an item beyond the protocol bound: %s", rendered)
	}
}

func TestRenderExecutionContextScopesObservedRuntimeFactsWithoutChoosingNextAction(t *testing.T) {
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
	runtimeGraph := protocol.ExecutionRuntimeGraph{
		GraphID: "execution:execution-context",
		Nodes:   []protocol.ExecutionRuntimeNodeRun{failed, retried, otherAgent},
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
		`code="page_unavailable">The requested page was unavailable</error>`,
		`<result_summary>The page was retrieved</result_summary>`,
		`tool_use_id="tool-search-retried" path="reports/source.md"`,
		`kind="retry" source_node_id="runtime-tool-failed"`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("runtime facts missing %q:\n%s", expected, rendered)
		}
	}
	for _, forbidden := range []string{
		"OTHER AGENT PRIVATE INTERMEDIATE",
		"next_action",
		"retry this tool",
	} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("runtime facts leaked or prescribed %q:\n%s", forbidden, rendered)
		}
	}
}

func TestRenderExecutionContextShowsOnlyActorAssignmentAndAcceptedDependencyUnlock(t *testing.T) {
	snapshot := executionContextTestSnapshot()
	rendered := RenderExecutionContext(&snapshot, ExecutionContextOptions{
		ActorAgentID: "analyst",
		Role:         ExecutionActorMember,
	})

	for _, expected := range []string{
		`<actor agent_id="analyst" role="member" />`,
		`<graph_digest notation="nexus-dag-v1" scope="actor_slice" plan_revision="3">`,
		`<node key="W1" subject="Research" kind="produce" status="accepted" />`,
		`<node key="W2" subject="Analyze" kind="produce" status="assigned" owner_agent_id="analyst" current_actor="true" />`,
		`<edge from="W1" to="W2" kind="hard" />`,
		`<item id="work-analysis" logical_key="W2" spec_id="spec-analysis" assignment_id="assignment-analysis"`,
		`attempt_id="attempt-analysis" dispatch_id="dispatch-analysis"`,
		`<deliverable>Compare &amp; verify</deliverable>`,
		`<ref>artifact://accepted-research</ref>`,
		`<scope mode="exclusive">file:reports/comparison.md</scope>`,
		`<work_item_id>work-research</work_item_id>`,
		`<dependency kind="hard" status="accepted">`,
		`<upstream work_item_id="work-research" logical_key="W1" spec_id="spec-research" />`,
		`<accepted_submission id="submission-research">`,
		`<result_summary>Accepted source set</result_summary>`,
		`<ref>artifact://source-set</ref>`,
		`<acceptance id="acceptance-research">`,
		`<action>submit_work</action>`,
		`<action>mutate_sibling_work</action>`,
		`<blocker>terminal verification is not accepted</blocker>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("context missing %q:\n%s", expected, rendered)
		}
	}
	for _, forbiddenLeak := range []string{
		`assignment_id="assignment-research"`,
		`<item id="work-integration"`,
		`<node key="W3"`,
		`owner_agent_id="researcher"`,
		`<submission id="submission-analysis"`,
	} {
		if strings.Contains(rendered, forbiddenLeak) {
			t.Fatalf("member context leaked coordinator-only state %q:\n%s", forbiddenLeak, rendered)
		}
	}
}

func TestRenderExecutionContextGivesCoordinatorFullGraphDigest(t *testing.T) {
	snapshot := executionContextTestSnapshot()
	rendered := RenderExecutionContext(&snapshot, ExecutionContextOptions{
		ActorAgentID: "lead",
		Role:         ExecutionActorCoordinator,
	})
	for _, expected := range []string{
		`<graph_digest notation="nexus-dag-v1" scope="full" plan_revision="3">`,
		`<node key="W1" subject="Research" kind="produce" status="accepted" owner_agent_id="researcher" />`,
		`<node key="W2" subject="Analyze" kind="produce" status="assigned" owner_agent_id="analyst" />`,
		`<node key="W3" subject="Integrate" kind="integrate" status="waiting" terminal="true" />`,
		`<edge from="W1" to="W2" kind="hard" />`,
		`<edge from="W2" to="W3" kind="hard" />`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("coordinator graph digest missing %q:\n%s", expected, rendered)
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

func TestRenderExecutionContextCoordinatorSeesReadyWorkAndPendingReview(t *testing.T) {
	snapshot := executionContextTestSnapshot()
	snapshot.Assignments[1].Status = protocol.WorkAssignmentStatusCompleted
	snapshot.Attempts[0].Status = protocol.WorkAttemptStatusSucceeded
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
		ResultSummary:    "Compared",
		ResultRefs:       []string{"artifact://comparison"},
		Evidence:         []string{"benchmark-42"},
	})

	rendered := RenderExecutionContext(&snapshot, ExecutionContextOptions{
		ActorAgentID: "lead",
		ScopeKind:    protocol.ExecutionScopeRoom,
		ReviewBound:  true,
	})
	for _, expected := range []string{
		`role="coordinator"`,
		`<objective></objective>`,
		`<submission id="submission-analysis" work_item_id="work-analysis" logical_key="W2" spec_id="spec-analysis" submitter_agent_id="analyst">`,
		`<objective>Synthesize accepted research</objective>`,
		`<deliverable>Compare &amp; verify</deliverable>`,
		`<criterion>Uses accepted sources</criterion>`,
		`<work_item_id>work-research</work_item_id>`,
		`<result_summary>Compared</result_summary>`,
		`<ref>artifact://comparison</ref>`,
		`<item>benchmark-42</item>`,
		`<action>review_work</action>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("coordinator context missing %q:\n%s", expected, rendered)
		}
	}
	if strings.Contains(rendered, `<item id="work-integration"`) {
		t.Fatalf("terminal work must remain blocked until analysis is accepted:\n%s", rendered)
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

func TestRenderUnmanagedExecutionContextMakesRoleAndActionsExplicit(t *testing.T) {
	coordinator := RenderUnmanagedExecutionContext(ExecutionContextOptions{
		ActorAgentID: "lead",
		Role:         ExecutionActorCoordinator,
		ScopeKind:    protocol.ExecutionScopeRoom,
	})
	for _, expected := range []string{
		`<execution state="unmanaged" />`,
		`<actor agent_id="lead" role="coordinator" />`,
		`<action>plan_execution</action>`,
		`<action>Agent</action>`,
		`<action>assign_work</action>`,
	} {
		if !strings.Contains(coordinator, expected) {
			t.Fatalf("unmanaged coordinator context missing %q:\n%s", expected, coordinator)
		}
	}
	allowed := coordinator[strings.Index(coordinator, "<allowed_actions>"):strings.Index(coordinator, "</allowed_actions>")]
	if strings.Contains(allowed, "<action>assign_work</action>") {
		t.Fatalf("unmanaged coordinator may not assign before a Plan exists:\n%s", coordinator)
	}

	member := RenderUnmanagedExecutionContext(ExecutionContextOptions{
		ActorAgentID: "worker",
		ScopeKind:    protocol.ExecutionScopeRoom,
	})
	memberAllowed := member[strings.Index(member, "<allowed_actions>"):strings.Index(member, "</allowed_actions>")]
	if !strings.Contains(member, `role="member"`) ||
		!strings.Contains(member, `<action>create_shared_execution</action>`) ||
		!strings.Contains(memberAllowed, `<action>Agent</action>`) ||
		!strings.Contains(memberAllowed, "<action>get_execution</action>") ||
		strings.Contains(memberAllowed, "<action>plan_execution</action>") {
		t.Fatalf("unmanaged Room member context = %s", member)
	}
}

func TestRenderConversationExecutionContextKeepsBackgroundWorkGraphUnbound(t *testing.T) {
	snapshot := executionContextTestSnapshot()
	rendered := RenderConversationExecutionContext(
		&snapshot,
		ExecutionContextOptions{
			ActorAgentID: "observer",
			ScopeKind:    protocol.ExecutionScopeRoom,
		},
	)
	for _, expected := range []string{
		`lane="conversation"`,
		`relation="background"`,
		`no trusted WorkBinding or ReviewBinding`,
		`<action>submit_work</action>`,
		`<action>Agent</action>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("conversation context missing %q:\n%s", expected, rendered)
		}
	}
	allowedStart := strings.Index(rendered, "<allowed_actions>")
	allowedEnd := strings.Index(rendered, "</allowed_actions>")
	allowed := rendered[allowedStart:allowedEnd]
	if !strings.Contains(allowed, "<action>Agent</action>") ||
		!strings.Contains(allowed, "<action>get_execution</action>") ||
		strings.Contains(allowed, "<action>submit_work</action>") ||
		strings.Contains(rendered, "<assigned_work>") ||
		strings.Contains(rendered, "<active_assignments>") {
		t.Fatalf("conversation round received WorkGraph authority:\n%s", rendered)
	}
}

func TestRenderExecutionContextMakesRoomObservationFullAndReadOnly(t *testing.T) {
	snapshot := executionContextTestSnapshot()
	rendered := RenderExecutionContext(
		&snapshot,
		ExecutionContextOptions{
			ActorAgentID: "observer",
			ScopeKind:    protocol.ExecutionScopeRoom,
			ObserveOnly:  true,
		},
	)
	for _, expected := range []string{
		`<lane type="observation" />`,
		`<graph_digest notation="nexus-dag-v1" scope="full"`,
		`shared Room WorkGraph observation only`,
		`<action>get_execution</action>`,
		`<action>submit_work</action>`,
		`<action>plan_execution</action>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("Room observation missing %q:\n%s", expected, rendered)
		}
	}
	allowed := rendered[strings.Index(
		rendered,
		"<allowed_actions>",
	):strings.Index(rendered, "</allowed_actions>")]
	if strings.Contains(allowed, "<action>submit_work</action>") ||
		strings.Contains(allowed, "<action>plan_execution</action>") ||
		strings.Contains(rendered, "<assigned_work>") ||
		strings.Contains(rendered, "<active_assignments>") {
		t.Fatalf("Room observation leaked mutation authority:\n%s", rendered)
	}
}

func TestRenderConversationExecutionContextGivesOnlyCoordinatorBootstrapActions(t *testing.T) {
	snapshot := executionContextTestSnapshot()
	rendered := RenderConversationExecutionContext(
		&snapshot,
		ExecutionContextOptions{
			ActorAgentID: "lead",
			Role:         ExecutionActorCoordinator,
			ScopeKind:    protocol.ExecutionScopeRoom,
		},
	)
	for _, expected := range []string{
		`role="coordinator" lane="conversation"`,
		`<coordination_transition available="true">`,
		`<action>get_execution</action>`,
		`<action>plan_execution</action>`,
		`participant count and raw mentions are never sufficient`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("conversation coordinator context missing %q:\n%s", expected, rendered)
		}
	}
	allowed := rendered[strings.Index(
		rendered,
		"<allowed_actions>",
	):strings.Index(rendered, "</allowed_actions>")]
	for _, forbidden := range []string{
		"assign_work",
		"submit_work",
		"review_work",
		"promote_execution_to_goal",
	} {
		if strings.Contains(allowed, "<action>"+forbidden+"</action>") {
			t.Fatalf("conversation coordinator received %q:\n%s", forbidden, rendered)
		}
	}
	if !strings.Contains(allowed, "<action>Agent</action>") {
		t.Fatalf("conversation coordinator lost native subagent affordance:\n%s", rendered)
	}
	if strings.Contains(rendered, "<assigned_work>") ||
		strings.Contains(rendered, "<active_assignments>") {
		t.Fatalf("conversation coordinator received WorkGraph projection:\n%s", rendered)
	}
}

func TestRenderExecutionContextExposesEligibleGoalPromotionWithSuggestedReason(t *testing.T) {
	snapshot := executionContextTestSnapshot()
	snapshot.Execution.GoalID = ""
	snapshot.Execution.GoalObjectiveRevision = 0
	eligible := RenderExecutionContext(&snapshot, ExecutionContextOptions{
		ActorAgentID: "lead",
		GoalPromotionReasons: []protocol.GoalActivationReason{
			protocol.GoalActivationReasonRoomDependencyChain,
		},
	})
	if !strings.Contains(eligible, `<goal_promotion eligible="true">`) ||
		!strings.Contains(eligible, `<activation_reason>room_dependency_chain</activation_reason>`) {
		t.Fatalf("eligible promotion context = %s", eligible)
	}
	eligibleAllowedStart := strings.Index(eligible, "<allowed_actions>")
	eligibleAllowedEnd := strings.Index(eligible, "</allowed_actions>")
	eligibleAllowed := eligible[eligibleAllowedStart:eligibleAllowedEnd]
	if !strings.Contains(eligibleAllowed, "<action>promote_execution_to_goal</action>") {
		t.Fatalf("eligible promotion action missing: %s", eligible)
	}

	blocked := RenderExecutionContext(&snapshot, ExecutionContextOptions{
		ActorAgentID:          "lead",
		GoalPromotionBlockers: []string{"automatic_goal_disabled"},
	})
	blockedAllowedStart := strings.Index(blocked, "<allowed_actions>")
	blockedAllowedEnd := strings.Index(blocked, "</allowed_actions>")
	blockedAllowed := blocked[blockedAllowedStart:blockedAllowedEnd]
	if !strings.Contains(blocked, `<goal_promotion eligible="false">`) ||
		!strings.Contains(blocked, `<blocker>automatic_goal_disabled</blocker>`) ||
		strings.Contains(blockedAllowed, "<action>promote_execution_to_goal</action>") {
		t.Fatalf("blocked promotion context = %s", blocked)
	}
}

func TestRenderExecutionContextPlanModeOnlyAllowsInspectionAndProposal(t *testing.T) {
	snapshot := executionContextTestSnapshot()
	rendered := RenderExecutionContext(&snapshot, ExecutionContextOptions{
		ActorAgentID: "lead",
		PlanMode:     true,
	})
	for _, expected := range []string{
		`<mode plan_only="true" />`,
		`<action>get_execution</action>`,
		`<action>plan_execution</action>`,
		`<action>assign_work</action>`,
		`<action>execute_work_in_plan_mode</action>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("Plan Mode context missing %q:\n%s", expected, rendered)
		}
	}
	allowedStart := strings.Index(rendered, "<allowed_actions>")
	allowedEnd := strings.Index(rendered, "</allowed_actions>")
	if allowedStart < 0 || allowedEnd < allowedStart {
		t.Fatalf("Plan Mode context missing allowed actions:\n%s", rendered)
	}
	allowed := rendered[allowedStart:allowedEnd]
	for _, forbidden := range []string{"assign_work", "submit_work", "review_work", "promote_execution_to_goal"} {
		if strings.Contains(allowed, "<action>"+forbidden+"</action>") {
			t.Fatalf("Plan Mode allowed executable action %q:\n%s", forbidden, rendered)
		}
	}
}

func TestRenderExecutionContextMakesExecutionTransitionAuthorityExplicit(t *testing.T) {
	transient := executionContextTestSnapshot()
	transient.Execution.GoalID = ""
	transient.Execution.GoalObjectiveRevision = 0

	coordinator := RenderExecutionContext(&transient, ExecutionContextOptions{
		ActorAgentID: "lead",
	})
	if !strings.Contains(
		coordinator,
		`<execution_transition replace_current_allowed="true" abandon_allowed="true" validation_only="false">`,
	) {
		t.Fatalf("transient coordinator transition = %s", coordinator)
	}
	coordinatorAllowed := coordinator[strings.Index(
		coordinator,
		"<allowed_actions>",
	):strings.Index(coordinator, "</allowed_actions>")]
	for _, action := range []string{"plan_execution", "abandon_execution"} {
		if !strings.Contains(coordinatorAllowed, "<action>"+action+"</action>") {
			t.Fatalf("transient coordinator missing %q:\n%s", action, coordinator)
		}
	}

	planOnly := RenderExecutionContext(&transient, ExecutionContextOptions{
		ActorAgentID: "lead",
		PlanMode:     true,
	})
	if !strings.Contains(
		planOnly,
		`<execution_transition replace_current_allowed="true" abandon_allowed="true" validation_only="true">`,
	) {
		t.Fatalf("Plan Mode transition is not validation-only:\n%s", planOnly)
	}

	goalBound := executionContextTestSnapshot()
	goalCoordinator := RenderExecutionContext(&goalBound, ExecutionContextOptions{
		ActorAgentID: "lead",
	})
	if !strings.Contains(
		goalCoordinator,
		`<execution_transition replace_current_allowed="false" abandon_allowed="false" validation_only="false" reason_code="goal_retarget_required">`,
	) {
		t.Fatalf("Goal-bound transition boundary = %s", goalCoordinator)
	}
	goalAllowed := goalCoordinator[strings.Index(
		goalCoordinator,
		"<allowed_actions>",
	):strings.Index(goalCoordinator, "</allowed_actions>")]
	if strings.Contains(goalAllowed, "<action>abandon_execution</action>") {
		t.Fatalf("Goal-bound coordinator can abandon transient Execution:\n%s", goalCoordinator)
	}

	member := RenderExecutionContext(&transient, ExecutionContextOptions{
		ActorAgentID: "analyst",
		Role:         ExecutionActorMember,
	})
	if !strings.Contains(
		member,
		`replace_current_allowed="false" abandon_allowed="false" validation_only="false" reason_code="wrong_owner"`,
	) {
		t.Fatalf("Room member transition authority = %s", member)
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
	if !strings.Contains(allowed, "<action>Agent</action>") {
		t.Fatalf("terminal background Execution blocked native subagent use:\n%s", rendered)
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

func TestRenderExecutionContextPublishesOnlyCurrentlyCallableOrchestrationActions(t *testing.T) {
	snapshot := executionContextTestSnapshot()
	member := RenderExecutionContext(&snapshot, ExecutionContextOptions{
		ActorAgentID: "observer",
		Role:         ExecutionActorMember,
	})
	memberAllowed := member[strings.Index(
		member,
		"<allowed_actions>",
	):strings.Index(member, "</allowed_actions>")]
	if !strings.Contains(
		member,
		"<action_scope>Use only allowed_actions; load execution-orchestrator and follow the exact round-scoped execution contract for the selected action.</action_scope>",
	) ||
		strings.Contains(memberAllowed, "<action>submit_work</action>") ||
		strings.Contains(memberAllowed, "<action>block_work</action>") {
		t.Fatalf("unassigned member actions are not exact:\n%s", member)
	}

	coordinator := RenderExecutionContext(&snapshot, ExecutionContextOptions{
		ActorAgentID: "lead",
	})
	if !strings.Contains(
		coordinator,
		`<item id="work-analysis" logical_key="W2" spec_id="spec-analysis" assignment_id="assignment-analysis" owner_agent_id="analyst"`,
	) {
		t.Fatalf("coordinator cannot see the active responsibility map:\n%s", coordinator)
	}
	for _, expected := range []string{
		`<objective>Synthesize accepted research</objective>`,
		`<criterion>Uses accepted sources</criterion>`,
		`<work_item_id>work-research</work_item_id>`,
	} {
		if !strings.Contains(coordinator, expected) {
			t.Fatalf("active Assignment context missing %q:\n%s", expected, coordinator)
		}
	}
	coordinatorAllowed := coordinator[strings.Index(
		coordinator,
		"<allowed_actions>",
	):strings.Index(coordinator, "</allowed_actions>")]
	for _, unavailable := range []string{
		"assign_work",
		"submit_work",
		"review_work",
		"promote_execution_to_goal",
	} {
		if strings.Contains(
			coordinatorAllowed,
			"<action>"+unavailable+"</action>",
		) {
			t.Fatalf("coordinator advertised unavailable %q:\n%s", unavailable, coordinator)
		}
	}
	for _, available := range []string{"plan_execution", "block_work", "take_over_work"} {
		if !strings.Contains(
			coordinatorAllowed,
			"<action>"+available+"</action>",
		) {
			t.Fatalf("coordinator omitted available %q:\n%s", available, coordinator)
		}
	}
	if !strings.Contains(coordinator, `<plan_revision available="true" guarded="true">`) ||
		!strings.Contains(
			coordinator,
			"replacing a different active Plan requires supersede_active_work=true and a non-empty revision_reason",
		) {
		t.Fatalf("coordinator guarded replan boundary is missing:\n%s", coordinator)
	}

	unreviewed := cloneExecutionSnapshot(&snapshot)
	unreviewed.Submissions = append(unreviewed.Submissions, protocol.WorkSubmission{
		ID:               "submission-analysis",
		ExecutionID:      "execution-1",
		PlanID:           "plan-1",
		WorkItemID:       "work-analysis",
		SpecID:           "spec-analysis",
		AssignmentID:     "assignment-analysis",
		SubmitterAgentID: "analyst",
		Sequence:         1,
	})
	blocked := RenderExecutionContext(unreviewed, ExecutionContextOptions{
		ActorAgentID: "lead",
		ScopeKind:    protocol.ExecutionScopeRoom,
		ReviewBound:  true,
	})
	blockedAllowed := blocked[strings.Index(
		blocked,
		"<allowed_actions>",
	):strings.Index(blocked, "</allowed_actions>")]
	if strings.Contains(blockedAllowed, "<action>plan_execution</action>") ||
		strings.Contains(blockedAllowed, "<action>block_work</action>") ||
		strings.Contains(blockedAllowed, "<action>take_over_work</action>") ||
		!strings.Contains(blockedAllowed, "<action>review_work</action>") ||
		!strings.Contains(blocked, `<plan_revision available="false" guarded="true">`) ||
		!strings.Contains(blocked, "review every unreviewed Submission before replacing the active Plan") {
		t.Fatalf("unreviewed Submission did not lock responsibility-changing actions:\n%s", blocked)
	}
}

func TestRenderExecutionContextKeepsSafeParallelResponsibilityActions(t *testing.T) {
	snapshot := executionContextTestSnapshot()
	snapshot.Dependencies = snapshot.Dependencies[:1]
	snapshot.WorkItemSpecs[2].Objective = "Integrate the independent appendix"
	snapshot.WorkItemSpecs[2].AcceptanceCriteria = []string{"Appendix is verified"}
	snapshot.Assignments = append(snapshot.Assignments, protocol.WorkAssignment{
		ID:           "assignment-integration",
		ExecutionID:  "execution-1",
		PlanID:       "plan-1",
		WorkItemID:   "work-integration",
		SpecID:       "spec-integration",
		OwnerAgentID: "integrator",
		Status:       protocol.WorkAssignmentStatusActive,
	})
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
		ResultSummary:    "analysis ready for review",
	})

	rendered := RenderExecutionContext(&snapshot, ExecutionContextOptions{
		ActorAgentID: "lead",
		ScopeKind:    protocol.ExecutionScopeRoom,
		ReviewBound:  true,
	})
	allowed := rendered[strings.Index(
		rendered,
		"<allowed_actions>",
	):strings.Index(rendered, "</allowed_actions>")]
	for _, expected := range []string{
		"<action>review_work</action>",
		"<action>block_work</action>",
		"<action>take_over_work</action>",
	} {
		if !strings.Contains(allowed, expected) {
			t.Fatalf("safe parallel action %q was hidden:\n%s", expected, rendered)
		}
	}
	if strings.Contains(allowed, "<action>plan_execution</action>") {
		t.Fatalf("pending review incorrectly allowed Plan replacement:\n%s", rendered)
	}
	for _, expected := range []string{
		`status="submitted" responsibility_mutation_allowed="false" pending_submission_id="submission-analysis"`,
		`status="active" responsibility_mutation_allowed="true"`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("per-item review lock missing %q:\n%s", expected, rendered)
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

func TestRenderExecutionContextLetsReleasedCurrentSpecOwnerResume(t *testing.T) {
	snapshot := executionContextTestSnapshot()
	snapshot.WorkItemStates[1].Status = protocol.WorkItemStatusWaitingInput
	snapshot.WorkItemStates[1].BlockReason = "approval missing"
	snapshot.WorkItemStates[1].NeededInput = "approval-42"
	snapshot.Assignments[1].Status = protocol.WorkAssignmentStatusReleased
	snapshot.Attempts = nil

	owner := RenderExecutionContext(&snapshot, ExecutionContextOptions{
		ActorAgentID: "analyst",
		Role:         ExecutionActorMember,
	})
	ownerAllowed := owner[strings.Index(
		owner,
		"<allowed_actions>",
	):strings.Index(owner, "</allowed_actions>")]
	for _, expected := range []string{
		`<item id="work-analysis" logical_key="W2" spec_id="spec-analysis" assignment_id="assignment-analysis" owner_agent_id="analyst" assignment_status="released">`,
		`<reason>approval missing</reason>`,
		`<needed_input>approval-42</needed_input>`,
		`<action>resume_work</action>`,
	} {
		if !strings.Contains(owner, expected) {
			t.Fatalf("released owner recovery context missing %q:\n%s", expected, owner)
		}
	}
	if strings.Contains(ownerAllowed, "<action>submit_work</action>") ||
		strings.Contains(ownerAllowed, "<action>block_work</action>") {
		t.Fatalf("released owner retained live Assignment actions:\n%s", owner)
	}

	observer := RenderExecutionContext(&snapshot, ExecutionContextOptions{
		ActorAgentID: "observer",
		Role:         ExecutionActorMember,
	})
	observerAllowed := observer[strings.Index(
		observer,
		"<allowed_actions>",
	):strings.Index(observer, "</allowed_actions>")]
	if strings.Contains(observerAllowed, "<action>resume_work</action>") ||
		strings.Contains(observer, `logical_key="W2" spec_id="spec-analysis"`) {
		t.Fatalf("unrelated member saw released owner recovery affordance:\n%s", observer)
	}

	coordinator := RenderExecutionContext(&snapshot, ExecutionContextOptions{
		ActorAgentID: "lead",
	})
	coordinatorAllowed := coordinator[strings.Index(
		coordinator,
		"<allowed_actions>",
	):strings.Index(coordinator, "</allowed_actions>")]
	if !strings.Contains(coordinatorAllowed, "<action>resume_work</action>") ||
		!strings.Contains(coordinator, `assignment_status="released"`) {
		t.Fatalf("coordinator recovery affordance is missing:\n%s", coordinator)
	}
}

func TestRenderExecutionContextProjectsManagedAndRuntimeOnlySubagentModes(t *testing.T) {
	unique := assignedExecutionSnapshot()
	rendered := RenderExecutionContext(unique, ExecutionContextOptions{
		ActorAgentID: "agent-worker",
		Role:         ExecutionActorMember,
	})
	for _, expected := range []string{
		`<subagent_admission eligible="true" native_tool="Agent" candidate_assignment_count="1" binding_mode="managed" assignment_id="assignment-1" work_item_id="work-1" parent_attempt_id="attempt-1" />`,
		`<action>Agent</action>`,
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
		`<subagent_admission eligible="true" native_tool="Agent" candidate_assignment_count="2" binding_mode="runtime_only" managed_binding_reason="ambiguous_assignment">`,
		`<note>native delegation is available, but this run is runtime observation only and does not claim managed Work Item evidence: the current Agent has multiple delegable Assignments; select one through the WorkGraph before launching a subagent</note>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("ambiguous subagent context missing %q:\n%s", expected, rendered)
		}
	}
	allowed := rendered[strings.Index(rendered, "<allowed_actions>"):strings.Index(rendered, "</allowed_actions>")]
	if !strings.Contains(allowed, "<action>Agent</action>") {
		t.Fatalf("runtime-only mode hid native Agent affordance:\n%s", rendered)
	}

	rendered = RenderExecutionContext(unique, ExecutionContextOptions{
		ActorAgentID: "agent-worker",
		Role:         ExecutionActorMember,
		PlanMode:     true,
	})
	if !strings.Contains(
		rendered,
		`<subagent_admission eligible="false" native_tool="Agent" candidate_assignment_count="1" reason_code="plan_mode">`,
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
