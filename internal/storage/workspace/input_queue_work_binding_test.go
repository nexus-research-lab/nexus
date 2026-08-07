package workspace

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestInputQueuePersistsCompleteExecutionWorkBinding(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	location := InputQueueLocation{
		OwnerUserID:    "owner-1",
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  filepath.Join(appfs.UserWorkspaceRootAt(stateRoot, "owner-1"), "agent-worker"),
		SessionKey:     "room:agent:conversation-1:agent-worker",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
	}
	binding := &protocol.ExecutionWorkBinding{
		ExecutionID:  "execution-1",
		PlanID:       "plan-1",
		WorkItemID:   "work-1",
		SpecID:       "spec-1",
		AssignmentID: "assignment-1",
		AttemptID:    "attempt-1",
		DispatchID:   "dispatch-1",
	}
	store := NewInputQueueStore("")
	if _, err := store.Enqueue(location, protocol.InputQueueItem{
		ID:              "execution_dispatch_dispatch-1",
		AgentID:         "agent-worker",
		SourceAgentID:   "agent-lead",
		SourceMessageID: "execution_dispatch_dispatch-1",
		TargetAgentIDs:  []string{"agent-worker"},
		Source:          protocol.InputQueueSourceAgentRoomMessage,
		Content:         "deliver the evidence set",
		DeliveryPolicy:  protocol.ChatDeliveryPolicyQueue,
		WorkBinding:     binding,
	}); err != nil {
		t.Fatal(err)
	}

	replayed, err := NewInputQueueStore("").Snapshot(location)
	if err != nil {
		t.Fatal(err)
	}
	if len(replayed) != 1 || !reflect.DeepEqual(replayed[0].WorkBinding, binding) {
		t.Fatalf("replayed WorkBinding = %+v, want %+v", replayed, binding)
	}

	different := replayed[0]
	different.WorkBinding = cloneTestExecutionWorkBinding(binding)
	different.WorkBinding.AttemptID = "attempt-stale"
	if MatchesInputQueueEnqueueIntent(replayed[0], different) {
		t.Fatal("different Attempt binding matched the same enqueue intent")
	}
}

func TestInputQueueStoreRejectsGuidanceMutationForExecutionBinding(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	location := InputQueueLocation{
		OwnerUserID:    "owner-1",
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  filepath.Join(appfs.UserWorkspaceRootAt(stateRoot, "owner-1"), "agent-worker"),
		SessionKey:     "room:agent:conversation-1:agent-worker",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
	}
	item := protocol.InputQueueItem{
		ID:              "execution_dispatch_dispatch-1",
		AgentID:         "agent-worker",
		SourceAgentID:   "agent-lead",
		SourceMessageID: "execution_dispatch_dispatch-1",
		TargetAgentIDs:  []string{"agent-worker"},
		Source:          protocol.InputQueueSourceAgentRoomMessage,
		Content:         "deliver the evidence set",
		DeliveryPolicy:  protocol.ChatDeliveryPolicyQueue,
		WorkBinding: &protocol.ExecutionWorkBinding{
			ExecutionID:  "execution-1",
			PlanID:       "plan-1",
			WorkItemID:   "work-1",
			SpecID:       "spec-1",
			AssignmentID: "assignment-1",
			AttemptID:    "attempt-1",
			DispatchID:   "dispatch-1",
		},
	}
	store := NewInputQueueStore("")
	if _, err := store.Enqueue(location, item); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateDeliveryPolicy(
		location,
		item.ID,
		protocol.ChatDeliveryPolicyGuide,
		"round-running",
	); !errors.Is(err, protocol.ErrInvalidInputQueueCapabilityEnvelope) {
		t.Fatalf("guide mutation error = %v, want capability envelope error", err)
	}
	items, err := store.Snapshot(location)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].DeliveryPolicy != protocol.ChatDeliveryPolicyQueue {
		t.Fatalf("bound item changed after rejected guidance mutation: %+v", items)
	}
}

func TestInputQueueStoreRejectsInvalidCapabilityEnvelopeAcrossEnqueuePaths(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	location := InputQueueLocation{
		OwnerUserID:    "owner-1",
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  filepath.Join(appfs.UserWorkspaceRootAt(stateRoot, "owner-1"), "agent-worker"),
		SessionKey:     "room:agent:conversation-1:agent-worker",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
	}
	invalid := protocol.InputQueueItem{
		ID:             "invalid-bound-guide",
		AgentID:        "agent-worker",
		TargetAgentIDs: []string{"agent-worker"},
		Source:         protocol.InputQueueSourceAgentRoomMessage,
		Content:        "must not enter an active round as guidance",
		DeliveryPolicy: protocol.ChatDeliveryPolicyGuide,
		WorkBinding: &protocol.ExecutionWorkBinding{
			ExecutionID:  "execution-1",
			PlanID:       "plan-1",
			WorkItemID:   "work-1",
			SpecID:       "spec-1",
			AssignmentID: "assignment-1",
			AttemptID:    "attempt-1",
			DispatchID:   "dispatch-1",
		},
	}
	store := NewInputQueueStore("")
	assertRejected := func(name string, err error) {
		t.Helper()
		if !errors.Is(err, protocol.ErrInvalidInputQueueCapabilityEnvelope) {
			t.Fatalf("%s error = %v, want capability envelope error", name, err)
		}
	}
	_, err := store.Enqueue(location, invalid)
	assertRejected("enqueue", err)
	_, err = store.EnqueueIdempotent(location, invalid, "client-message-1")
	assertRejected("idempotent enqueue", err)
	_, _, err = store.EnqueueBounded(location, invalid, 8)
	assertRejected("bounded enqueue", err)
	_, err = store.EnqueueBatchWithItems([]InputQueueEnqueue{{Location: location, Item: invalid}})
	assertRejected("batch enqueue", err)
}

func cloneTestExecutionWorkBinding(
	source *protocol.ExecutionWorkBinding,
) *protocol.ExecutionWorkBinding {
	if source == nil {
		return nil
	}
	result := *source
	return &result
}

func TestInputQueuePersistsIndependentExecutionReviewBinding(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	location := InputQueueLocation{
		OwnerUserID:    "owner-1",
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  filepath.Join(appfs.UserWorkspaceRootAt(stateRoot, "owner-1"), "agent-lead"),
		SessionKey:     "room:agent:conversation-1:agent-lead",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
	}
	binding := &protocol.ExecutionReviewBinding{
		ExecutionID:      "execution-1",
		PlanID:           "plan-1",
		WorkItemID:       "work-1",
		SpecID:           "spec-1",
		AssignmentID:     "assignment-1",
		SubmissionID:     "submission-1",
		ReviewDispatchID: "review-dispatch-1",
		TargetAgentID:    "agent-lead",
	}
	store := NewInputQueueStore("")
	if _, err := store.Enqueue(location, protocol.InputQueueItem{
		ID:              "execution_review_dispatch_review-dispatch-1",
		AgentID:         "agent-lead",
		SourceAgentID:   "agent-worker",
		SourceMessageID: "execution_review_dispatch_review-dispatch-1",
		TargetAgentIDs:  []string{"agent-lead"},
		Source:          protocol.InputQueueSourceAgentRoomMessage,
		Content:         "review the submitted evidence",
		DeliveryPolicy:  protocol.ChatDeliveryPolicyQueue,
		ReviewBinding:   binding,
	}); err != nil {
		t.Fatal(err)
	}
	replayed, err := NewInputQueueStore("").Snapshot(location)
	if err != nil {
		t.Fatal(err)
	}
	if len(replayed) != 1 ||
		replayed[0].WorkBinding != nil ||
		!reflect.DeepEqual(replayed[0].ReviewBinding, binding) {
		t.Fatalf("replayed ReviewBinding = %+v, want %+v", replayed, binding)
	}
	different := replayed[0]
	cloned := *binding
	cloned.SubmissionID = "submission-stale"
	different.ReviewBinding = &cloned
	if MatchesInputQueueEnqueueIntent(replayed[0], different) {
		t.Fatal("different Submission review binding matched the same enqueue intent")
	}
}
