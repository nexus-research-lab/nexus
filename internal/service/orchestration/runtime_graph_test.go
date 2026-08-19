package orchestration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtimecommand"
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

type runtimeGraphSubagentHistoryProviderFunc func(
	context.Context,
	string,
	string,
) ([]RuntimeGraphSubagentToolHistory, error)

func (provider runtimeGraphSubagentHistoryProviderFunc) ListRuntimeGraphSubagentToolHistory(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
) ([]RuntimeGraphSubagentToolHistory, error) {
	return provider(ctx, ownerUserID, sessionKey)
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
			runtimeGraphCommandDomainMetadataKey:    runtimecommand.DomainExecution,
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
	err = service.ObserveRuntimeCommandReceipts(context.Background(), actor, []runtimecommand.Receipt{
		{
			Domain: runtimecommand.DomainExecution, Operation: "assign_work",
			RequestID: "assign-request-1", Outcome: string(protocol.MutationResultApplied),
			ExecutionID: "execution-1",
			Changed:     []string{"assignment:assignment-1", "attempt:attempt-1"},
		},
		{
			Domain: runtimecommand.DomainExecution, Operation: "submit_work",
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

func TestRuntimeGraphObservesBridgeToolLifecycleWithoutModelStatusCall(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	repository := &runtimeGraphRepositoryFake{fakeRepository: &fakeRepository{}}
	service := NewService(repository)
	service.now = func() time.Time { return now }
	actor := ActorContext{
		OwnerUserID:    "owner-1",
		SessionKey:     "session-1",
		AgentID:        "agent-1",
		RootRoundID:    "round-1",
		RuntimeRoundID: "round-1",
		AgentRoundID:   "agent-round-1",
	}
	message, err := sdkprotocol.DecodeMessage(map[string]any{
		"type": "assistant",
		"uuid": "assistant-1",
		"message": map[string]any{
			"role": "assistant",
			"content": []any{map[string]any{
				"type":  "tool_use",
				"id":    "tool-1",
				"name":  "search",
				"input": map[string]any{"secret": "not persisted"},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.ObserveRuntimeMessage(context.Background(), actor, message); err != nil {
		t.Fatal(err)
	}
	if repository.reconciled != 1 || len(repository.nodes) != 2 || len(repository.edges) != 1 {
		t.Fatalf("runtime graph writes = reconcile:%d nodes:%d edges:%d", repository.reconciled, len(repository.nodes), len(repository.edges))
	}
	tool := repository.nodes[1]
	if tool.Kind != protocol.ExecutionRuntimeNodeTool ||
		tool.SubjectID != "tool-1" || tool.Name != "search" ||
		tool.Status != protocol.ExecutionRuntimeNodeRunning {
		t.Fatalf("unexpected tool node: %+v", tool)
	}
	if _, leaked := tool.Metadata["secret"]; leaked {
		t.Fatalf("tool input leaked into runtime graph metadata: %+v", tool.Metadata)
	}
	if err = service.FinishRuntimeRound(
		context.Background(),
		actor,
		"",
		"round interrupted",
	); err != nil {
		t.Fatal(err)
	}
	if repository.finishedStatus != protocol.ExecutionRuntimeNodeInterrupted {
		t.Fatalf("finished status = %q, want interrupted", repository.finishedStatus)
	}
}

func TestRuntimeGraphMarksOnlyExactManagedCLITransportAsDetail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		toolName  string
		command   string
		transport bool
	}{
		{
			name: "managed Bash execution inspect", toolName: "Bash",
			command: `"${NEXUS_COMMAND_PATH}" --json execution inspect`, transport: true,
		},
		{
			name: "managed PowerShell Goal inspect", toolName: "PowerShell",
			command: `& "${env:NEXUS_COMMAND_PATH}" --json goal inspect`, transport: true,
		},
		{
			name: "shell chaining is not managed transport", toolName: "Bash",
			command: `"${NEXUS_COMMAND_PATH}" --json execution inspect; make deploy`,
		},
		{
			name: "ordinary Bash is not managed transport", toolName: "Bash",
			command: `go test ./internal/service/orchestration`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			message, err := sdkprotocol.DecodeMessage(map[string]any{
				"type": "assistant", "uuid": "assistant-1",
				"message": map[string]any{
					"role": "assistant",
					"content": []any{map[string]any{
						"type": "tool_use", "id": "tool-1", "name": test.toolName,
						"input": map[string]any{"command": test.command},
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
			marked := strings.EqualFold(
				events[0].Metadata[runtimeGraphCommandTransportMetadataKey],
				"true",
			)
			if marked != test.transport {
				t.Fatalf("managed transport = %t, want %t metadata=%+v", marked, test.transport, events[0].Metadata)
			}
		})
	}
}

func TestRuntimeGraphPersistsManagedCLITransportWithoutRawCommand(t *testing.T) {
	t.Parallel()

	repository := &runtimeGraphRepositoryFake{fakeRepository: &fakeRepository{}}
	service := NewService(repository)
	service.now = func() time.Time {
		return time.Date(2026, 8, 18, 17, 0, 0, 0, time.UTC)
	}
	actor := ActorContext{
		OwnerUserID: "owner-1", SessionKey: "session-1", AgentID: "agent-1",
		RootRoundID: "round-1", RuntimeRoundID: "round-1", AgentRoundID: "agent-round-1",
	}
	message, err := sdkprotocol.DecodeMessage(map[string]any{
		"type": "assistant", "uuid": "assistant-1",
		"message": map[string]any{
			"role": "assistant",
			"content": []any{map[string]any{
				"type": "tool_use", "id": "tool-1", "name": "Bash",
				"input": map[string]any{
					"command": `"${NEXUS_COMMAND_PATH}" --json execution inspect`,
				},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.ObserveRuntimeMessage(context.Background(), actor, message); err != nil {
		t.Fatal(err)
	}
	if len(repository.nodes) != 2 {
		t.Fatalf("runtime graph nodes = %+v", repository.nodes)
	}
	tool := repository.nodes[1]
	if !runtimeGraphIsCommandTransport(tool) ||
		runtimeGraphMetadataString(tool, runtimeGraphCommandDomainMetadataKey) != "execution" ||
		runtimeGraphMetadataString(tool, runtimeGraphCommandActionMetadataKey) != "inspect" {
		t.Fatalf("managed command transport metadata = %+v", tool.Metadata)
	}
	if _, leaked := tool.Metadata["command"]; leaked {
		t.Fatalf("managed command text leaked into runtime graph metadata: %+v", tool.Metadata)
	}
	if projected := projectRuntimeGraphNode(tool, 0, true); projected.Visibility != protocol.ExecutionGraphNodeDetail {
		t.Fatalf("managed command transport visibility = %q", projected.Visibility)
	}
}

func TestRuntimeGraphRecordsSanitizedFailureAndObservedControlReturn(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	repository := &runtimeGraphRepositoryFake{fakeRepository: &fakeRepository{}}
	service := NewService(repository)
	service.now = func() time.Time { return now }
	actor := ActorContext{
		OwnerUserID: "owner-1", SessionKey: "session-1", AgentID: "agent-1",
		RootRoundID: "round-1", RuntimeRoundID: "round-1", AgentRoundID: "agent-round-1",
	}
	message, err := sdkprotocol.DecodeMessage(map[string]any{
		"type": "user",
		"uuid": "tool-result-1",
		"message": map[string]any{
			"role": "user",
			"content": []any{map[string]any{
				"type": "tool_result", "tool_use_id": "tool-1",
				"is_error": true, "error_code": "page_unavailable",
				"content": "Authorization: Bearer live-secret could not fetch the page",
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.ObserveRuntimeMessage(context.Background(), actor, message); err != nil {
		t.Fatal(err)
	}
	if len(repository.nodes) != 2 || len(repository.edges) != 2 {
		t.Fatalf("runtime writes nodes=%+v edges=%+v", repository.nodes, repository.edges)
	}
	tool := repository.nodes[1]
	if tool.Status != protocol.ExecutionRuntimeNodeFailed ||
		tool.ErrorCode != "page_unavailable" ||
		tool.ErrorSummary != "Authorization: Bearer <redacted> could not fetch the page" {
		t.Fatalf("sanitized failure = %+v", tool)
	}
	if repository.edges[1].Kind != protocol.ExecutionRuntimeEdgeLoopBack ||
		repository.edges[1].SourceNodeID != tool.ID ||
		repository.edges[1].TargetNodeID != repository.nodes[0].ID {
		t.Fatalf("control return edge = %+v", repository.edges[1])
	}
}

func TestRuntimeGraphTreatsRejectedMutationAsFailedControlReturn(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 4, 12, 15, 0, 0, time.UTC)
	repository := &runtimeGraphRepositoryFake{fakeRepository: &fakeRepository{}}
	service := NewService(repository)
	service.now = func() time.Time { return now }
	actor := ActorContext{
		OwnerUserID: "owner-1", SessionKey: "session-1", AgentID: "agent-1",
		RootRoundID: "round-1", RuntimeRoundID: "round-1", AgentRoundID: "agent-round-1",
	}
	message, err := sdkprotocol.DecodeMessage(map[string]any{
		"type": "user",
		"uuid": "tool-result-rejected",
		"message": map[string]any{
			"role": "user",
			"content": []any{map[string]any{
				"type": "tool_result", "tool_use_id": "tool-plan", "is_error": false,
				"content": `{"message":"Plan Document items must contain at least one complete Work Item","outcome":"rejected","reason_code":"plan_items_empty"}`,
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.ObserveRuntimeMessage(context.Background(), actor, message); err != nil {
		t.Fatal(err)
	}
	if len(repository.nodes) != 2 || len(repository.edges) != 2 {
		t.Fatalf("runtime writes nodes=%+v edges=%+v", repository.nodes, repository.edges)
	}
	tool := repository.nodes[1]
	if tool.Status != protocol.ExecutionRuntimeNodeFailed || !tool.Failed ||
		tool.ErrorCode != "plan_items_empty" ||
		tool.ErrorSummary != "Plan Document items must contain at least one complete Work Item" ||
		tool.ResultSummary != "" || tool.Metadata["mutation_outcome"] != "rejected" {
		t.Fatalf("rejected mutation node = %+v", tool)
	}
	if repository.edges[1].Kind != protocol.ExecutionRuntimeEdgeLoopBack ||
		repository.edges[1].SourceNodeID != tool.ID ||
		repository.edges[1].TargetNodeID != repository.nodes[0].ID {
		t.Fatalf("rejected mutation control return = %+v", repository.edges[1])
	}
}

func TestCompactRuntimeGraphSummaryHidesInternalSentinels(t *testing.T) {
	t.Parallel()

	got, truncated := compactRuntimeGraphSummary(
		"  __nexus_interrupt_without_message__  ",
	)
	if got != "" || truncated {
		t.Fatalf("internal sentinel summary = %q truncated=%t", got, truncated)
	}
	got, truncated = compactRuntimeGraphSummary(
		"request failed __nexus_internal_control__ after 2s",
	)
	if got != "request failed after 2s" || truncated {
		t.Fatalf("mixed sentinel summary = %q truncated=%t", got, truncated)
	}
}

func TestRuntimeGraphRecordsOnlyExactlyCorrelatedRetry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 4, 12, 30, 0, 0, time.UTC)
	previous := protocol.ExecutionRuntimeNodeRun{
		ID: "runtime-tool-previous", Kind: protocol.ExecutionRuntimeNodeTool,
		SubjectID: "tool-previous", AgentRoundID: "agent-round-1",
	}
	repository := &runtimeGraphRepositoryFake{
		fakeRepository: &fakeRepository{},
		graph:          protocol.ExecutionRuntimeGraph{Nodes: []protocol.ExecutionRuntimeNodeRun{previous}},
	}
	service := NewService(repository)
	service.now = func() time.Time { return now }
	actor := ActorContext{
		OwnerUserID: "owner-1", SessionKey: "session-1", AgentID: "agent-1",
		RootRoundID: "round-1", RuntimeRoundID: "round-1", AgentRoundID: "agent-round-1",
	}
	message := sdkprotocol.ReceivedMessage{
		UUID: "assistant-retry-1",
		RuntimeLifecycle: []sdkprotocol.RuntimeLifecycleEvent{{
			EventID: "retry-event-1", NodeKind: sdkprotocol.RuntimeLifecycleNodeTool,
			Phase: sdkprotocol.RuntimeLifecycleStarted, SubjectID: "tool-retry",
			Name: "search", Status: "running",
			Metadata: map[string]string{"retry_of_tool_use_id": "tool-previous"},
		}},
	}
	if err := service.ObserveRuntimeMessage(context.Background(), actor, message); err != nil {
		t.Fatal(err)
	}
	if len(repository.edges) != 2 ||
		repository.edges[1].Kind != protocol.ExecutionRuntimeEdgeRetry ||
		repository.edges[1].SourceNodeID != previous.ID ||
		repository.edges[1].TargetNodeID != repository.nodes[1].ID {
		t.Fatalf("exact retry edge = %+v", repository.edges)
	}
}

func TestRuntimeGraphDoesNotPersistUnstartedProgressFacetAsTool(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 9, 0, 0, 0, time.UTC)
	repository := &runtimeGraphRepositoryFake{fakeRepository: &fakeRepository{}}
	service := NewService(repository)
	service.now = func() time.Time { return now }
	actor := ActorContext{
		OwnerUserID: "owner-1", SessionKey: "session-1", AgentID: "agent-1",
		RootRoundID: "round-1", RuntimeRoundID: "round-1", AgentRoundID: "agent-round-1",
	}
	message := sdkprotocol.ReceivedMessage{
		UUID: "progress-only-1",
		RuntimeLifecycle: []sdkprotocol.RuntimeLifecycleEvent{{
			EventID:   "runtime:tool:progress:agent_msg_1:running",
			NodeKind:  sdkprotocol.RuntimeLifecycleNodeTool,
			Phase:     sdkprotocol.RuntimeLifecycleProgress,
			SubjectID: "agent_msg_1", ParentSubjectID: "spawn-tool-1",
			Name: "Agent", Status: "running",
		}},
	}
	if err := service.ObserveRuntimeMessage(context.Background(), actor, message); err != nil {
		t.Fatal(err)
	}
	if len(repository.nodes) != 1 || repository.nodes[0].Kind != protocol.ExecutionRuntimeNodeAgent ||
		len(repository.edges) != 0 {
		t.Fatalf("progress facet invented a Tool node: nodes=%+v edges=%+v", repository.nodes, repository.edges)
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

func TestRuntimeGraphNestsChildToolsUnderExactSubagentIdentity(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 3, 10, 30, 0, 0, time.UTC)
	actor := ActorContext{
		OwnerUserID:    "owner-1",
		SessionKey:     "session-1",
		AgentID:        "agent-1",
		RootRoundID:    "round-1",
		RuntimeRoundID: "round-1",
		AgentRoundID:   "agent-round-1",
	}
	subagentNodeID := "runtime-subagent-1"
	repository := &runtimeGraphRepositoryFake{
		fakeRepository: &fakeRepository{},
		graph: protocol.ExecutionRuntimeGraph{Nodes: []protocol.ExecutionRuntimeNodeRun{{
			ID:           subagentNodeID,
			Kind:         protocol.ExecutionRuntimeNodeSubagent,
			SubjectID:    "task-1",
			AgentRoundID: "agent-round-1",
			Metadata:     map[string]any{"tool_use_id": "spawn-tool-1"},
		}}},
	}
	service := NewService(repository)
	service.now = func() time.Time { return now }
	message, err := sdkprotocol.DecodeMessage(map[string]any{
		"type":               "assistant",
		"uuid":               "assistant-child-1",
		"parent_tool_use_id": "spawn-tool-1",
		"message": map[string]any{
			"role": "assistant",
			"content": []any{map[string]any{
				"type": "tool_use",
				"id":   "child-tool-1",
				"name": "read_file",
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.ObserveRuntimeMessage(context.Background(), actor, message); err != nil {
		t.Fatal(err)
	}
	if len(repository.edges) != 1 ||
		repository.edges[0].SourceNodeID != subagentNodeID ||
		repository.edges[0].Kind != protocol.ExecutionRuntimeEdgeInvoke {
		t.Fatalf("child tool edge = %+v", repository.edges)
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

func TestRuntimeGraphExactReceiptCompanionDoesNotClearDMSegment(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 19, 15, 0, 0, 0, time.UTC)
	identity := runtimeGraphIdentity{
		ExecutionID: "execution-1", AgentRoundID: "agent-round-1", AgentID: "agent-1",
	}
	segment := runtimeExecutionSegment{
		ExecutionID: "execution-1", WorkItemID: "work-a",
		AssignmentID: "assignment-a", AttemptID: "attempt-a",
		Source: "assign_work_receipt",
	}
	receiptMetadata := map[string]any{
		runtimeGraphCommandTransportMetadataKey: true,
		runtimeGraphCommandDomainMetadataKey:    runtimecommand.DomainExecution,
		runtimeGraphCommandOperationMetadataKey: "assign_work",
		runtimeGraphCommandRequestIDMetadataKey: "assign-a",
		runtimeGraphSegmentBoundaryKey:          runtimeGraphSegmentBoundaryAssign,
	}
	applyRuntimeExecutionSegment(receiptMetadata, segment)
	unresolvedCompanionMetadata := map[string]any{
		runtimeGraphCommandTransportMetadataKey: true,
		runtimeGraphCommandDomainMetadataKey:    runtimecommand.DomainExecution,
		runtimeGraphCommandOperationMetadataKey: "assign_work",
		runtimeGraphCommandRequestIDMetadataKey: "assign-a",
		runtimeGraphSegmentBoundaryKey:          runtimeGraphSegmentBoundaryUnresolved,
	}
	nodes := []protocol.ExecutionRuntimeNodeRun{
		{
			ID: "receipt-assign-a", Kind: protocol.ExecutionRuntimeNodeTool,
			AgentRoundID: "agent-round-1", AgentID: "agent-1", Name: "assign_work",
			Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now, UpdatedAt: now,
			Metadata: receiptMetadata,
		},
		{
			ID: "tool-assign-a", Kind: protocol.ExecutionRuntimeNodeTool,
			AgentRoundID: "agent-round-1", AgentID: "agent-1", Name: "Bash",
			Status:    protocol.ExecutionRuntimeNodeSucceeded,
			StartedAt: now.Add(time.Second), UpdatedAt: now.Add(2 * time.Second),
			Metadata: unresolvedCompanionMetadata,
		},
	}
	if got := latestRuntimeExecutionSegment(nodes, identity); got != segment {
		t.Fatalf("exact unresolved companion cleared trusted receipt segment: got %+v, want %+v", got, segment)
	}
}

func TestRuntimeGraphKeepsRoomWorkBindingExplicit(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 16, 45, 0, 0, time.UTC)
	actor := ActorContext{
		OwnerUserID: "owner-room", SessionKey: "session-room",
		ExecutionID: "execution-room", AgentID: "agent-worker",
		ScopeKind:   protocol.ExecutionScopeRoom,
		RootRoundID: "round-room", RuntimeRoundID: "round-room",
		AgentRoundID: "agent-round-room",
		WorkBinding: &protocol.ExecutionWorkBinding{
			ExecutionID: "execution-room", WorkItemID: "work-room",
			AssignmentID: "assignment-room", AttemptID: "attempt-room",
		},
	}
	repository := &runtimeGraphRepositoryFake{fakeRepository: &fakeRepository{}}
	service := NewService(repository)
	service.now = func() time.Time { return now }
	message, err := sdkprotocol.DecodeMessage(map[string]any{
		"type": "user", "uuid": "result-room-assign",
		"message": map[string]any{
			"role": "user",
			"content": []any{map[string]any{
				"type": "tool_result", "tool_use_id": "tool-room-assign",
				"content": `{"outcome":"applied","execution_id":"execution-room","changed":["assignment:assignment-other","attempt:attempt-other"]}`,
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.ObserveRuntimeMessage(context.Background(), actor, message); err != nil {
		t.Fatal(err)
	}
	node := repository.nodes[len(repository.nodes)-1]
	segment := runtimeExecutionSegmentFromNode(node)
	if !segment.valid() || segment.Source != "work_binding" ||
		segment.WorkItemID != "work-room" ||
		runtimeGraphMetadataString(node, runtimeGraphSegmentBoundaryKey) != "" {
		t.Fatalf("Room WorkBinding was replaced by DM segment logic: %+v node=%+v", segment, node)
	}
	if repository.boundRoundID != "" || repository.boundExecution != "" {
		t.Fatalf("Room round entered DM execution binding: %q/%q", repository.boundRoundID, repository.boundExecution)
	}
}

func TestRuntimeGraphViewSegmentsOneDMRoundAcrossWorkItems(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 17, 0, 0, 0, time.UTC)
	view := &protocol.ExecutionView{
		ID: "execution-1", ScopeKind: protocol.ExecutionScopeDM,
		WorkItems: []protocol.ExecutionWorkItemView{
			{
				ID: "work-b", Position: 1, OwnerAgentID: "agent-1",
				Attempts: []protocol.ExecutionAttemptView{{
					ID: "attempt-b", AssignmentID: "assignment-b",
					ExecutorKind:    protocol.AttemptExecutorAgent,
					ExecutorAgentID: "agent-1", AgentRoundID: "agent-round-1",
					Status:    protocol.WorkAttemptStatusSucceeded,
					CreatedAt: now.Add(6500 * time.Millisecond),
				}},
			},
			{
				ID: "work-c", Position: 2, OwnerAgentID: "agent-1",
				Attempts: []protocol.ExecutionAttemptView{{
					ID: "attempt-c", AssignmentID: "assignment-c",
					ExecutorKind:    protocol.AttemptExecutorAgent,
					ExecutorAgentID: "agent-1", AgentRoundID: "agent-round-1",
					Status:    protocol.WorkAttemptStatusSucceeded,
					CreatedAt: now.Add(10500 * time.Millisecond),
				}},
			},
			{
				ID: "work-a", Position: 0, OwnerAgentID: "agent-1",
				Attempts: []protocol.ExecutionAttemptView{{
					ID: "attempt-a", AssignmentID: "assignment-a",
					ExecutorKind:    protocol.AttemptExecutorAgent,
					ExecutorAgentID: "agent-1", AgentRoundID: "agent-round-1",
					Status:    protocol.WorkAttemptStatusSucceeded,
					CreatedAt: now.Add(2500 * time.Millisecond),
				}},
			},
		},
	}
	view.Graph = projectExecutionGraphView(view.WorkItems)
	finishedAt := func(offset time.Duration) *time.Time {
		value := now.Add(offset)
		return &value
	}
	nodes := []protocol.ExecutionRuntimeNodeRun{
		{
			ID: "runtime-root", Kind: protocol.ExecutionRuntimeNodeAgent,
			SubjectID: "agent-round-1", AgentRoundID: "agent-round-1", AgentID: "agent-1",
			Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now, UpdatedAt: now,
			Metadata: map[string]any{"execution_lane": "coordination"},
		},
		{
			ID: "tool-plan", Kind: protocol.ExecutionRuntimeNodeTool, SubjectID: "tool-plan",
			AgentRoundID: "agent-round-1", AgentID: "agent-1", Name: "Bash",
			Status:    protocol.ExecutionRuntimeNodeSucceeded,
			StartedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
		},
		{
			ID: "tool-assign-a", Kind: protocol.ExecutionRuntimeNodeTool, SubjectID: "tool-assign-a",
			AgentRoundID: "agent-round-1", AgentID: "agent-1", Name: "Bash",
			Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now.Add(2 * time.Second),
			UpdatedAt: now.Add(3 * time.Second), FinishedAt: finishedAt(3 * time.Second),
			Metadata: map[string]any{
				runtimeGraphCommandTransportMetadataKey: true,
				runtimeGraphCommandOperationMetadataKey: "assign_work",
			},
		},
		{
			ID: "tool-write-a", Kind: protocol.ExecutionRuntimeNodeTool, SubjectID: "tool-write-a",
			AgentRoundID: "agent-round-1", AgentID: "agent-1", Name: "Write",
			Status:    protocol.ExecutionRuntimeNodeSucceeded,
			StartedAt: now.Add(4 * time.Second), UpdatedAt: now.Add(4 * time.Second),
		},
		{
			ID: "tool-assign-b", Kind: protocol.ExecutionRuntimeNodeTool, SubjectID: "tool-assign-b",
			AgentRoundID: "agent-round-1", AgentID: "agent-1", Name: "Bash",
			Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now.Add(6 * time.Second),
			UpdatedAt: now.Add(7 * time.Second), FinishedAt: finishedAt(7 * time.Second),
			Metadata: map[string]any{
				runtimeGraphCommandTransportMetadataKey: true,
				runtimeGraphCommandOperationMetadataKey: "assign_work",
			},
		},
		{
			ID: "tool-bash-b", Kind: protocol.ExecutionRuntimeNodeTool, SubjectID: "tool-bash-b",
			AgentRoundID: "agent-round-1", AgentID: "agent-1", Name: "Bash",
			Status:    protocol.ExecutionRuntimeNodeSucceeded,
			StartedAt: now.Add(8 * time.Second), UpdatedAt: now.Add(8 * time.Second),
		},
		{
			ID: "tool-assign-c", Kind: protocol.ExecutionRuntimeNodeTool, SubjectID: "tool-assign-c",
			AgentRoundID: "agent-round-1", AgentID: "agent-1", Name: "Bash",
			Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now.Add(10 * time.Second),
			UpdatedAt: now.Add(11 * time.Second), FinishedAt: finishedAt(11 * time.Second),
			Metadata: map[string]any{
				runtimeGraphCommandTransportMetadataKey: true,
				runtimeGraphCommandOperationMetadataKey: "assign_work",
			},
		},
		{
			ID: "tool-edit-c", Kind: protocol.ExecutionRuntimeNodeTool, SubjectID: "tool-edit-c",
			AgentRoundID: "agent-round-1", AgentID: "agent-1", Name: "Edit",
			Status:    protocol.ExecutionRuntimeNodeSucceeded,
			StartedAt: now.Add(12 * time.Second), UpdatedAt: now.Add(12 * time.Second),
		},
	}
	edges := make([]protocol.ExecutionRuntimeEdgeRun, 0, len(nodes)-1)
	for index := 1; index < len(nodes); index++ {
		edges = append(edges, protocol.ExecutionRuntimeEdgeRun{
			ID: fmt.Sprintf("edge-%d", index), SourceNodeID: "runtime-root",
			TargetNodeID: nodes[index].ID, Kind: protocol.ExecutionRuntimeEdgeInvoke,
			CreatedAt: nodes[index].StartedAt,
		})
	}
	mergeExecutionRuntimeGraph(view, protocol.ExecutionRuntimeGraph{Nodes: nodes, Edges: edges})

	for nodeID, wantParent := range map[string]string{
		"tool-plan": "work-a", "tool-assign-a": "work-a", "tool-write-a": "work-a",
		"tool-assign-b": "work-b", "tool-bash-b": "work-b",
		"tool-assign-c": "work-c", "tool-edit-c": "work-c",
	} {
		node := graphNodeByID(view.Graph.Nodes, nodeID)
		if node.ID == "" || node.ParentNodeID != wantParent || node.WorkItemID != wantParent {
			t.Fatalf("%s ownership = parent:%q work:%q, want %q; graph=%+v", nodeID, node.ParentNodeID, node.WorkItemID, wantParent, view.Graph)
		}
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
				runtimeGraphCommandDomainMetadataKey:    runtimecommand.DomainExecution,
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
				runtimeGraphCommandDomainMetadataKey:    runtimecommand.DomainExecution,
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

func TestRuntimeGraphViewDoesNotInferDMSegmentsForRoomCoordinator(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 18, 0, 0, 0, time.UTC)
	view := &protocol.ExecutionView{
		ID: "execution-room", ScopeKind: protocol.ExecutionScopeRoom,
		CoordinatorAgentID: "agent-lead",
		WorkItems: []protocol.ExecutionWorkItemView{{
			ID: "work-a", Position: 0, OwnerAgentID: "agent-lead",
			Attempts: []protocol.ExecutionAttemptView{{
				ID: "attempt-a", AssignmentID: "assignment-a",
				ExecutorKind:    protocol.AttemptExecutorAgent,
				ExecutorAgentID: "agent-lead", AgentRoundID: "agent-round-1",
				CreatedAt: now.Add(1500 * time.Millisecond),
			}},
		}},
	}
	view.Graph = projectExecutionGraphView(view.WorkItems)
	projectExecutionCoordinatorNode(view)
	finished := now.Add(2 * time.Second)
	mergeExecutionRuntimeGraph(view, protocol.ExecutionRuntimeGraph{
		Nodes: []protocol.ExecutionRuntimeNodeRun{
			{
				ID: "runtime-root", Kind: protocol.ExecutionRuntimeNodeAgent,
				SubjectID: "agent-round-1", AgentRoundID: "agent-round-1", AgentID: "agent-lead",
				Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now, UpdatedAt: now,
				Metadata: map[string]any{"execution_lane": "coordination"},
			},
			{
				ID: "tool-assign", Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "tool-assign", AgentRoundID: "agent-round-1", AgentID: "agent-lead",
				Name: "assign_work", Status: protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt: now.Add(time.Second), UpdatedAt: finished, FinishedAt: &finished,
			},
			{
				ID: "tool-write", Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "tool-write", AgentRoundID: "agent-round-1", AgentID: "agent-lead",
				Name: "Write", Status: protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt: now.Add(3 * time.Second), UpdatedAt: now.Add(3 * time.Second),
			},
		},
		Edges: []protocol.ExecutionRuntimeEdgeRun{
			{ID: "edge-assign", SourceNodeID: "runtime-root", TargetNodeID: "tool-assign", Kind: protocol.ExecutionRuntimeEdgeInvoke, CreatedAt: now.Add(time.Second)},
			{ID: "edge-write", SourceNodeID: "runtime-root", TargetNodeID: "tool-write", Kind: protocol.ExecutionRuntimeEdgeInvoke, CreatedAt: now.Add(3 * time.Second)},
		},
	})
	write := graphNodeByID(view.Graph.Nodes, "tool-write")
	if write.ParentNodeID != "coordinator:execution-room" || write.WorkItemID != "" {
		t.Fatalf("Room coordination tool was rebound by DM inference: %+v", write)
	}
}

func TestRuntimeGraphViewBindsLaunchToolsAndChildrenToSiblingSubagents(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 8, 10, 0, 0, time.UTC)
	view := &protocol.ExecutionView{
		WorkItems: []protocol.ExecutionWorkItemView{{
			ID:       "work-a",
			Position: 0,
			Status:   protocol.ExecutionWorkItemViewRunning,
			Attempts: []protocol.ExecutionAttemptView{
				{
					ID:           "attempt-root",
					ExecutorKind: protocol.AttemptExecutorAgent,
					AgentRoundID: "agent-round-1",
					Status:       protocol.WorkAttemptStatusRunning,
					CreatedAt:    now,
				},
				{
					ID:              "attempt-child-first",
					ParentAttemptID: "attempt-root",
					ExecutorKind:    protocol.AttemptExecutorSubagent,
					ToolUseID:       "spawn-first",
					Status:          protocol.WorkAttemptStatusInterrupted,
					CreatedAt:       now.Add(time.Second),
				},
				{
					ID:              "attempt-child-second",
					ParentAttemptID: "attempt-root",
					ExecutorKind:    protocol.AttemptExecutorSubagent,
					ToolUseID:       "spawn-second",
					Status:          protocol.WorkAttemptStatusRunning,
					CreatedAt:       now.Add(2 * time.Second),
				},
			},
		}},
	}
	view.Graph = projectExecutionGraphView(view.WorkItems)

	rootRunID := "runtime-agent-root"
	firstLaunchRunID := "runtime-launch-first"
	secondLaunchRunID := "runtime-launch-second"
	firstToolRunID := "runtime-tool-first"
	secondToolRunID := "runtime-tool-second"
	mergeExecutionRuntimeGraph(view, protocol.ExecutionRuntimeGraph{
		Nodes: []protocol.ExecutionRuntimeNodeRun{
			{
				ID: rootRunID, Kind: protocol.ExecutionRuntimeNodeAgent,
				SubjectID: "agent-round-1", AgentRoundID: "agent-round-1",
				AgentID: "parent-agent", Status: protocol.ExecutionRuntimeNodeRunning,
				StartedAt: now, UpdatedAt: now,
			},
			{
				ID: firstLaunchRunID, Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "spawn-first", AgentRoundID: "agent-round-1",
				AgentID: "parent-agent", Name: "Agent",
				Status:    protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
			},
			{
				ID: firstToolRunID, Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "read-first", ParentSubjectID: "spawn-first",
				AgentRoundID: "agent-round-1", AgentID: "parent-agent", Name: "Read",
				Status:    protocol.ExecutionRuntimeNodeRunning,
				StartedAt: now.Add(2 * time.Second), UpdatedAt: now.Add(2 * time.Second),
			},
			{
				ID: secondLaunchRunID, Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "spawn-second", AgentRoundID: "agent-round-1",
				AgentID: "parent-agent", Name: "Agent",
				Status:    protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt: now.Add(3 * time.Second), UpdatedAt: now.Add(3 * time.Second),
			},
			{
				ID: secondToolRunID, Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "read-second", ParentSubjectID: "spawn-second",
				AgentRoundID: "agent-round-1", AgentID: "parent-agent", Name: "Read",
				Status:    protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt: now.Add(4 * time.Second), UpdatedAt: now.Add(4 * time.Second),
			},
		},
		Edges: []protocol.ExecutionRuntimeEdgeRun{
			{
				ID: "edge-root-first", SourceNodeID: rootRunID,
				TargetNodeID: firstLaunchRunID, Kind: protocol.ExecutionRuntimeEdgeInvoke,
				CreatedAt: now.Add(time.Second),
			},
			{
				ID: "edge-stale-root-first-tool", SourceNodeID: rootRunID,
				TargetNodeID: firstToolRunID, Kind: protocol.ExecutionRuntimeEdgeInvoke,
				CreatedAt: now.Add(2 * time.Second),
			},
			{
				ID: "edge-root-second", SourceNodeID: rootRunID,
				TargetNodeID: secondLaunchRunID, Kind: protocol.ExecutionRuntimeEdgeInvoke,
				CreatedAt: now.Add(3 * time.Second),
			},
			{
				ID: "edge-second-tool", SourceNodeID: secondLaunchRunID,
				TargetNodeID: secondToolRunID, Kind: protocol.ExecutionRuntimeEdgeInvoke,
				CreatedAt: now.Add(4 * time.Second),
			},
		},
	})

	if len(view.Graph.Nodes) != 5 || len(view.Graph.Edges) != 4 {
		t.Fatalf("runtime launch Tools were not merged into sibling Subagents: %+v", view.Graph)
	}
	if graphNodeByID(view.Graph.Nodes, firstLaunchRunID).ID != "" ||
		graphNodeByID(view.Graph.Nodes, secondLaunchRunID).ID != "" {
		t.Fatalf("launch Tool leaked as a duplicate Subagent node: %+v", view.Graph.Nodes)
	}
	firstSubagent := graphNodeByID(view.Graph.Nodes, "attempt-child-first")
	secondSubagent := graphNodeByID(view.Graph.Nodes, "attempt-child-second")
	firstTool := graphNodeByID(view.Graph.Nodes, firstToolRunID)
	secondTool := graphNodeByID(view.Graph.Nodes, secondToolRunID)
	if firstSubagent.ParentNodeID != "work-a" || secondSubagent.ParentNodeID != "work-a" ||
		firstSubagent.AgentID != "" || secondSubagent.AgentID != "" ||
		firstSubagent.RunStatus != protocol.WorkAttemptStatusInterrupted ||
		firstSubagent.LifecycleStatus != "" ||
		firstTool.ParentNodeID != firstSubagent.ID ||
		secondTool.ParentNodeID != secondSubagent.ID ||
		firstTool.Visibility != protocol.ExecutionGraphNodeNested ||
		secondTool.Visibility != protocol.ExecutionGraphNodeDetail {
		t.Fatalf(
			"runtime child ownership is incorrect: first=%+v first_tool=%+v second=%+v second_tool=%+v",
			firstSubagent,
			firstTool,
			secondSubagent,
			secondTool,
		)
	}
	if !hasExecutionGraphEdge(
		view.Graph.Edges,
		protocol.ExecutionGraphEdgeInvoke,
		firstSubagent.ID,
		firstTool.ID,
	) || !hasExecutionGraphEdge(
		view.Graph.Edges,
		protocol.ExecutionGraphEdgeInvoke,
		secondSubagent.ID,
		secondTool.ID,
	) || hasExecutionGraphEdge(
		view.Graph.Edges,
		protocol.ExecutionGraphEdgeInvoke,
		"work-a",
		firstTool.ID,
	) {
		t.Fatalf("runtime Tool edges escaped their exact Subagent: %+v", view.Graph.Edges)
	}
}

func TestRuntimeGraphRecoversHistoricalSubagentToolsWithoutPersistingContent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 11, 0, 0, 0, time.UTC)
	service := NewService(&runtimeGraphRepositoryFake{fakeRepository: &fakeRepository{}})
	service.SetRuntimeGraphSubagentToolHistoryProvider(runtimeGraphSubagentHistoryProviderFunc(
		func(_ context.Context, ownerUserID string, sessionKey string) ([]RuntimeGraphSubagentToolHistory, error) {
			if ownerUserID != "owner-1" || sessionKey != "session-1" {
				t.Fatalf("history scope = %q/%q", ownerUserID, sessionKey)
			}
			return []RuntimeGraphSubagentToolHistory{{
				ParentToolUseID: "spawn-tool-1",
				TaskID:          "task-1",
				AgentID:         "child-1",
				ToolUseID:       "read-1",
				Name:            "Read",
				Status:          "succeeded",
				StartedAt:       now.Add(time.Second).UnixMilli(),
				FinishedAt:      now.Add(2 * time.Second).UnixMilli(),
			}}, nil
		},
	))
	graph := service.mergeRuntimeGraphSubagentToolHistory(
		context.Background(),
		"owner-1",
		"session-1",
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

func TestRuntimeGraphReviewEdgesUseSuccessfulSubmissionAsControlAnchor(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 11, 30, 0, 0, time.UTC)
	view := &protocol.ExecutionView{Graph: protocol.ExecutionGraphView{
		Nodes: []protocol.ExecutionGraphNodeView{
			{ID: "work-1", Kind: protocol.ExecutionGraphNodeAgent},
			{
				ID: "submit-1", Kind: protocol.ExecutionGraphNodeTool,
				Visibility:   protocol.ExecutionGraphNodeNested,
				ParentNodeID: "work-1", Name: "submit_work",
				LifecycleStatus: "succeeded", StartedAt: &now,
			},
			{ID: "gate-1", Kind: protocol.ExecutionGraphNodeGate},
		},
		Edges: []protocol.ExecutionGraphEdgeView{
			{ID: "review-1", Kind: protocol.ExecutionGraphEdgeReview, SourceNodeID: "work-1", TargetNodeID: "gate-1"},
			{ID: "return-1", Kind: protocol.ExecutionGraphEdgeLoopBack, SourceNodeID: "gate-1", TargetNodeID: "work-1"},
		},
	}}
	reanchorExecutionReviewEdgesToSubmission(view, map[string]int{
		"work-1": 0, "submit-1": 1, "gate-1": 2,
	})
	if view.Graph.Edges[0].SourceNodeID != "submit-1" ||
		view.Graph.Edges[1].TargetNodeID != "submit-1" {
		t.Fatalf("review control anchor = %+v", view.Graph.Edges)
	}

	view.Graph.Nodes[1].Visibility = protocol.ExecutionGraphNodeDetail
	view.Graph.Edges[0].SourceNodeID = "work-1"
	view.Graph.Edges[1].TargetNodeID = "work-1"
	reanchorExecutionReviewEdgesToSubmission(view, map[string]int{
		"work-1": 0, "submit-1": 1, "gate-1": 2,
	})
	if view.Graph.Edges[0].SourceNodeID != "work-1" ||
		view.Graph.Edges[1].TargetNodeID != "work-1" {
		t.Fatalf("detail command transport captured visible review edges: %+v", view.Graph.Edges)
	}
}

func TestRuntimeGraphViewFiltersHistoricalProgressFacet(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 9, 10, 0, 0, time.UTC)
	view := &protocol.ExecutionView{}
	mergeExecutionRuntimeGraph(view, protocol.ExecutionRuntimeGraph{
		NodeTotal: 2,
		EdgeTotal: 1,
		Nodes: []protocol.ExecutionRuntimeNodeRun{
			{
				ID: "runtime-agent", Kind: protocol.ExecutionRuntimeNodeAgent,
				SubjectID: "agent-round-1", AgentRoundID: "agent-round-1",
				Status:    protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt: now, UpdatedAt: now,
			},
			{
				ID: "runtime-progress", Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "agent_msg_1", ParentSubjectID: "spawn-tool-1",
				AgentRoundID: "agent-round-1", Name: "Agent",
				Status:    protocol.ExecutionRuntimeNodeInterrupted,
				StartedAt: now.Add(time.Second), UpdatedAt: now.Add(2 * time.Second),
				Metadata: map[string]any{
					"bridge_event_id": "runtime:tool:progress:agent_msg_1:running",
				},
			},
		},
		Edges: []protocol.ExecutionRuntimeEdgeRun{{
			ID: "progress-edge", SourceNodeID: "runtime-agent",
			TargetNodeID: "runtime-progress", Kind: protocol.ExecutionRuntimeEdgeInvoke,
			CreatedAt: now.Add(time.Second),
		}},
	})

	if len(view.Graph.Nodes) != 1 || view.Graph.Nodes[0].ID != "runtime-agent" ||
		len(view.Graph.Edges) != 0 || view.Graph.RuntimeNodeTotal != 1 {
		t.Fatalf("historical progress facet leaked into graph: %+v", view.Graph)
	}
}

func TestRuntimeGraphViewAppliesPartialAfterCanvasVisibility(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	view := &protocol.ExecutionView{
		WorkItems: []protocol.ExecutionWorkItemView{{ID: "work-1"}},
		Graph: protocol.ExecutionGraphView{Nodes: []protocol.ExecutionGraphNodeView{{
			ID: "work-1", Kind: protocol.ExecutionGraphNodeAgent,
			Visibility: protocol.ExecutionGraphNodePrimary, WorkItemID: "work-1",
		}}},
	}
	runtimeGraph := protocol.ExecutionRuntimeGraph{
		NodeTotal:      protocol.ExecutionRuntimeGraphNodeProjectionLimit + 46,
		NodesTruncated: true,
		Nodes: []protocol.ExecutionRuntimeNodeRun{{
			ID: "runtime-agent", Kind: protocol.ExecutionRuntimeNodeAgent,
			SubjectID: "agent-round-1", AgentRoundID: "agent-round-1",
			Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now, UpdatedAt: now,
			Metadata: map[string]any{"execution_lane": "work", "work_item_id": "work-1"},
		}},
	}
	for index := 0; index < 300; index++ {
		observedAt := now.Add(time.Duration(index+1) * time.Second)
		nodeID := fmt.Sprintf("detail-read-%03d", index)
		runtimeGraph.Nodes = append(runtimeGraph.Nodes, protocol.ExecutionRuntimeNodeRun{
			ID: nodeID, Kind: protocol.ExecutionRuntimeNodeTool,
			SubjectID: nodeID, ParentSubjectID: "agent-round-1", AgentRoundID: "agent-round-1",
			Name: "Read", Status: protocol.ExecutionRuntimeNodeSucceeded,
			StartedAt: observedAt, UpdatedAt: observedAt,
		})
		runtimeGraph.Edges = append(runtimeGraph.Edges, protocol.ExecutionRuntimeEdgeRun{
			ID: "edge-" + nodeID, SourceNodeID: "runtime-agent", TargetNodeID: nodeID,
			Kind: protocol.ExecutionRuntimeEdgeInvoke, CreatedAt: observedAt,
		})
	}
	failureAt := now.Add(-time.Minute)
	runtimeGraph.Nodes = append(runtimeGraph.Nodes, protocol.ExecutionRuntimeNodeRun{
		ID: "visible-failure", Kind: protocol.ExecutionRuntimeNodeTool,
		SubjectID: "visible-failure", ParentSubjectID: "agent-round-1", AgentRoundID: "agent-round-1",
		Name: "internal-check", Status: protocol.ExecutionRuntimeNodeFailed,
		StartedAt: failureAt, UpdatedAt: failureAt,
	})
	runtimeGraph.Edges = append(runtimeGraph.Edges, protocol.ExecutionRuntimeEdgeRun{
		ID: "edge-visible-failure", SourceNodeID: "runtime-agent", TargetNodeID: "visible-failure",
		Kind: protocol.ExecutionRuntimeEdgeInvoke, CreatedAt: failureAt,
	})

	mergeExecutionRuntimeGraph(view, runtimeGraph)

	if view.Graph.RuntimeNodeTotal != 1 || view.Graph.RuntimeNodesTruncated ||
		view.Graph.RuntimeEdgeTotal != 1 || view.Graph.RuntimeEdgesTruncated {
		t.Fatalf(
			"hidden detail facts marked canvas partial: nodes=%d node_partial=%v edges=%d edge_partial=%v",
			view.Graph.RuntimeNodeTotal,
			view.Graph.RuntimeNodesTruncated,
			view.Graph.RuntimeEdgeTotal,
			view.Graph.RuntimeEdgesTruncated,
		)
	}
	visibleFailure := false
	for _, node := range view.Graph.Nodes {
		if node.ID == "visible-failure" {
			visibleFailure = node.Visibility == protocol.ExecutionGraphNodeNested
		}
	}
	if !visibleFailure {
		t.Fatalf("early display-worthy failure was not preserved: %+v", view.Graph.Nodes)
	}
}

func TestRuntimeGraphViewMarksPartialOnlyWhenCanvasNodesAreOmitted(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	view := &protocol.ExecutionView{
		WorkItems: []protocol.ExecutionWorkItemView{{ID: "work-1"}},
		Graph: protocol.ExecutionGraphView{Nodes: []protocol.ExecutionGraphNodeView{{
			ID: "work-1", Kind: protocol.ExecutionGraphNodeAgent,
			Visibility: protocol.ExecutionGraphNodePrimary, WorkItemID: "work-1",
		}}},
	}
	runtimeGraph := protocol.ExecutionRuntimeGraph{Nodes: []protocol.ExecutionRuntimeNodeRun{{
		ID: "runtime-agent", Kind: protocol.ExecutionRuntimeNodeAgent,
		SubjectID: "agent-round-1", AgentRoundID: "agent-round-1",
		Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now, UpdatedAt: now,
		Metadata: map[string]any{"execution_lane": "work", "work_item_id": "work-1"},
	}}}
	for index := 0; index <= protocol.ExecutionRuntimeGraphNodeProjectionLimit; index++ {
		observedAt := now.Add(time.Duration(index+1) * time.Second)
		nodeID := fmt.Sprintf("visible-failure-%03d", index)
		runtimeGraph.Nodes = append(runtimeGraph.Nodes, protocol.ExecutionRuntimeNodeRun{
			ID: nodeID, Kind: protocol.ExecutionRuntimeNodeTool,
			SubjectID: nodeID, ParentSubjectID: "agent-round-1", AgentRoundID: "agent-round-1",
			Name: "Read", Status: protocol.ExecutionRuntimeNodeFailed,
			StartedAt: observedAt, UpdatedAt: observedAt,
		})
		runtimeGraph.Edges = append(runtimeGraph.Edges, protocol.ExecutionRuntimeEdgeRun{
			ID: "edge-" + nodeID, SourceNodeID: "runtime-agent", TargetNodeID: nodeID,
			Kind: protocol.ExecutionRuntimeEdgeInvoke, CreatedAt: observedAt,
		})
	}

	mergeExecutionRuntimeGraph(view, runtimeGraph)

	if view.Graph.RuntimeNodeTotal != protocol.ExecutionRuntimeGraphNodeProjectionLimit+1 ||
		!view.Graph.RuntimeNodesTruncated {
		t.Fatalf("display-worthy overflow was not marked partial: %+v", view.Graph)
	}
	canvasRuntimeNodes := 0
	for _, node := range view.Graph.Nodes {
		if node.ID != "work-1" && node.Visibility != protocol.ExecutionGraphNodeDetail {
			canvasRuntimeNodes++
		}
	}
	if canvasRuntimeNodes != protocol.ExecutionRuntimeGraphNodeProjectionLimit {
		t.Fatalf("canvas runtime nodes = %d, want %d", canvasRuntimeNodes, protocol.ExecutionRuntimeGraphNodeProjectionLimit)
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

func TestRuntimeGraphAssignmentBoundaryRequiresManagedTransportIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		node protocol.ExecutionRuntimeNodeRun
		want string
	}{
		{
			name: "typed CLI takeover",
			node: protocol.ExecutionRuntimeNodeRun{
				Name: "Bash",
				Metadata: map[string]any{
					runtimeGraphCommandTransportMetadataKey: true,
					runtimeGraphCommandOperationMetadataKey: "take_over_work",
				},
			},
			want: "take_over_work",
		},
		{
			name: "legacy Nexus MCP takeover",
			node: protocol.ExecutionRuntimeNodeRun{Name: "mcp__nexus_execution__take_over_work"},
			want: "take_over_work",
		},
		{
			name: "external same-leaf capability",
			node: protocol.ExecutionRuntimeNodeRun{Name: "mcp__external__take_over_work"},
		},
		{
			name: "unverified Bash metadata",
			node: protocol.ExecutionRuntimeNodeRun{
				Name: "Bash",
				Metadata: map[string]any{
					runtimeGraphCommandOperationMetadataKey: "take_over_work",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := runtimeGraphAssignmentBoundaryOperationForNode(test.node); got != test.want {
				t.Fatalf("boundary operation = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRuntimeGraphMarksCommandInputStagingToolWithoutPersistingRawPath(t *testing.T) {
	t.Parallel()

	path := "/private/state/users/owner/runtime/tmp/runtime-command-inputs/0123456789abcdef0123456789abcdef/input.json"
	message, err := sdkprotocol.DecodeMessage(map[string]any{
		"type": "assistant", "uuid": "assistant-input-write",
		"message": map[string]any{
			"role": "assistant",
			"content": []any{map[string]any{
				"type": "tool_use", "id": "tool-input-write", "name": "Write",
				"input": map[string]any{"file_path": path, "content": `{"secret":"not persisted"}`},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	events := runtimeGraphLifecycleEvents(message)
	if len(events) != 1 ||
		!strings.EqualFold(events[0].Metadata[runtimeGraphCommandTransportMetadataKey], "true") ||
		events[0].Metadata[runtimeGraphCommandActionMetadataKey] != "input_staging" {
		t.Fatalf("command input staging lifecycle = %+v", events)
	}
	for key, value := range events[0].Metadata {
		if strings.Contains(key, path) || strings.Contains(value, path) || strings.Contains(value, "secret") {
			t.Fatalf("command input staging metadata leaked raw input: %+v", events[0].Metadata)
		}
	}
}

func TestRuntimeGraphKeepsHistoricalGoalAndExecutionMCPTransportInDetail(t *testing.T) {
	t.Parallel()

	operations := []struct {
		domain    string
		operation string
	}{
		{domain: "nexus_goal", operation: "get_goal"},
		{domain: "nexus_goal", operation: "create_goal"},
		{domain: "nexus_goal", operation: "retarget_goal"},
		{domain: "nexus_goal", operation: "audit_objective_alignment"},
		{domain: "nexus_goal", operation: "update_goal"},
		{domain: "nexus_execution", operation: "get_execution"},
		{domain: "nexus_execution", operation: "prepare_plan_execution"},
		{domain: "nexus_execution", operation: "plan_execution"},
		{domain: "nexus_execution", operation: "abandon_execution"},
		{domain: "nexus_execution", operation: "assign_work"},
		{domain: "nexus_execution", operation: "submit_work"},
		{domain: "nexus_execution", operation: "review_work"},
		{domain: "nexus_execution", operation: "block_work"},
		{domain: "nexus_execution", operation: "resume_work"},
		{domain: "nexus_execution", operation: "take_over_work"},
		{domain: "nexus_execution", operation: "audit_execution_alignment"},
		{domain: "nexus_execution", operation: "promote_execution_to_goal"},
	}
	for _, test := range operations {
		for _, name := range []string{
			test.operation,
			"mcp__" + test.domain + "__" + test.operation,
			test.domain + "." + test.operation,
			"functions.mcp__" + test.domain + "__" + test.operation,
		} {
			item := protocol.ExecutionRuntimeNodeRun{
				Kind:   protocol.ExecutionRuntimeNodeTool,
				Name:   name,
				Status: protocol.ExecutionRuntimeNodeFailed,
			}
			if !runtimeGraphIsCommandTransport(item) {
				t.Fatalf("historical managed transport %q was not classified", name)
			}
			if runtimeGraphToolActionVisible(item) {
				t.Fatalf("historical managed transport %q entered the canvas", name)
			}
			if projected := projectRuntimeGraphNode(item, 0, true); projected.Visibility != protocol.ExecutionGraphNodeDetail {
				t.Fatalf("historical managed transport %q visibility = %q", name, projected.Visibility)
			}
		}
	}

	externalSameLeaf := protocol.ExecutionRuntimeNodeRun{
		Kind: protocol.ExecutionRuntimeNodeTool,
		Name: "mcp__slack__update_goal",
	}
	if runtimeGraphIsCommandTransport(externalSameLeaf) {
		t.Fatal("an external MCP capability sharing one legacy leaf was misclassified")
	}
}

func TestRuntimeGraphPromotesShellOnlyWithImportantEvidence(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 18, 23, 30, 0, 0, time.UTC)
	nodes := []protocol.ExecutionRuntimeNodeRun{
		{
			ID: "shell-ordinary", Kind: protocol.ExecutionRuntimeNodeTool, Name: "Bash",
			Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now, UpdatedAt: now,
		},
		{
			ID: "shell-running", Kind: protocol.ExecutionRuntimeNodeTool, Name: "Bash",
			Status: protocol.ExecutionRuntimeNodeRunning, StartedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
		},
		{
			ID: "shell-failed", Kind: protocol.ExecutionRuntimeNodeTool, Name: "Bash",
			Status: protocol.ExecutionRuntimeNodeFailed, StartedAt: now.Add(2 * time.Second), UpdatedAt: now.Add(2 * time.Second),
		},
		{
			ID: "shell-artifact", Kind: protocol.ExecutionRuntimeNodeTool, Name: "Bash",
			Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now.Add(3 * time.Second), UpdatedAt: now.Add(3 * time.Second),
			Artifacts: []protocol.WorkspaceFileArtifactBlock{{
				Type: protocol.ContentBlockTypeWorkspaceFileArtifact,
				Path: "report.md",
			}},
		},
		{
			ID: "shell-explicit", Kind: protocol.ExecutionRuntimeNodeTool, Name: "Bash",
			Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now.Add(4 * time.Second), UpdatedAt: now.Add(4 * time.Second),
			Metadata: map[string]any{protocol.ExecutionRuntimeMetadataWorkGraphVisibility: string(protocol.ExecutionGraphNodeNested)},
		},
		{
			ID: "shell-retry", Kind: protocol.ExecutionRuntimeNodeTool, Name: "Bash",
			Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now.Add(5 * time.Second), UpdatedAt: now.Add(5 * time.Second),
		},
	}
	graph := protocol.ExecutionRuntimeGraph{
		Nodes: nodes,
		Edges: []protocol.ExecutionRuntimeEdgeRun{{
			Kind:         protocol.ExecutionRuntimeEdgeRetry,
			SourceNodeID: "shell-failed", TargetNodeID: "shell-retry",
		}},
	}
	promoted := runtimeGraphPromotedNodeIDs(graph)
	want := map[string]protocol.ExecutionGraphNodeVisibility{
		"shell-ordinary": protocol.ExecutionGraphNodeDetail,
		"shell-running":  protocol.ExecutionGraphNodeNested,
		"shell-failed":   protocol.ExecutionGraphNodeNested,
		"shell-artifact": protocol.ExecutionGraphNodeNested,
		"shell-explicit": protocol.ExecutionGraphNodeNested,
		"shell-retry":    protocol.ExecutionGraphNodeNested,
	}
	for index, node := range nodes {
		_, isPromoted := promoted[node.ID]
		if projected := projectRuntimeGraphNode(node, index, isPromoted); projected.Visibility != want[node.ID] {
			t.Fatalf("%s visibility = %q, want %q", node.ID, projected.Visibility, want[node.ID])
		}
	}
}

func TestRuntimeGraphProjectsObservableActionsAndKeepsSupportingReadsInDetail(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	nodes := []protocol.ExecutionRuntimeNodeRun{
		{
			ID: "runtime-web-fetch", Kind: protocol.ExecutionRuntimeNodeTool,
			SubjectID: "tool-web-fetch", Name: "WebFetch",
			Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now, UpdatedAt: now,
		},
		{
			ID: "runtime-read", Kind: protocol.ExecutionRuntimeNodeTool,
			SubjectID: "tool-read", Name: "Read",
			Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now, UpdatedAt: now,
		},
	}
	promoted := runtimeGraphPromotedNodeIDs(protocol.ExecutionRuntimeGraph{Nodes: nodes})
	for index, item := range nodes {
		_, isPromoted := promoted[item.ID]
		projected := projectRuntimeGraphNode(item, index, isPromoted)
		want := protocol.ExecutionGraphNodeDetail
		if item.Name == "WebFetch" {
			want = protocol.ExecutionGraphNodeNested
		}
		if projected.Visibility != want {
			t.Fatalf("%s visibility = %s, want %s", item.Name, projected.Visibility, want)
		}
	}
}

func TestRuntimeGraphSubagentRecoveryIsVisibleWithoutPromotingUnrelatedSupportingTools(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 10, 30, 0, 0, time.UTC)
	graph := protocol.ExecutionRuntimeGraph{Nodes: []protocol.ExecutionRuntimeNodeRun{
		{
			ID: "runtime-subagent", Kind: protocol.ExecutionRuntimeNodeSubagent,
			SubjectID: "spawn-tool", Status: protocol.ExecutionRuntimeNodeRunning,
			StartedAt: now, UpdatedAt: now,
		},
		{
			ID: "read-failed", Kind: protocol.ExecutionRuntimeNodeTool,
			ParentSubjectID: "spawn-tool", Name: "Read",
			Status:    protocol.ExecutionRuntimeNodeFailed,
			StartedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
		},
		{
			ID: "read-early", Kind: protocol.ExecutionRuntimeNodeTool,
			ParentSubjectID: "spawn-tool", Name: "Read",
			Status:    protocol.ExecutionRuntimeNodeSucceeded,
			StartedAt: now.Add(2 * time.Second), UpdatedAt: now.Add(2 * time.Second),
		},
		{
			ID: "read-recovered", Kind: protocol.ExecutionRuntimeNodeTool,
			ParentSubjectID: "spawn-tool", Name: "Read",
			Status:    protocol.ExecutionRuntimeNodeSucceeded,
			StartedAt: now.Add(3 * time.Second), UpdatedAt: now.Add(3 * time.Second),
		},
		{
			ID: "grep-latest", Kind: protocol.ExecutionRuntimeNodeTool,
			ParentSubjectID: "spawn-tool", Name: "Grep",
			Status:    protocol.ExecutionRuntimeNodeSucceeded,
			StartedAt: now.Add(4 * time.Second), UpdatedAt: now.Add(4 * time.Second),
		},
	}}
	promoted := runtimeGraphPromotedNodeIDs(graph)

	wantVisibility := map[string]protocol.ExecutionGraphNodeVisibility{
		"read-failed":    protocol.ExecutionGraphNodeNested,
		"read-early":     protocol.ExecutionGraphNodeDetail,
		"read-recovered": protocol.ExecutionGraphNodeNested,
		"grep-latest":    protocol.ExecutionGraphNodeDetail,
	}
	for index, node := range graph.Nodes[1:] {
		_, isPromoted := promoted[node.ID]
		projected := projectRuntimeGraphNode(node, index+1, isPromoted)
		if projected.Visibility != wantVisibility[node.ID] {
			t.Fatalf("%s visibility = %s, want %s", node.ID, projected.Visibility, wantVisibility[node.ID])
		}
	}
}

func TestGetLatestViewDoesNotExposePlanlessRuntimeGraphAsWorkGraph(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	repository := &runtimeGraphRepositoryFake{
		fakeRepository: &fakeRepository{},
		graph: protocol.ExecutionRuntimeGraph{
			GraphID:        "round:round-1",
			NodeTotal:      40,
			EdgeTotal:      50,
			NodesTruncated: true,
			EdgesTruncated: true,
			Nodes: []protocol.ExecutionRuntimeNodeRun{
				{
					ID: "runtime-agent-1", GraphID: "round:round-1",
					OwnerUserID: "owner-1", SessionKey: "session-1",
					Kind: protocol.ExecutionRuntimeNodeAgent, SubjectID: "agent-round-1",
					RootRoundID: "round-1", RuntimeRoundID: "round-1",
					AgentRoundID: "agent-round-1", AgentID: "agent-1",
					Status:    protocol.ExecutionRuntimeNodeRunning,
					StartedAt: now, UpdatedAt: now,
				},
				{
					ID: "runtime-tool-1", GraphID: "round:round-1",
					OwnerUserID: "owner-1", SessionKey: "session-1",
					Kind: protocol.ExecutionRuntimeNodeTool, SubjectID: "tool-1",
					RootRoundID: "round-1", RuntimeRoundID: "round-1",
					AgentRoundID: "agent-round-1", AgentID: "agent-1", Name: "search",
					Status:    protocol.ExecutionRuntimeNodeRunning,
					StartedAt: now, UpdatedAt: now,
				},
			},
			Edges: []protocol.ExecutionRuntimeEdgeRun{{
				ID: "runtime-edge-1", GraphID: "round:round-1",
				OwnerUserID: "owner-1", SessionKey: "session-1",
				SourceNodeID: "runtime-agent-1", TargetNodeID: "runtime-tool-1",
				Kind: protocol.ExecutionRuntimeEdgeInvoke, CreatedAt: now,
			}},
		},
	}
	view, err := NewService(repository).GetLatestView(
		context.Background(),
		"owner-1",
		"session-1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if view != nil {
		t.Fatalf("planless runtime graph leaked through WorkGraph read surface: %+v", view)
	}
}

func TestRuntimeGraphRetryKeepsSucceededToolVisibleWithoutToolNameRouting(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC)
	rootID := "runtime-agent-retry"
	failedID := "runtime-tool-failed"
	retriedID := "runtime-tool-retried"
	view := &protocol.ExecutionView{}
	mergeExecutionRuntimeGraph(view, protocol.ExecutionRuntimeGraph{
		GraphID: "round:retry",
		Nodes: []protocol.ExecutionRuntimeNodeRun{
			{
				ID: rootID, Kind: protocol.ExecutionRuntimeNodeAgent,
				SubjectID: "agent-round-retry", AgentRoundID: "agent-round-retry",
				AgentID: "agent-1", Status: protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt: now, UpdatedAt: now,
			},
			{
				ID: failedID, Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "lookup-1", ParentSubjectID: "agent-round-retry",
				AgentRoundID: "agent-round-retry", AgentID: "agent-1",
				Name: "future_web_lookup", Status: protocol.ExecutionRuntimeNodeFailed,
				StartedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
			},
			{
				ID: retriedID, Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "lookup-2", ParentSubjectID: "agent-round-retry",
				AgentRoundID: "agent-round-retry", AgentID: "agent-1",
				Name: "future_web_lookup", Status: protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt: now.Add(2 * time.Second), UpdatedAt: now.Add(2 * time.Second),
			},
		},
		Edges: []protocol.ExecutionRuntimeEdgeRun{
			{ID: "invoke-failed", SourceNodeID: rootID, TargetNodeID: failedID, Kind: protocol.ExecutionRuntimeEdgeInvoke, CreatedAt: now},
			{ID: "loop-failed", SourceNodeID: failedID, TargetNodeID: rootID, Kind: protocol.ExecutionRuntimeEdgeLoopBack, CreatedAt: now.Add(time.Second)},
			{ID: "invoke-retried", SourceNodeID: rootID, TargetNodeID: retriedID, Kind: protocol.ExecutionRuntimeEdgeInvoke, CreatedAt: now.Add(2 * time.Second)},
			{ID: "retry-observed", SourceNodeID: failedID, TargetNodeID: retriedID, Kind: protocol.ExecutionRuntimeEdgeRetry, CreatedAt: now.Add(2 * time.Second)},
		},
	})

	failed := graphNodeByID(view.Graph.Nodes, failedID)
	retried := graphNodeByID(view.Graph.Nodes, retriedID)
	if failed.ID == retried.ID ||
		len(failed.Runs) != 1 || len(retried.Runs) != 1 ||
		failed.LifecycleStatus != string(protocol.ExecutionRuntimeNodeFailed) ||
		retried.LifecycleStatus != string(protocol.ExecutionRuntimeNodeSucceeded) {
		t.Fatalf("retry NodeRuns were folded instead of preserved: failed=%+v retried=%+v", failed, retried)
	}
	if retried.Visibility != protocol.ExecutionGraphNodeNested {
		t.Fatalf("succeeded retry target was hidden after completion: %+v", retried)
	}
	foundRetry := false
	for _, edge := range view.Graph.Edges {
		if edge.Kind == protocol.ExecutionGraphEdgeRetry &&
			edge.SourceNodeID == failedID && edge.TargetNodeID == retriedID {
			foundRetry = true
			break
		}
	}
	if !foundRetry {
		t.Fatalf("agent-chosen retry relation missing: %+v", view.Graph.Edges)
	}
}

func TestRuntimeGraphKeepsUnlinkedFailureAndRecoveryAsSeparateVisibleNodes(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 15, 0, 0, 0, time.UTC)
	rootID := "runtime-agent-unlinked-recovery"
	failedID := "runtime-read-failed"
	succeededID := "runtime-read-succeeded"
	view := &protocol.ExecutionView{}
	mergeExecutionRuntimeGraph(view, protocol.ExecutionRuntimeGraph{
		GraphID: "round:unlinked-recovery",
		Nodes: []protocol.ExecutionRuntimeNodeRun{
			{
				ID: rootID, Kind: protocol.ExecutionRuntimeNodeAgent,
				SubjectID: "agent-round-unlinked", AgentRoundID: "agent-round-unlinked",
				AgentID: "agent-1", Status: protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt: now, UpdatedAt: now,
			},
			{
				ID: failedID, Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "read-1", ParentSubjectID: "agent-round-unlinked",
				AgentRoundID: "agent-round-unlinked", AgentID: "agent-1",
				Name: "Read", Status: protocol.ExecutionRuntimeNodeFailed,
				StartedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
			},
			{
				ID: succeededID, Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "read-2", ParentSubjectID: "agent-round-unlinked",
				AgentRoundID: "agent-round-unlinked", AgentID: "agent-1",
				Name: "Read", Status: protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt: now.Add(2 * time.Second), UpdatedAt: now.Add(2 * time.Second),
			},
		},
		Edges: []protocol.ExecutionRuntimeEdgeRun{
			{ID: "invoke-failed", SourceNodeID: rootID, TargetNodeID: failedID, Kind: protocol.ExecutionRuntimeEdgeInvoke, CreatedAt: now.Add(time.Second)},
			{ID: "invoke-succeeded", SourceNodeID: rootID, TargetNodeID: succeededID, Kind: protocol.ExecutionRuntimeEdgeInvoke, CreatedAt: now.Add(2 * time.Second)},
		},
	})

	failed := graphNodeByID(view.Graph.Nodes, failedID)
	succeeded := graphNodeByID(view.Graph.Nodes, succeededID)
	if failed.ID == succeeded.ID || len(failed.Runs) != 1 || len(succeeded.Runs) != 1 {
		t.Fatalf("unlinked NodeRuns were folded: failed=%+v succeeded=%+v", failed, succeeded)
	}
	if failed.Visibility != protocol.ExecutionGraphNodeNested ||
		succeeded.Visibility != protocol.ExecutionGraphNodeNested {
		t.Fatalf("failure and latest recovery must both stay visible: failed=%+v succeeded=%+v", failed, succeeded)
	}
	if failed.LifecycleStatus != string(protocol.ExecutionRuntimeNodeFailed) ||
		succeeded.LifecycleStatus != string(protocol.ExecutionRuntimeNodeSucceeded) {
		t.Fatalf("independent NodeRun status was lost: failed=%+v succeeded=%+v", failed, succeeded)
	}
	for _, edge := range view.Graph.Edges {
		if edge.Kind == protocol.ExecutionGraphEdgeRetry {
			t.Fatalf("unlinked calls must not invent a retry edge: %+v", view.Graph.Edges)
		}
	}
}

func TestRuntimeGraphViewRepairsMissingParentEdge(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	rootID := "runtime-agent-repair"
	toolID := "runtime-tool-repair"
	view := &protocol.ExecutionView{}
	mergeExecutionRuntimeGraph(view, protocol.ExecutionRuntimeGraph{
		GraphID: "round:repair",
		Nodes: []protocol.ExecutionRuntimeNodeRun{
			{
				ID: rootID, Kind: protocol.ExecutionRuntimeNodeAgent,
				SubjectID: "agent-round-repair", AgentRoundID: "agent-round-repair",
				AgentID: "agent-1", Status: protocol.ExecutionRuntimeNodeRunning,
				StartedAt: now, UpdatedAt: now,
			},
			{
				ID: toolID, Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "tool-repair", AgentRoundID: "agent-round-repair",
				AgentID: "agent-1", Name: "search",
				Status:    protocol.ExecutionRuntimeNodeRunning,
				StartedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
			},
		},
	})

	tool := graphNodeByID(view.Graph.Nodes, toolID)
	if tool.ParentNodeID != rootID ||
		!hasExecutionGraphEdge(
			view.Graph.Edges,
			protocol.ExecutionGraphEdgeInvoke,
			rootID,
			toolID,
		) {
		t.Fatalf("missing runtime parent edge was not repaired: %+v", view.Graph)
	}
}

func TestRuntimeGraphViewBindsManagedRoundsAndFiltersConversationRoots(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	workID := "work-managed"
	coordinatorID := "runtime-coordinator"
	coordinatorNodeID := "coordinator:execution-managed"
	workRoundID := "runtime-work-round"
	toolID := "runtime-work-tool"
	view := &protocol.ExecutionView{
		ID: "execution-managed", CoordinatorAgentID: "lead-1",
		WorkItems: []protocol.ExecutionWorkItemView{{
			ID: workID,
		}},
		Graph: protocol.ExecutionGraphView{Nodes: []protocol.ExecutionGraphNodeView{
			{
				ID: coordinatorNodeID, Kind: protocol.ExecutionGraphNodeAgent,
				Visibility: protocol.ExecutionGraphNodePrimary, AgentID: "lead-1",
			},
			{
				ID: workID, Kind: protocol.ExecutionGraphNodeAgent,
				Visibility: protocol.ExecutionGraphNodePrimary,
				WorkItemID: workID, AgentID: "worker-1",
			},
		}},
	}
	mergeExecutionRuntimeGraph(view, protocol.ExecutionRuntimeGraph{
		GraphID: "execution:execution-managed",
		Nodes: []protocol.ExecutionRuntimeNodeRun{
			{
				ID: coordinatorID, Kind: protocol.ExecutionRuntimeNodeAgent,
				SubjectID: "coord-round", AgentRoundID: "coord-round",
				AgentID: "lead-1", Status: protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt: now, UpdatedAt: now,
				Metadata: map[string]any{"execution_lane": "coordination"},
			},
			{
				ID: "runtime-conversation-only", Kind: protocol.ExecutionRuntimeNodeAgent,
				SubjectID: "chat-round", AgentRoundID: "chat-round",
				AgentID: "observer-1", Status: protocol.ExecutionRuntimeNodeSucceeded,
				StartedAt: now, UpdatedAt: now,
			},
			{
				ID: "runtime-work-agent", Kind: protocol.ExecutionRuntimeNodeAgent,
				SubjectID: workRoundID, AgentRoundID: workRoundID,
				AgentID: "worker-1", Status: protocol.ExecutionRuntimeNodeRunning,
				StartedAt: now, UpdatedAt: now,
				Metadata: map[string]any{
					"execution_lane": "work",
					"work_item_id":   workID,
				},
			},
			{
				ID: toolID, Kind: protocol.ExecutionRuntimeNodeTool,
				SubjectID: "tool-managed", AgentRoundID: workRoundID,
				AgentID: "worker-1", Name: "search",
				Status:    protocol.ExecutionRuntimeNodeRunning,
				StartedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
			},
		},
		Edges: []protocol.ExecutionRuntimeEdgeRun{{
			ID: "runtime-work-invoke", SourceNodeID: "runtime-work-agent",
			TargetNodeID: toolID, Kind: protocol.ExecutionRuntimeEdgeInvoke,
			CreatedAt: now.Add(time.Second),
		}},
	})

	if len(view.Graph.Nodes) != 3 {
		t.Fatalf("managed graph nodes = %+v", view.Graph.Nodes)
	}
	for _, node := range view.Graph.Nodes {
		if node.ID == "runtime-conversation-only" || node.ID == "runtime-work-agent" ||
			node.ID == coordinatorID {
			t.Fatalf("unbound or duplicate runtime Agent leaked into managed graph: %+v", node)
		}
	}
	work := graphNodeByID(view.Graph.Nodes, workID)
	if work.AgentRoundID != workRoundID || work.LifecycleStatus != "running" {
		t.Fatalf("managed Work Item did not absorb exact runtime round: %+v", work)
	}
	coordinator := graphNodeByID(view.Graph.Nodes, coordinatorNodeID)
	if coordinator.AgentRoundID != "coord-round" || coordinator.LifecycleStatus != "succeeded" {
		t.Fatalf("stable coordinator node did not absorb its runtime round: %+v", coordinator)
	}
	if !hasExecutionGraphEdge(
		view.Graph.Edges,
		protocol.ExecutionGraphEdgeInvoke,
		workID,
		toolID,
	) || !hasExecutionGraphEdge(
		view.Graph.Edges,
		protocol.ExecutionGraphEdgeCoordination,
		coordinatorNodeID,
		workID,
	) {
		t.Fatalf("managed runtime graph edges = %+v", view.Graph.Edges)
	}
}

func TestRuntimeGraphViewKeepsEveryManagedAgentRun(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 4, 10, 30, 0, 0, time.UTC)
	view := &protocol.ExecutionView{
		ID:        "execution-history",
		WorkItems: []protocol.ExecutionWorkItemView{{ID: "work-history"}},
		Graph: protocol.ExecutionGraphView{Nodes: []protocol.ExecutionGraphNodeView{{
			ID: "work-history", Kind: protocol.ExecutionGraphNodeAgent,
			Visibility: protocol.ExecutionGraphNodePrimary,
			WorkItemID: "work-history", AgentID: "worker-1",
		}}},
	}
	firstFinished := now.Add(time.Second)
	secondFinished := now.Add(3 * time.Second)
	mergeExecutionRuntimeGraph(view, protocol.ExecutionRuntimeGraph{Nodes: []protocol.ExecutionRuntimeNodeRun{
		{
			ID: "runtime-agent-first", Kind: protocol.ExecutionRuntimeNodeAgent,
			SubjectID: "round-first", AgentRoundID: "round-first", AgentID: "worker-1",
			Status: protocol.ExecutionRuntimeNodeFailed, Failed: true,
			ErrorSummary: "Provider disconnected", StartedAt: now, UpdatedAt: firstFinished,
			FinishedAt: &firstFinished,
			Metadata: map[string]any{
				"execution_lane": "work",
				"work_item_id":   "work-history",
			},
		},
		{
			ID: "runtime-agent-second", Kind: protocol.ExecutionRuntimeNodeAgent,
			SubjectID: "round-second", AgentRoundID: "round-second", AgentID: "worker-1",
			Status: protocol.ExecutionRuntimeNodeSucceeded, ResultSummary: "Recovered",
			StartedAt: now.Add(2 * time.Second), UpdatedAt: secondFinished,
			FinishedAt: &secondFinished,
			Metadata: map[string]any{
				"execution_lane": "work",
				"work_item_id":   "work-history",
			},
		},
	}})

	node := graphNodeByID(view.Graph.Nodes, "work-history")
	if len(node.Runs) != 2 ||
		node.Runs[0].RuntimeNodeID != "runtime-agent-first" ||
		node.Runs[0].ErrorSummary != "Provider disconnected" ||
		node.Runs[1].RuntimeNodeID != "runtime-agent-second" ||
		node.Runs[1].ResultSummary != "Recovered" ||
		node.LifecycleStatus != string(protocol.ExecutionRuntimeNodeSucceeded) {
		t.Fatalf("managed NodeRun history = %+v", node)
	}
}

func TestRuntimeGraphViewKeepsLegacyRoundToolsOnTheirAttemptCycle(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	oldStarted := now
	currentStarted := now.Add(2 * time.Minute)
	view := &protocol.ExecutionView{
		ID: "execution-history",
		WorkItems: []protocol.ExecutionWorkItemView{{
			ID: "work-history", Position: 0,
		}},
		Graph: protocol.ExecutionGraphView{Nodes: []protocol.ExecutionGraphNodeView{
			{
				ID: "attempt-old", Kind: protocol.ExecutionGraphNodeAgent,
				Visibility: protocol.ExecutionGraphNodePrimary,
				WorkItemID: "work-history", AttemptID: "attempt-old",
				AgentID: "worker-1", AgentRoundID: "round-old", Position: 0,
				Runs: []protocol.ExecutionGraphNodeRunView{{StartedAt: &oldStarted}},
			},
			{
				ID: "work-history", Kind: protocol.ExecutionGraphNodeAgent,
				Visibility: protocol.ExecutionGraphNodePrimary,
				WorkItemID: "work-history", AttemptID: "attempt-current",
				AgentID: "worker-1", AgentRoundID: "round-current", Position: 0,
				Runs: []protocol.ExecutionGraphNodeRunView{{StartedAt: &currentStarted}},
			},
		}},
	}
	runtimeStarted := now.Add(time.Second)
	mergeExecutionRuntimeGraph(view, protocol.ExecutionRuntimeGraph{Nodes: []protocol.ExecutionRuntimeNodeRun{
		{
			ID: "runtime-old", Kind: protocol.ExecutionRuntimeNodeAgent,
			SubjectID: "round-old", AgentRoundID: "round-old", AgentID: "worker-1",
			Status:    protocol.ExecutionRuntimeNodeSucceeded,
			StartedAt: now, UpdatedAt: runtimeStarted,
			Metadata: map[string]any{
				"execution_lane": "work",
				"work_item_id":   "work-history",
			},
		},
		{
			ID: "tool-old", Kind: protocol.ExecutionRuntimeNodeTool,
			SubjectID: "tool-old", ParentSubjectID: "round-old",
			AgentRoundID: "round-old", AgentID: "worker-1", Name: "Write",
			Status:    protocol.ExecutionRuntimeNodeSucceeded,
			StartedAt: runtimeStarted, UpdatedAt: runtimeStarted,
		},
	}})

	tool := graphNodeByID(view.Graph.Nodes, "tool-old")
	if tool.ID == "" || tool.ParentNodeID != "attempt-old" || tool.WorkItemID != "work-history" {
		t.Fatalf("legacy historical tool attached to wrong cycle: %+v", tool)
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

func TestAuditExecutionAlignmentRejectsIncompleteReportBeforeGraphWrite(t *testing.T) {
	t.Parallel()

	snapshot := executionSnapshot()
	snapshot.Execution.Objective = "Ship"
	snapshot.Execution.CompletionCriteria = []string{"tested"}
	repository := &runtimeGraphRepositoryFake{
		fakeRepository: &fakeRepository{snapshot: snapshot},
	}
	actor := coordinatorActor()
	actor.RootRoundID = "round-1"
	actor.RuntimeRoundID = "runtime-round-1"
	actor.AgentRoundID = "agent-round-1"
	result, err := NewService(repository).AuditExecutionAlignment(
		context.Background(),
		actor,
		AuditExecutionAlignmentInput{
			ExecutionID:      snapshot.Execution.ID,
			SnapshotRevision: snapshot.Execution.Version,
			CommandID:        "tool-alignment-invalid",
			Report: protocol.ObjectiveAlignmentReport{
				Decision: protocol.ObjectiveAlignmentAligned,
				Summary:  "looks done",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationRejected || len(repository.nodes) != 0 || len(repository.edges) != 0 {
		t.Fatalf("result=%+v nodes=%+v edges=%+v", result, repository.nodes, repository.edges)
	}
}
