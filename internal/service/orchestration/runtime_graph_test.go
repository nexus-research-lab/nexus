package orchestration

import (
	"context"
	"strings"
	"testing"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type runtimeGraphRepositoryFake struct {
	*fakeRepository
	nodes            []protocol.ExecutionRuntimeNodeRun
	edges            []protocol.ExecutionRuntimeEdgeRun
	artifacts        []protocol.ExecutionRuntimeArtifactRef
	boundRoundID     string
	boundExecution   string
	reconciled       int
	finishedStatus   protocol.ExecutionRuntimeNodeStatus
	graph            protocol.ExecutionRuntimeGraph
	getGraphCalls    int
	graphExecutionID string
	graphRootRoundID string
}

func (f *runtimeGraphRepositoryFake) FindCurrentManaged(ctx context.Context, owner, session string) (*protocol.Execution, error) {
	if f.snapshot == nil || f.snapshot.Plan == nil || len(f.snapshot.WorkItems) == 0 {
		return nil, nil
	}
	return f.FindCurrent(ctx, owner, session)
}

func (f *runtimeGraphRepositoryFake) FindLatestManaged(ctx context.Context, owner, session string) (*protocol.Execution, error) {
	return f.FindCurrentManaged(ctx, owner, session)
}

func (f *runtimeGraphRepositoryFake) GetWorkGraphState(ctx context.Context, id string) (*protocol.ExecutionSnapshot, protocol.ExecutionWorkGraphHistory, error) {
	snapshot, err := f.GetSnapshot(ctx, id)
	return snapshot, protocol.ExecutionWorkGraphHistory{}, err
}

func (f *runtimeGraphRepositoryFake) GetWorkGraphRuntimeGraph(ctx context.Context, owner, session, execution, round string) (protocol.ExecutionRuntimeGraph, error) {
	return f.GetRuntimeGraph(ctx, owner, session, execution, round)
}

func graphNodeByID(
	nodes []protocol.ExecutionGraphNodeView,
	id string,
) protocol.ExecutionGraphNodeView {
	for _, node := range nodes {
		if node.ID == id {
			return node
		}
	}
	return protocol.ExecutionGraphNodeView{}
}

func hasExecutionGraphEdge(
	edges []protocol.ExecutionGraphEdgeView,
	kind protocol.ExecutionGraphEdgeKind,
	sourceID string,
	targetID string,
) bool {
	for _, edge := range edges {
		if edge.Kind == kind &&
			edge.SourceNodeID == sourceID &&
			edge.TargetNodeID == targetID {
			return true
		}
	}
	return false
}

func (f *runtimeGraphRepositoryFake) UpsertRuntimeGraphNode(
	_ context.Context,
	item protocol.ExecutionRuntimeNodeRun,
) error {
	f.nodes = append(f.nodes, item)
	return nil
}

func (f *runtimeGraphRepositoryFake) UpsertRuntimeGraphEdge(
	_ context.Context,
	item protocol.ExecutionRuntimeEdgeRun,
) error {
	f.edges = append(f.edges, item)
	return nil
}

func (f *runtimeGraphRepositoryFake) UpsertRuntimeGraphArtifact(
	_ context.Context,
	item protocol.ExecutionRuntimeArtifactRef,
) error {
	f.artifacts = append(f.artifacts, item)
	return nil
}

func (f *runtimeGraphRepositoryFake) BindRuntimeGraphRoundExecution(
	_ context.Context,
	_ string,
	_ string,
	agentRoundID string,
	executionID string,
) error {
	f.boundRoundID = agentRoundID
	f.boundExecution = executionID
	return nil
}

func (f *runtimeGraphRepositoryFake) ReconcileRuntimeGraphAgent(
	context.Context,
	string,
	string,
	string,
	string,
	time.Time,
) error {
	f.reconciled++
	return nil
}

func (f *runtimeGraphRepositoryFake) FinishRuntimeGraphRound(
	_ context.Context,
	_ string,
	_ string,
	_ string,
	status protocol.ExecutionRuntimeNodeStatus,
	_ time.Time,
) error {
	f.finishedStatus = status
	return nil
}

func (f *runtimeGraphRepositoryFake) GetRuntimeGraph(
	_ context.Context,
	_ string,
	_ string,
	executionID string,
	rootRoundID string,
) (protocol.ExecutionRuntimeGraph, error) {
	f.getGraphCalls++
	f.graphExecutionID = executionID
	f.graphRootRoundID = rootRoundID
	return f.graph, nil
}

func TestRuntimeContextLoadsRuntimeFactsOnlyForManagedExecution(t *testing.T) {
	roomSnapshot, binding := structuredRoomWorkBindingSnapshot()
	conversationActor := structuredRoomMemberActor(binding)
	conversationActor.WorkBinding = nil
	observationActor := conversationActor
	observationActor.ObservationOnly = true
	tests := []struct {
		name       string
		snapshot   *protocol.ExecutionSnapshot
		actor      ActorContext
		wantGraph  bool
		contextTag string
	}{
		{
			name:       "unmanaged DM",
			actor:      coordinatorActor(),
			contextTag: `<nexus_round lane="coordination" role="coordinator" />`,
		},
		{
			name:       "Room conversation with background Execution",
			snapshot:   roomSnapshot,
			actor:      conversationActor,
			contextTag: `<nexus_round lane="conversation" role="member" execution="background" />`,
		},
		{
			name:       "bound Room work",
			snapshot:   roomSnapshot,
			actor:      structuredRoomMemberActor(binding),
			wantGraph:  true,
			contextTag: `<assigned_work>`,
		},
		{
			name:       "Room observation",
			snapshot:   roomSnapshot,
			actor:      observationActor,
			wantGraph:  true,
			contextTag: `<lane type="observation" />`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &runtimeGraphRepositoryFake{
				fakeRepository: &fakeRepository{snapshot: test.snapshot},
				graph: protocol.ExecutionRuntimeGraph{
					GraphID: "execution:execution-1",
					Nodes: []protocol.ExecutionRuntimeNodeRun{{
						ID:      "runtime-tool-1",
						Kind:    protocol.ExecutionRuntimeNodeTool,
						AgentID: test.actor.AgentID,
						Name:    "search",
						Status:  protocol.ExecutionRuntimeNodeRunning,
					}},
					NodeTotal: 1,
				},
			}

			rendered, err := NewService(repository).RuntimeContext(
				context.Background(),
				test.actor,
			)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(rendered, test.contextTag) {
				t.Fatalf("runtime context missing %q:\n%s", test.contextTag, rendered)
			}
			if got := strings.Contains(rendered, "<runtime_facts"); got != test.wantGraph {
				t.Fatalf("runtime facts present = %t, want %t:\n%s", got, test.wantGraph, rendered)
			}
			wantCalls := 0
			if test.wantGraph {
				wantCalls = 1
				if !strings.Contains(rendered, `id="runtime-tool-1"`) ||
					repository.graphExecutionID != test.snapshot.Execution.ID {
					t.Fatalf("managed Runtime Graph was not preserved:\n%s", rendered)
				}
			}
			if repository.getGraphCalls != wantCalls {
				t.Fatalf("GetRuntimeGraph calls = %d, want %d", repository.getGraphCalls, wantCalls)
			}
		})
	}
}

func TestRuntimeCommandReceiptsReconcileCLITransportInOneGraphRead(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 18, 16, 0, 0, 0, time.UTC)
	actor := ActorContext{
		OwnerUserID: "owner-1", SessionKey: "session-1", ExecutionID: "execution-1",
		AgentID: "agent-1", Role: ExecutionActorCoordinator,
		ActorKind: protocol.ExecutionActorAgent, ScopeKind: protocol.ExecutionScopeDM,
		RootRoundID: "round-1", RuntimeRoundID: "round-1", AgentRoundID: "agent-round-1",
	}
	identity, err := runtimeGraphIdentityFromActor(actor)
	if err != nil {
		t.Fatal(err)
	}
	candidate := protocol.ExecutionRuntimeNodeRun{
		ID:      runtimeGraphNodeID(identity, protocol.ExecutionRuntimeNodeTool, "tool-assign"),
		GraphID: identity.GraphID, OwnerUserID: actor.OwnerUserID, SessionKey: actor.SessionKey,
		ExecutionID: actor.ExecutionID, Kind: protocol.ExecutionRuntimeNodeTool,
		SubjectID: "tool-assign", ParentSubjectID: actor.AgentRoundID,
		RootRoundID: actor.RootRoundID, RuntimeRoundID: actor.RuntimeRoundID,
		AgentRoundID: actor.AgentRoundID, AgentID: actor.AgentID, Name: "Bash",
		Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now.Add(-time.Second),
		UpdatedAt: now.Add(-time.Second), Metadata: map[string]any{
			runtimeGraphCommandDomainMetadataKey:    command.DomainExecution,
			runtimeGraphCommandOperationMetadataKey: "assign_work",
			runtimeGraphCommandRequestIDMetadataKey: "assign-request-1",
		},
	}
	repository := &runtimeGraphRepositoryFake{
		fakeRepository: &fakeRepository{snapshot: func() *protocol.ExecutionSnapshot {
			snapshot := assignedExecutionSnapshot()
			snapshot.Execution.CoordinatorAgentID = actor.AgentID
			snapshot.Assignments[0].OwnerAgentID = actor.AgentID
			snapshot.Assignments[0].Strategy = protocol.AssignmentStrategySelf
			snapshot.Attempts[0].ExecutorAgentID = actor.AgentID
			snapshot.Attempts[0].RootRoundID = actor.RootRoundID
			snapshot.Attempts[0].RuntimeRoundID = actor.RuntimeRoundID
			snapshot.Attempts[0].AgentRoundID = actor.AgentRoundID
			return snapshot
		}()},
		graph: protocol.ExecutionRuntimeGraph{Nodes: []protocol.ExecutionRuntimeNodeRun{candidate}},
	}
	service := NewService(repository)
	service.now = func() time.Time { return now }
	err = service.ObserveRuntimeCommands(context.Background(), actor, []RuntimeCommandFact{
		{
			Operation: "assign_work",
			RequestID: "assign-request-1", Outcome: string(protocol.MutationResultApplied),
			ExecutionID: "execution-1",
			Changed:     []string{"assignment:assignment-1", "attempt:attempt-1"},
		},
		{
			Operation: "submit_work",
			RequestID: "submit-request-1", Outcome: string(protocol.MutationResultApplied),
			ExecutionID: "execution-1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.getGraphCalls != 1 {
		t.Fatalf("GetRuntimeGraph calls = %d, want one batched read", repository.getGraphCalls)
	}
	if repository.graphExecutionID != actor.ExecutionID ||
		repository.graphRootRoundID != actor.RootRoundID {
		t.Fatalf(
			"GetRuntimeGraph identity = %q/%q, want %q/%q",
			repository.graphExecutionID,
			repository.graphRootRoundID,
			actor.ExecutionID,
			actor.RootRoundID,
		)
	}
	if repository.boundRoundID != actor.AgentRoundID || repository.boundExecution != actor.ExecutionID {
		t.Fatalf("runtime segment binding = %q/%q", repository.boundRoundID, repository.boundExecution)
	}
	var assignNode, submitNode *protocol.ExecutionRuntimeNodeRun
	for index := range repository.nodes {
		node := &repository.nodes[index]
		switch node.Name {
		case "assign_work":
			assignNode = node
		case "submit_work":
			submitNode = node
		}
	}
	if assignNode == nil || !runtimeGraphMetadataBool(*assignNode, runtimeGraphCommandVerifiedMetadataKey) ||
		runtimeGraphMetadataString(*assignNode, runtimeGraphAssignmentIDMetadataKey) != "assignment-1" {
		t.Fatalf("verified assign node = %+v", assignNode)
	}
	if submitNode == nil || !runtimeGraphIsSubmissionTool(submitNode.Name) {
		t.Fatalf("submit review anchor = %+v", submitNode)
	}
}

func TestRuntimeGraphMarksStructuredRuntimeCommandIdentity(t *testing.T) {
	t.Parallel()
	message, err := sdkprotocol.DecodeMessage(map[string]any{
		"type": "assistant", "uuid": "assistant-structured-command",
		"message": map[string]any{
			"role": "assistant",
			"content": []any{map[string]any{
				"type": "tool_use", "id": "tool-runtime-1", "name": "mcp__nexus__command",
				"input": map[string]any{
					"domain": "execution", "action": "invoke", "operation": "assign_work",
					"request_id": "assign-structured-1", "input": map[string]any{"work_item_id": "private"},
				},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	events := runtimeGraphLifecycleEvents(message)
	if len(events) != 1 {
		t.Fatalf("lifecycle events = %+v", events)
	}
	metadata := events[0].Metadata
	if metadata[runtimeGraphCommandTransportMetadataKey] != "true" ||
		metadata[runtimeGraphCommandDomainMetadataKey] != "execution" ||
		metadata[runtimeGraphCommandActionMetadataKey] != "invoke" ||
		metadata[runtimeGraphCommandOperationMetadataKey] != "assign_work" ||
		metadata[runtimeGraphCommandRequestIDMetadataKey] != "assign-structured-1" {
		t.Fatalf("structured command metadata = %+v", metadata)
	}
	if _, leaked := metadata["input"]; leaked {
		t.Fatalf("structured command leaked business input: %+v", metadata)
	}
}

func TestRuntimeGraphRecoversSubagentToolLifecycleFromAttachment(t *testing.T) {
	t.Parallel()

	message, err := sdkprotocol.DecodeMessage(map[string]any{
		"type": "attachment",
		"uuid": "subagent-result-1",
		"attachment": map[string]any{
			"type": "structured_output",
			"data": map[string]any{
				"toolUseId": "spawn-tool-1",
				"messages": []any{
					map[string]any{
						"type": "assistant", "uuid": "child-assistant-1",
						"message": map[string]any{
							"role": "assistant",
							"content": []any{map[string]any{
								"type": "tool_use", "id": "child-tool-1", "name": "Read",
								"input": map[string]any{"file_path": "/private/input"},
							}},
						},
					},
					map[string]any{
						"type": "user", "uuid": "child-result-1",
						"message": map[string]any{
							"role": "user",
							"content": []any{map[string]any{
								"type": "tool_result", "tool_use_id": "child-tool-1",
								"content": "private output",
							}},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	events := runtimeGraphLifecycleEvents(message)
	if len(events) != 2 ||
		events[0].NodeKind != sdkprotocol.RuntimeLifecycleNodeTool ||
		events[0].Phase != sdkprotocol.RuntimeLifecycleStarted ||
		events[0].SubjectID != "child-tool-1" ||
		events[0].ParentSubjectID != "spawn-tool-1" ||
		events[0].Name != "Read" ||
		events[1].Phase != sdkprotocol.RuntimeLifecycleFinished ||
		events[1].ParentSubjectID != "spawn-tool-1" ||
		events[1].Status != "succeeded" {
		t.Fatalf("subagent Tool lifecycle = %+v", events)
	}
	if events[0].Metadata != nil {
		t.Fatalf("subagent Tool input leaked into metadata: %+v", events[0].Metadata)
	}
}

func TestRuntimeGraphPersistsDMSelfAssignmentSegmentAcrossTools(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 16, 0, 0, 0, time.UTC)
	snapshot := assignedExecutionSnapshot()
	snapshot.Execution.ScopeKind = protocol.ExecutionScopeDM
	snapshot.Execution.CoordinatorAgentID = "agent-lead"
	// The WorkGraph may have been created in an earlier physical round; the
	// Assignment's root Attempt, not Execution creation time, binds this segment.
	snapshot.Execution.RootRoundID = "round-before"
	snapshot.Assignments[0].OwnerAgentID = "agent-lead"
	snapshot.Assignments[0].Strategy = protocol.AssignmentStrategySelf
	snapshot.Attempts[0].ExecutorAgentID = "agent-lead"
	snapshot.Attempts[0].RootRoundID = "round-1"
	snapshot.Attempts[0].RuntimeRoundID = "round-1"
	snapshot.Attempts[0].AgentRoundID = "agent-round-1"
	snapshot.Attempts[0].CreatedAt = now
	actor := ActorContext{
		OwnerUserID: "owner-1", SessionKey: "session-1",
		// The managed Execution is created inside this already-running DM round,
		// so the observer actor starts without an ExecutionID and adopts only the
		// exact server-issued receipt after validating its snapshot.
		ExecutionID: "", AgentID: "agent-lead",
		Role: ExecutionActorCoordinator, ScopeKind: protocol.ExecutionScopeDM,
		RootRoundID: "round-1", RuntimeRoundID: "round-1",
		AgentRoundID: "agent-round-1",
	}
	identity, err := runtimeGraphIdentityFromActor(actor)
	if err != nil {
		t.Fatal(err)
	}
	assignNode := protocol.ExecutionRuntimeNodeRun{
		ID:   runtimeGraphNodeID(identity, protocol.ExecutionRuntimeNodeTool, "tool-assign"),
		Kind: protocol.ExecutionRuntimeNodeTool, SubjectID: "tool-assign",
		AgentRoundID: "agent-round-1", AgentID: "agent-lead",
		Name:      "Bash",
		Status:    protocol.ExecutionRuntimeNodeRunning,
		StartedAt: now.Add(-time.Second), UpdatedAt: now.Add(-time.Second),
		Metadata: map[string]any{runtimeGraphCommandTransportMetadataKey: true},
	}
	repository := &runtimeGraphRepositoryFake{
		fakeRepository: &fakeRepository{snapshot: snapshot},
		graph:          protocol.ExecutionRuntimeGraph{Nodes: []protocol.ExecutionRuntimeNodeRun{assignNode}},
	}
	service := NewService(repository)
	service.now = func() time.Time { return now.Add(time.Second) }
	message, err := sdkprotocol.DecodeMessage(map[string]any{
		"type": "user", "uuid": "result-assign",
		"message": map[string]any{
			"role": "user",
			"content": []any{map[string]any{
				"type": "tool_result", "tool_use_id": "tool-assign",
				"content": `{"domain":"execution","action":"invoke","operation":"assign_work","request_id":"assign-request-1","result":{"data":{"outcome":"applied","execution_id":"execution-1","changed":["assignment:assignment-1","attempt:attempt-1"]}}}`,
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.ObserveRuntimeMessage(context.Background(), actor, message); err != nil {
		t.Fatal(err)
	}
	finishedAssign := repository.nodes[len(repository.nodes)-1]
	segment := runtimeExecutionSegmentFromNode(finishedAssign)
	if !segment.valid() || segment.ExecutionID != "execution-1" ||
		segment.WorkItemID != "work-1" || segment.AssignmentID != "assignment-1" ||
		segment.AttemptID != "attempt-1" || segment.Source != "assign_work_receipt" {
		t.Fatalf("persisted assign_work segment = %+v node=%+v", segment, finishedAssign)
	}
	if finishedAssign.ExecutionID != "execution-1" {
		t.Fatalf("new in-round Execution identity was not adopted: %+v", finishedAssign)
	}
	if repository.boundRoundID != "agent-round-1" ||
		repository.boundExecution != "execution-1" {
		t.Fatalf(
			"round binding = %q/%q, want agent-round-1/execution-1",
			repository.boundRoundID,
			repository.boundExecution,
		)
	}

	// A later provider message has no WorkBinding of its own. It must inherit
	// the exact persisted segment instead of falling back to AgentRoundID.
	finishedAssign.Name = assignNode.Name
	repository.graph = protocol.ExecutionRuntimeGraph{Nodes: []protocol.ExecutionRuntimeNodeRun{finishedAssign}}
	service.now = func() time.Time { return now.Add(2 * time.Second) }
	nextMessage, err := sdkprotocol.DecodeMessage(map[string]any{
		"type": "assistant", "uuid": "assistant-write",
		"message": map[string]any{
			"role": "assistant",
			"content": []any{map[string]any{
				"type": "tool_use", "id": "tool-write", "name": "Write",
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.ObserveRuntimeMessage(context.Background(), actor, nextMessage); err != nil {
		t.Fatal(err)
	}
	writeSegment := runtimeExecutionSegmentFromNode(repository.nodes[len(repository.nodes)-1])
	if writeSegment.ExecutionID != segment.ExecutionID ||
		writeSegment.WorkItemID != segment.WorkItemID ||
		writeSegment.AssignmentID != segment.AssignmentID ||
		writeSegment.AttemptID != segment.AttemptID {
		t.Fatalf("inherited Write segment = %+v, want %+v", writeSegment, segment)
	}
}

func TestRuntimeGraphUnresolvedSuccessfulAssignmentStopsPreviousDMSegment(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 16, 30, 0, 0, time.UTC)
	identity := runtimeGraphIdentity{
		ExecutionID: "execution-1", AgentRoundID: "agent-round-1", AgentID: "agent-1",
	}
	segment := runtimeExecutionSegment{
		ExecutionID: "execution-1", WorkItemID: "work-a",
		AssignmentID: "assignment-a", AttemptID: "attempt-a",
		Source: "assign_work_receipt",
	}
	firstMetadata := make(map[string]any)
	applyRuntimeExecutionSegment(firstMetadata, segment)
	firstMetadata[runtimeGraphSegmentBoundaryKey] = runtimeGraphSegmentBoundaryAssign
	unresolvedMetadata := map[string]any{
		runtimeGraphSegmentBoundaryKey: runtimeGraphSegmentBoundaryUnresolved,
	}
	nodes := []protocol.ExecutionRuntimeNodeRun{
		{
			ID: "tool-assign-a", Kind: protocol.ExecutionRuntimeNodeTool,
			AgentRoundID: "agent-round-1", AgentID: "agent-1",
			Name:      "assign_work",
			Status:    protocol.ExecutionRuntimeNodeSucceeded,
			StartedAt: now, UpdatedAt: now, Metadata: firstMetadata,
		},
		{
			ID: "tool-assign-unknown", Kind: protocol.ExecutionRuntimeNodeTool,
			AgentRoundID: "agent-round-1", AgentID: "agent-1",
			Name:      "assign_work",
			Status:    protocol.ExecutionRuntimeNodeSucceeded,
			StartedAt: now.Add(time.Second), UpdatedAt: now.Add(2 * time.Second),
			Metadata: unresolvedMetadata,
		},
	}
	if got := latestRuntimeExecutionSegment(nodes, identity); got.valid() {
		t.Fatalf("unresolved successful assign leaked previous segment after restart: %+v", got)
	}
}

func TestRuntimeGraphViewKeepsBlockedResumeAttemptsAndArtifactsInExactDMSegments(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 19, 13, 0, 0, 0, time.UTC)
	view := &protocol.ExecutionView{
		ID: "execution-cindy", ScopeKind: protocol.ExecutionScopeDM,
		CoordinatorAgentID: "agent-cindy",
		WorkItems: []protocol.ExecutionWorkItemView{
			{
				ID: "work-left", Position: 0, OwnerAgentID: "agent-cindy",
				Attempts: []protocol.ExecutionAttemptView{
					{
						ID: "attempt-left-1", AssignmentID: "assignment-left-1",
						ExecutorKind: protocol.AttemptExecutorAgent, ExecutorAgentID: "agent-cindy",
						CreatedAt: now.Add(2 * time.Second),
					},
					{
						ID: "attempt-left-2", AssignmentID: "assignment-left-2",
						ExecutorKind: protocol.AttemptExecutorAgent, ExecutorAgentID: "agent-cindy",
						AgentRoundID: "agent-round-cindy", CreatedAt: now.Add(10 * time.Second),
					},
				},
			},
			{
				ID: "work-right", Position: 1, OwnerAgentID: "agent-cindy",
				Attempts: []protocol.ExecutionAttemptView{{
					ID: "attempt-right", AssignmentID: "assignment-right",
					ExecutorKind: protocol.AttemptExecutorAgent, ExecutorAgentID: "agent-cindy",
					AgentRoundID: "agent-round-cindy", CreatedAt: now.Add(6 * time.Second),
				}},
			},
			{
				ID: "work-merge", Position: 2, OwnerAgentID: "agent-cindy",
				Attempts: []protocol.ExecutionAttemptView{{
					ID: "attempt-merge", AssignmentID: "assignment-merge",
					ExecutorKind: protocol.AttemptExecutorAgent, ExecutorAgentID: "agent-cindy",
					AgentRoundID: "agent-round-cindy", CreatedAt: now.Add(14 * time.Second),
				}},
			},
		},
	}
	view.Graph = projectExecutionGraphView(view.WorkItems)
	finishedAt := func(offset time.Duration) *time.Time {
		value := now.Add(offset)
		return &value
	}
	root := protocol.ExecutionRuntimeNodeRun{
		ID: "runtime-cindy", Kind: protocol.ExecutionRuntimeNodeAgent,
		SubjectID: "agent-round-cindy", AgentRoundID: "agent-round-cindy", AgentID: "agent-cindy",
		Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now, UpdatedAt: now,
		Metadata: map[string]any{"execution_lane": "coordination"},
	}
	assignmentBoundary := func(id, requestID string, start, finish time.Duration) protocol.ExecutionRuntimeNodeRun {
		return protocol.ExecutionRuntimeNodeRun{
			ID: id, Kind: protocol.ExecutionRuntimeNodeTool, SubjectID: id,
			AgentRoundID: "agent-round-cindy", AgentID: "agent-cindy", Name: "Bash",
			Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now.Add(start),
			UpdatedAt: now.Add(finish), FinishedAt: finishedAt(finish),
			Metadata: map[string]any{
				runtimeGraphCommandTransportMetadataKey: true,
				runtimeGraphCommandDomainMetadataKey:    command.DomainExecution,
				runtimeGraphCommandOperationMetadataKey: "assign_work",
				runtimeGraphCommandRequestIDMetadataKey: requestID,
			},
		}
	}
	receiptCompanion := func(id, requestID string, offset time.Duration) protocol.ExecutionRuntimeNodeRun {
		return protocol.ExecutionRuntimeNodeRun{
			ID: id, Kind: protocol.ExecutionRuntimeNodeTool, SubjectID: "runtime-command:" + requestID,
			AgentRoundID: "agent-round-cindy", AgentID: "agent-cindy", Name: "assign_work",
			Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now.Add(offset),
			UpdatedAt: now.Add(offset), FinishedAt: finishedAt(offset),
			Metadata: map[string]any{
				runtimeGraphCommandTransportMetadataKey: true,
				runtimeGraphCommandDomainMetadataKey:    command.DomainExecution,
				runtimeGraphCommandOperationMetadataKey: "assign_work",
				runtimeGraphCommandRequestIDMetadataKey: requestID,
				runtimeGraphCommandVerifiedMetadataKey:  true,
			},
		}
	}
	artifactWrite := func(id, path string, offset time.Duration) protocol.ExecutionRuntimeNodeRun {
		return protocol.ExecutionRuntimeNodeRun{
			ID: id, Kind: protocol.ExecutionRuntimeNodeTool, SubjectID: id,
			AgentRoundID: "agent-round-cindy", AgentID: "agent-cindy", Name: "Write",
			Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now.Add(offset), UpdatedAt: now.Add(offset),
			Artifacts: []protocol.WorkspaceFileArtifactBlock{{
				Type: protocol.ContentBlockTypeWorkspaceFileArtifact, Path: path, SourceToolUseID: id,
			}},
		}
	}
	stagingPath := "/private/state/users/owner/runtime/tmp/runtime-command-inputs/0123456789abcdef0123456789abcdef/input.json"
	nodes := []protocol.ExecutionRuntimeNodeRun{
		root,
		assignmentBoundary("tool-assign-left-1", "assign-left-1", time.Second, 3*time.Second),
		receiptCompanion("receipt-assign-left-1", "assign-left-1", 3200*time.Millisecond),
		{
			ID: "tool-staging", Kind: protocol.ExecutionRuntimeNodeTool, SubjectID: "tool-staging",
			AgentRoundID: "agent-round-cindy", AgentID: "agent-cindy", Name: "Write",
			Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now.Add(3500 * time.Millisecond),
			UpdatedAt: now.Add(3500 * time.Millisecond), ResultSummary: "updated " + stagingPath,
		},
		assignmentBoundary("tool-assign-right", "assign-right", 5*time.Second, 7*time.Second),
		receiptCompanion("receipt-assign-right", "assign-right", 7200*time.Millisecond),
		artifactWrite("tool-write-right", "right.md", 8*time.Second),
		assignmentBoundary("tool-assign-left-2", "assign-left-2", 9*time.Second, 11*time.Second),
		receiptCompanion("receipt-assign-left-2", "assign-left-2", 11200*time.Millisecond),
		artifactWrite("tool-write-left", "left.md", 12*time.Second),
		assignmentBoundary("tool-assign-merge", "assign-merge", 13*time.Second, 15*time.Second),
		receiptCompanion("receipt-assign-merge", "assign-merge", 15200*time.Millisecond),
		artifactWrite("tool-write-merge", "merge-report.md", 16*time.Second),
		{
			ID: "tool-memory-edit", Kind: protocol.ExecutionRuntimeNodeTool, SubjectID: "tool-memory-edit",
			AgentRoundID: "agent-round-cindy", AgentID: "agent-cindy", Name: "Edit",
			Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now.Add(17 * time.Second),
			UpdatedAt: now.Add(17 * time.Second), ResultSummary: "memory/project_workgraph_smoke.md",
			Artifacts: []protocol.WorkspaceFileArtifactBlock{{
				Type: protocol.ContentBlockTypeWorkspaceFileArtifact,
				Path: "memory/project_workgraph_smoke.md", SourceToolUseID: "tool-memory-edit",
			}},
		},
		{
			ID: "tool-memory-edit-2", Kind: protocol.ExecutionRuntimeNodeTool, SubjectID: "tool-memory-edit-2",
			AgentRoundID: "agent-round-cindy", AgentID: "agent-cindy", Name: "Edit",
			Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now.Add(18 * time.Second),
			UpdatedAt: now.Add(18 * time.Second), ResultSummary: "memory/project_workgraph_smoke.md",
			Artifacts: []protocol.WorkspaceFileArtifactBlock{{
				Type: protocol.ContentBlockTypeWorkspaceFileArtifact,
				Path: "memory/project_workgraph_smoke.md", SourceToolUseID: "tool-memory-edit-2",
			}},
		},
	}
	edges := make([]protocol.ExecutionRuntimeEdgeRun, 0, len(nodes)-1)
	for _, node := range nodes[1:] {
		edges = append(edges, protocol.ExecutionRuntimeEdgeRun{
			ID: "edge-" + node.ID, SourceNodeID: root.ID, TargetNodeID: node.ID,
			Kind: protocol.ExecutionRuntimeEdgeInvoke, CreatedAt: node.StartedAt,
		})
	}
	mergeExecutionRuntimeGraph(view, protocol.ExecutionRuntimeGraph{Nodes: nodes, Edges: edges})

	attemptNodeID := func(attemptID string) string {
		for _, node := range view.Graph.Nodes {
			if node.Kind == protocol.ExecutionGraphNodeAgent && node.AttemptID == attemptID {
				return node.ID
			}
		}
		return ""
	}
	for toolID, attemptID := range map[string]string{
		"tool-assign-left-1":    "attempt-left-1",
		"receipt-assign-left-1": "attempt-left-1",
		"tool-staging":          "attempt-left-1",
		"tool-assign-right":     "attempt-right",
		"receipt-assign-right":  "attempt-right",
		"tool-write-right":      "attempt-right",
		"tool-assign-left-2":    "attempt-left-2",
		"receipt-assign-left-2": "attempt-left-2",
		"tool-write-left":       "attempt-left-2",
		"tool-assign-merge":     "attempt-merge",
		"receipt-assign-merge":  "attempt-merge",
		"tool-write-merge":      "attempt-merge",
		"tool-memory-edit":      "attempt-merge",
		"tool-memory-edit-2":    "attempt-merge",
	} {
		node := graphNodeByID(view.Graph.Nodes, toolID)
		if wantParent := attemptNodeID(attemptID); node.ID == "" || wantParent == "" || node.ParentNodeID != wantParent {
			t.Fatalf("%s parent = %q, want Attempt %s node %q", toolID, node.ParentNodeID, attemptID, wantParent)
		}
	}
	visibleRuntimeNodes := 0
	for _, node := range view.Graph.Nodes {
		if !strings.HasPrefix(node.ID, "tool-") || node.Visibility == protocol.ExecutionGraphNodeDetail {
			continue
		}
		visibleRuntimeNodes++
		if node.ID != "tool-write-right" && node.ID != "tool-write-left" && node.ID != "tool-write-merge" {
			t.Fatalf("supporting transport leaked onto canvas: %+v", node)
		}
	}
	if visibleRuntimeNodes != 3 {
		t.Fatalf("visible runtime nodes = %d, want three exact artifacts", visibleRuntimeNodes)
	}
}

func TestRuntimeGraphRecoversHistoricalSubagentToolsWithoutPersistingContent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 11, 0, 0, 0, time.UTC)
	graph := RecoverRuntimeGraphHistory("owner-1", "session-1",
		protocol.ExecutionRuntimeGraph{
			GraphID:   "graph-1",
			NodeTotal: 1,
			Nodes: []protocol.ExecutionRuntimeNodeRun{{
				ID:             "launch-tool-1",
				GraphID:        "graph-1",
				OwnerUserID:    "owner-1",
				SessionKey:     "session-1",
				ExecutionID:    "execution-1",
				Kind:           protocol.ExecutionRuntimeNodeTool,
				SubjectID:      "spawn-tool-1",
				RootRoundID:    "round-1",
				RuntimeRoundID: "round-1",
				AgentRoundID:   "agent-round-1",
				AgentID:        "parent-1",
				Name:           "Agent",
				Status:         protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt:      now,
				UpdatedAt:      now,
			}},
		},
		nil, []RuntimeGraphSubagentToolHistory{{
			ParentToolUseID: "spawn-tool-1", TaskID: "task-1", AgentID: "child-1",
			ToolUseID: "read-1", Name: "Read", Status: "succeeded",
			StartedAt: now.Add(time.Second).UnixMilli(), FinishedAt: now.Add(2 * time.Second).UnixMilli(),
		}},
	)
	if len(graph.Nodes) != 2 || len(graph.Edges) != 1 || graph.NodeTotal != 2 || graph.EdgeTotal != 1 {
		t.Fatalf("history graph = %+v", graph)
	}
	tool := graph.Nodes[1]
	if tool.Kind != protocol.ExecutionRuntimeNodeTool || tool.SubjectID != "read-1" ||
		tool.ParentSubjectID != "spawn-tool-1" || tool.Name != "Read" ||
		tool.Status != protocol.ExecutionRuntimeNodeSucceeded || tool.ResultSummary != "" ||
		graph.Edges[0].SourceNodeID != "launch-tool-1" || graph.Edges[0].TargetNodeID != tool.ID {
		t.Fatalf("recovered Tool lifecycle = %+v edge=%+v", tool, graph.Edges[0])
	}
}

func TestRuntimeGraphToolActionVisibilityUsesUserObservableSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		toolName string
		kind     protocol.ExecutionRuntimeNodeKind
		visible  bool
	}{
		{name: "web search", toolName: "WebSearch", kind: protocol.ExecutionRuntimeNodeTool, visible: true},
		{name: "wrapped web fetch", toolName: "browser.web-fetch", kind: protocol.ExecutionRuntimeNodeTool, visible: true},
		{name: "ordinary shell detail", toolName: "Bash", kind: protocol.ExecutionRuntimeNodeTool, visible: false},
		{name: "workspace mutation needs artifact or explicit importance", toolName: "Edit", kind: protocol.ExecutionRuntimeNodeTool, visible: false},
		{name: "external mutation", toolName: "mcp__slack__send_message", kind: protocol.ExecutionRuntimeNodeTool, visible: true},
		{name: "browser action", toolName: "mcp__browser__navigate", kind: protocol.ExecutionRuntimeNodeTool, visible: true},
		{name: "local read", toolName: "Read", kind: protocol.ExecutionRuntimeNodeTool, visible: false},
		{name: "local search", toolName: "Grep", kind: protocol.ExecutionRuntimeNodeTool, visible: false},
		{name: "local write variant", toolName: "write_file", kind: protocol.ExecutionRuntimeNodeTool, visible: false},
		{name: "workspace write variant", toolName: "mcp__filesystem__write_file", kind: protocol.ExecutionRuntimeNodeTool, visible: false},
		{name: "external query", toolName: "mcp__github__list_issues", kind: protocol.ExecutionRuntimeNodeTool, visible: true},
		{name: "workspace mcp read", toolName: "mcp__filesystem__read_file", kind: protocol.ExecutionRuntimeNodeTool, visible: false},
		{name: "unknown local query", toolName: "list_issues", kind: protocol.ExecutionRuntimeNodeTool, visible: false},
		{name: "tool discovery", toolName: "ToolSearch", kind: protocol.ExecutionRuntimeNodeTool, visible: false},
		{name: "legacy submission transport", toolName: "submit_work", kind: protocol.ExecutionRuntimeNodeTool, visible: false},
		{name: "external update capability", toolName: "mcp__external__update_record", kind: protocol.ExecutionRuntimeNodeTool, visible: true},
		{name: "non tool", toolName: "WebFetch", kind: protocol.ExecutionRuntimeNodeAgent, visible: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			item := protocol.ExecutionRuntimeNodeRun{Kind: test.kind, Name: test.toolName}
			if got := runtimeGraphToolActionVisible(item); got != test.visible {
				t.Fatalf("runtimeGraphToolActionVisible(%q) = %t, want %t", test.toolName, got, test.visible)
			}
		})
	}

	managedCommand := protocol.ExecutionRuntimeNodeRun{
		Kind:     protocol.ExecutionRuntimeNodeTool,
		Name:     "Bash",
		Status:   protocol.ExecutionRuntimeNodeFailed,
		Metadata: map[string]any{runtimeGraphCommandTransportMetadataKey: true},
	}
	if runtimeGraphToolActionVisible(managedCommand) {
		t.Fatal("managed Goal/Execution CLI transport must not enter the canvas")
	}
	projected := projectRuntimeGraphNode(managedCommand, 0, true)
	if projected.Visibility != protocol.ExecutionGraphNodeDetail {
		t.Fatalf("failed managed transport visibility = %q", projected.Visibility)
	}

	stagingPath := "/private/state/users/owner/runtime/tmp/runtime-command-inputs/0123456789abcdef0123456789abcdef/input.json"
	stagingWrite := protocol.ExecutionRuntimeNodeRun{
		Kind:          protocol.ExecutionRuntimeNodeTool,
		Name:          "Write",
		Status:        protocol.ExecutionRuntimeNodeFailed,
		ResultSummary: "The file " + stagingPath + " has been updated successfully.",
	}
	if !runtimeGraphIsCommandTransport(stagingWrite) {
		t.Fatal("historical command input staging Write was not classified as transport detail")
	}
	if projected := projectRuntimeGraphNode(stagingWrite, 0, true); projected.Visibility != protocol.ExecutionGraphNodeDetail {
		t.Fatalf("command input staging visibility = %q", projected.Visibility)
	}
	if runtimeGraphCommandInputStagingTool("Write", map[string]any{"file_path": "input.json"}) {
		t.Fatal("ordinary workspace input.json was mistaken for host command staging")
	}
	if !runtimeGraphCommandInputStagingTool("write_file", map[string]any{"file_path": stagingPath}) {
		t.Fatal("write_file command input staging variant was not classified as transport detail")
	}
}

func TestRuntimeGraphViewKeepsLegacyReviewerToolsOnTheirSubmissionGate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 19, 12, 30, 0, 0, time.UTC)
	view := &protocol.ExecutionView{
		ID: "execution-review-history",
		WorkItems: []protocol.ExecutionWorkItemView{{
			ID: "work-review-history", Position: 0,
		}},
		Graph: protocol.ExecutionGraphView{Nodes: []protocol.ExecutionGraphNodeView{
			{
				ID: "review:submission-old", Kind: protocol.ExecutionGraphNodeGate,
				Visibility: protocol.ExecutionGraphNodePrimary,
				WorkItemID: "work-review-history", AttemptID: "attempt-old",
				AgentID: "reviewer-1", AgentRoundID: "review-round-old",
				SubjectID: "submission-old", Position: 0,
			},
			{
				ID: "review:submission-current", Kind: protocol.ExecutionGraphNodeGate,
				Visibility: protocol.ExecutionGraphNodePrimary,
				WorkItemID: "work-review-history", AttemptID: "attempt-current",
				AgentID: "reviewer-1", AgentRoundID: "review-round-current",
				SubjectID: "submission-current", Position: 0,
			},
		}},
	}
	toolStarted := now.Add(time.Second)
	mergeExecutionRuntimeGraph(view, protocol.ExecutionRuntimeGraph{Nodes: []protocol.ExecutionRuntimeNodeRun{
		{
			ID: "runtime-review-old", Kind: protocol.ExecutionRuntimeNodeAgent,
			SubjectID: "review-round-old", AgentRoundID: "review-round-old",
			AgentID: "reviewer-1", Status: protocol.ExecutionRuntimeNodeSucceeded,
			StartedAt: now, UpdatedAt: toolStarted,
			Metadata: map[string]any{
				"execution_lane": "review",
				"work_item_id":   "work-review-history",
			},
		},
		{
			ID: "review-tool-old", Kind: protocol.ExecutionRuntimeNodeTool,
			SubjectID: "review-tool-old", ParentSubjectID: "review-round-old",
			AgentRoundID: "review-round-old", AgentID: "reviewer-1", Name: "Read",
			Status:    protocol.ExecutionRuntimeNodeSucceeded,
			StartedAt: toolStarted, UpdatedAt: toolStarted,
		},
	}})

	tool := graphNodeByID(view.Graph.Nodes, "review-tool-old")
	if tool.ID == "" || tool.ParentNodeID != "review:submission-old" ||
		tool.WorkItemID != "work-review-history" {
		t.Fatalf("legacy reviewer tool attached to wrong Submission gate: %+v", tool)
	}
}

func TestRuntimeGraphArtifactsPersistBeforeToolRunByExactIdentity(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 4, 11, 0, 0, 0, time.UTC)
	repository := &runtimeGraphRepositoryFake{
		fakeRepository: &fakeRepository{},
	}
	service := NewService(repository)
	service.now = func() time.Time { return now.Add(time.Second) }
	actor := ActorContext{
		OwnerUserID: "owner-1", SessionKey: "session-1", AgentID: "agent-1",
		RootRoundID: "round-1", RuntimeRoundID: "round-1", AgentRoundID: "agent-round-1",
	}
	err := service.ObserveRuntimeArtifacts(context.Background(), actor, protocol.Message{
		"content": []any{
			map[string]any{
				"type": protocol.ContentBlockTypeWorkspaceFileArtifact,
				"id":   "workspace_file:tool-artifact:reports/result.md",
				"path": "reports/result.md", "display_path": "reports/result.md",
				"workspace_agent_id": "agent-1", "source_tool_use_id": "tool-artifact",
			},
			map[string]any{
				"type": protocol.ContentBlockTypeWorkspaceFileArtifact,
				"path": "../outside.txt", "source_tool_use_id": "tool-artifact",
			},
			map[string]any{
				"type": protocol.ContentBlockTypeWorkspaceFileArtifact,
				"path": "reports/unknown.md", "source_tool_use_id": "unknown-tool",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(repository.nodes) != 0 {
		t.Fatalf("artifact observation must not rewrite runtime nodes: %+v", repository.nodes)
	}
	if len(repository.artifacts) != 2 {
		t.Fatalf("durable artifact refs = %+v", repository.artifacts)
	}
	byToolUseID := make(map[string]protocol.ExecutionRuntimeArtifactRef, len(repository.artifacts))
	for _, ref := range repository.artifacts {
		byToolUseID[ref.ToolUseID] = ref
	}
	ref := byToolUseID["tool-artifact"]
	if ref.ID == "" || ref.GraphID != "round:round-1" ||
		ref.AgentRoundID != "agent-round-1" || ref.Artifact.Path != "reports/result.md" {
		t.Fatalf("exact runtime artifact ref = %+v", ref)
	}
	if unknown := byToolUseID["unknown-tool"]; unknown.ID == "" ||
		unknown.Artifact.Path != "reports/unknown.md" {
		t.Fatalf("artifact arriving before its Tool NodeRun was dropped: %+v", unknown)
	}
	projected := projectRuntimeGraphNode(protocol.ExecutionRuntimeNodeRun{
		ID: "runtime-tool-artifact", Kind: protocol.ExecutionRuntimeNodeTool,
		SubjectID: "tool-artifact", AgentRoundID: "agent-round-1",
		Status:    protocol.ExecutionRuntimeNodeSucceeded,
		StartedAt: now, UpdatedAt: now,
		Artifacts: []protocol.WorkspaceFileArtifactBlock{ref.Artifact},
	}, 0, true)
	if projected.Visibility != protocol.ExecutionGraphNodeNested ||
		len(projected.Runs) != 1 || len(projected.Runs[0].Artifacts) != 1 {
		t.Fatalf("projected runtime artifacts = %+v", projected.Runs)
	}
}

func TestRuntimeGraphReadsProviderNeutralToolResultShapes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content any
		want    string
	}{
		{name: "plain text", content: "Found the page", want: "Found the page"},
		{name: "content blocks", content: []any{
			map[string]any{"type": "text", "text": "Found through MCP"},
		}, want: "Found through MCP"},
		{name: "structured result", content: map[string]any{
			"result": map[string]any{"message": "Found through server tool"},
		}, want: "Found through server tool"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			message, err := sdkprotocol.DecodeMessage(map[string]any{
				"type": "user", "uuid": "result-" + tt.name,
				"message": map[string]any{
					"role": "user",
					"content": []any{map[string]any{
						"type": "tool_result", "tool_use_id": "tool-provider",
						"content": tt.content,
					}},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			evidence := runtimeGraphEvidenceForEvent(message, sdkprotocol.RuntimeLifecycleEvent{
				NodeKind:  sdkprotocol.RuntimeLifecycleNodeTool,
				Phase:     sdkprotocol.RuntimeLifecycleFinished,
				SubjectID: "tool-provider",
			})
			if evidence.resultSummary != tt.want {
				t.Fatalf("provider-neutral result summary = %q, want %q", evidence.resultSummary, tt.want)
			}
		})
	}
}

func TestAuditExecutionAlignmentRecordsOptionalGateWithoutRoutingExecution(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 3, 11, 0, 0, 0, time.UTC)
	// Public WorkGraph reads are managed-only. Give the audited Execution a
	// real Plan/Work Item instead of relying on the removed planless view.
	snapshot := assignedExecutionSnapshot()
	snapshot.Execution.RootRoundID = "round-1"
	snapshot.Execution.Objective = "Ship the verified report"
	snapshot.Execution.CompletionCriteria = []string{"report is verified"}
	repository := &runtimeGraphRepositoryFake{
		fakeRepository: &fakeRepository{snapshot: snapshot},
	}
	service := NewService(repository)
	service.now = func() time.Time { return now }
	actor := coordinatorActor()
	actor.ExecutionID = snapshot.Execution.ID
	actor.RootRoundID = "round-1"
	actor.RuntimeRoundID = "runtime-round-1"
	actor.AgentRoundID = "agent-round-1"

	result, err := service.AuditExecutionAlignment(
		context.Background(),
		actor,
		AuditExecutionAlignmentInput{
			ExecutionID:      snapshot.Execution.ID,
			SnapshotRevision: snapshot.Execution.Version,
			CommandID:        "tool-alignment-1",
			Report: protocol.ObjectiveAlignmentReport{
				Decision: protocol.ObjectiveAlignmentNotAligned,
				CriteriaResults: []protocol.ObjectiveAlignmentCriterionResult{{
					Criterion: "report is verified",
					Status:    protocol.ObjectiveAlignmentCriterionUnsatisfied,
					Gap:       "verification has not run",
				}},
				Summary: "The report still needs verification.",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied ||
		snapshot.Execution.Status != protocol.ExecutionStatusActive ||
		len(repository.nodes) != 2 || len(repository.edges) != 2 {
		t.Fatalf("result=%+v nodes=%+v edges=%+v", result, repository.nodes, repository.edges)
	}
	gate := repository.nodes[1]
	if gate.Kind != protocol.ExecutionRuntimeNodeGate ||
		gate.Name != "objective_alignment" ||
		gate.Metadata["decision"] != "not_aligned" {
		t.Fatalf("gate = %+v", gate)
	}
	if repository.edges[0].Kind != protocol.ExecutionRuntimeEdgeGuard ||
		repository.edges[1].Kind != protocol.ExecutionRuntimeEdgeLoopBack ||
		repository.edges[1].TargetNodeID != repository.nodes[0].ID {
		t.Fatalf("gate edges = %+v", repository.edges)
	}
	rootNodeID := repository.nodes[0].ID

	repository.graph = protocol.ExecutionRuntimeGraph{
		GraphID: repository.nodes[0].GraphID,
		Nodes:   repository.nodes,
		Edges:   repository.edges,
	}
	view, err := service.GetLatestView(
		context.Background(),
		snapshot.Execution.OwnerUserID,
		snapshot.Execution.SessionKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	projectedGate := graphNodeByID(view.Graph.Nodes, gate.ID)
	if projectedGate.Kind != protocol.ExecutionGraphNodeGate ||
		projectedGate.Visibility != protocol.ExecutionGraphNodeNested ||
		projectedGate.LifecycleStatus != "not_aligned" ||
		!hasExecutionGraphEdge(
			view.Graph.Edges,
			protocol.ExecutionGraphEdgeGuard,
			rootNodeID,
			gate.ID,
		) ||
		!hasExecutionGraphEdge(
			view.Graph.Edges,
			protocol.ExecutionGraphEdgeLoopBack,
			gate.ID,
			rootNodeID,
		) {
		t.Fatalf("projected graph = %+v", view.Graph)
	}
}

func TestRuntimeInvalidationSkipsConversationButKeepsEveryManagedRuntimeUpdate(t *testing.T) {
	service := NewService(&runtimeGraphRepositoryFake{fakeRepository: &fakeRepository{}})
	sink := &capturingExecutionInvalidationSink{}
	service.SetExecutionInvalidationSink(sink)
	actor := ActorContext{OwnerUserID: "owner", SessionKey: "session"}
	service.invalidateActor(context.Background(), actor)
	if len(sink.invalidations) != 0 {
		t.Fatal("ordinary conversation invalidated a historical WorkGraph")
	}
	actor.ExecutionID = "execution"
	service.invalidateActor(context.Background(), actor)
	service.invalidateActor(context.Background(), actor)
	if len(sink.invalidations) != 2 || sink.invalidations[0].Version != 0 {
		t.Fatal("runtime changes were lost to a responsibility-version fence")
	}
}
