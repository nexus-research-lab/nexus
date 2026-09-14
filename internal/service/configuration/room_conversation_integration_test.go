// INPUT: owner-main、Room host/member 身份与版本化 Room/conversation 变更。
// OUTPUT: 生命周期权限、人工批准、CAS、写后核验、删除幂等与失败恢复证明。
// POS: configuration Room 与 conversation 生命周期集成测试。
package configuration_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
)

func TestConversationalRoomAndConversationLifecycleHonorsHostBoundaryAndCAS(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	host := fixture.createAgent(t, "Lifecycle Host")
	member := fixture.createAgent(t, "Lifecycle Member")
	ownerActor := configurationsvc.Actor{
		OwnerUserID: fixture.main.OwnerUserID,
		AgentID:     fixture.main.AgentID,
		SessionKey:  "agent:" + fixture.main.AgentID + ":ws:dm:room-lifecycle",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   fixture.main.AgentID,
	}
	bindConfigurationTestRound(t, fixture.services, &ownerActor)

	createInput := json.RawMessage(`{
		"agent_ids":["` + host.AgentID + `","` + member.AgentID + `"],
		"name":"Conversation lifecycle Room",
		"title":"Initial topic",
		"host_agent_id":"` + host.AgentID + `"
	}`)
	createPlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		ownerActor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainRooms, Operation: "create", Input: createInput,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if createPlan.Target != "" || createPlan.StateVersion != 0 ||
		!createPlan.RequiresConfirmation {
		t.Fatalf("unexpected Room create plan: %+v", createPlan)
	}
	createRequest := configurationsvc.ChangeRequest{
		RequestID: "room-lifecycle-create-001",
		Domain:    configurationsvc.DomainRooms, Operation: "create", Input: createInput,
		ExpectedRevision: createPlan.CurrentRevision,
		PlanDigest:       createPlan.PlanDigest,
	}
	if _, err = fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		ownerActor,
		createRequest,
	); err == nil {
		t.Fatal("Room creation must require interactive human approval")
	}
	approveConfigurationTestChange(
		t, fixture.services, fixture.ownerCtx, ownerActor, createRequest, createPlan,
	)
	createResult, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		ownerActor,
		createRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConfigurationCheck(createResult.Checks, "room_creation_verified") {
		t.Fatalf("Room creation lacked write-after proof: %+v", createResult)
	}

	rooms, err := fixture.services.Core.Room.ListRooms(fixture.ownerCtx, 100)
	if err != nil {
		t.Fatal(err)
	}
	var createdRoomID string
	for _, item := range rooms {
		if item.Room.Name == "Conversation lifecycle Room" {
			createdRoomID = item.Room.ID
			break
		}
	}
	if createdRoomID == "" {
		t.Fatal("created Room was not visible in owner Room catalog")
	}
	contexts, err := fixture.services.Core.Room.GetRoomContexts(
		fixture.ownerCtx,
		createdRoomID,
	)
	if err != nil {
		t.Fatal(err)
	}
	initialConversationID := contexts[0].Conversation.ID
	if err = fixture.services.Core.Room.MarkConversationStarted(
		fixture.ownerCtx,
		initialConversationID,
		time.Now(),
	); err != nil {
		t.Fatal(err)
	}

	hostActor := roomConfigurationActor(host, createdRoomID, initialConversationID)
	bindConfigurationTestRound(t, fixture.services, &hostActor)
	memberActor := roomConfigurationActor(member, createdRoomID, initialConversationID)
	bindConfigurationTestRound(t, fixture.services, &memberActor)
	if _, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		memberActor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainRooms, Operation: "create_conversation",
			Target: createdRoomID, Input: json.RawMessage(`{"title":"forbidden"}`),
		},
	); err == nil {
		t.Fatal("ordinary Room member must not manage conversation lifecycle")
	}

	createConversationInput := json.RawMessage(`{"title":"Host-created topic"}`)
	createConversationPlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		hostActor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainRooms, Operation: "create_conversation",
			Target: createdRoomID, Input: createConversationInput,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	createConversationRequest := configurationsvc.ChangeRequest{
		RequestID: "room-conversation-create-001",
		Domain:    configurationsvc.DomainRooms, Operation: "create_conversation",
		Target: createdRoomID, Input: createConversationInput,
		ExpectedRevision: createConversationPlan.CurrentRevision,
		PlanDigest:       createConversationPlan.PlanDigest,
	}
	approveConfigurationTestChange(
		t,
		fixture.services,
		fixture.ownerCtx,
		hostActor,
		createConversationRequest,
		createConversationPlan,
	)
	createConversationResult, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		hostActor,
		createConversationRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConfigurationCheck(
		createConversationResult.Checks,
		"room_conversation_creation_verified",
	) || !hasConfigurationCheck(
		createConversationResult.Checks,
		"configuration_resource_version_advanced",
	) {
		t.Fatalf("conversation creation lacked CAS/write-after proof: %+v", createConversationResult)
	}
	contexts, err = fixture.services.Core.Room.GetRoomContexts(fixture.ownerCtx, createdRoomID)
	if err != nil {
		t.Fatal(err)
	}
	var createdConversationID string
	for _, contextValue := range contexts {
		if contextValue.Conversation.Title == "Host-created topic" {
			createdConversationID = contextValue.Conversation.ID
			break
		}
	}
	if createdConversationID == "" {
		t.Fatal("created conversation was not visible in Room truth source")
	}

	updateInput := json.RawMessage(
		`{"conversation_id":"` + createdConversationID + `","title":"Renamed by host"}`,
	)
	updateResult := applyApprovedConfigurationChange(
		t,
		fixture,
		hostActor,
		"room-conversation-update-001",
		"update_conversation",
		createdRoomID,
		updateInput,
	)
	if !hasConfigurationCheck(updateResult.Checks, "room_conversation_update_verified") {
		t.Fatalf("conversation update lacked write-after proof: %+v", updateResult)
	}

	deleteInput := json.RawMessage(
		`{"conversation_id":"` + createdConversationID + `"}`,
	)
	stalePlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		hostActor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainRooms, Operation: "delete_conversation",
			Target: createdRoomID, Input: deleteInput,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	description := "version advanced outside the stale plan"
	if _, err = fixture.services.Core.Room.UpdateRoom(
		fixture.ownerCtx,
		createdRoomID,
		protocol.UpdateRoomRequest{
			Description:                  &description,
			ExpectedConfigurationVersion: &stalePlan.StateVersion,
		},
	); err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		hostActor,
		configurationsvc.ChangeRequest{
			RequestID: "room-conversation-delete-stale-001",
			Domain:    configurationsvc.DomainRooms, Operation: "delete_conversation",
			Target: createdRoomID, Input: deleteInput,
			ExpectedRevision: stalePlan.CurrentRevision,
			PlanDigest:       stalePlan.PlanDigest,
		},
	); err == nil {
		t.Fatal("stale conversation deletion plan must fail closed")
	}
	if _, err = fixture.services.Core.Room.GetConversationContext(
		fixture.ownerCtx,
		createdConversationID,
	); err != nil {
		t.Fatalf("stale plan removed conversation: %v", err)
	}

	deleteResult := applyApprovedConfigurationChange(
		t,
		fixture,
		hostActor,
		"room-conversation-delete-001",
		"delete_conversation",
		createdRoomID,
		deleteInput,
	)
	if !hasConfigurationCheck(deleteResult.Checks, "room_conversation_deletion_verified") ||
		!hasConfigurationCheck(deleteResult.Checks, "configuration_resource_version_advanced") {
		t.Fatalf("conversation deletion lacked absence/CAS proof: %+v", deleteResult)
	}
	if _, err = fixture.services.Core.Room.GetConversationContext(
		fixture.ownerCtx,
		createdConversationID,
	); !errors.Is(err, roomsvc.ErrConversationNotFound) {
		t.Fatalf("deleted conversation err = %v, want ErrConversationNotFound", err)
	}
}

func roomConfigurationActor(
	agent *protocol.Agent,
	roomID string,
	conversationID string,
) configurationsvc.Actor {
	return configurationsvc.Actor{
		OwnerUserID:    agent.OwnerUserID,
		AgentID:        agent.AgentID,
		ContextKind:    configurationsvc.ContextKindRoom,
		ContextID:      roomID,
		RoomID:         roomID,
		ConversationID: conversationID,
		SessionKey:     protocol.BuildRoomSharedSessionKey(conversationID),
		LeaseSessionKey: protocol.BuildRoomAgentSessionKey(
			conversationID,
			agent.AgentID,
			protocol.RoomTypeGroup,
		),
	}
}

func applyApprovedConfigurationChange(
	t *testing.T,
	fixture scopedConfigurationFixture,
	actor configurationsvc.Actor,
	requestID string,
	operation string,
	target string,
	input json.RawMessage,
) *configurationsvc.ApplyResult {
	t.Helper()
	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainRooms, Operation: operation,
			Target: target, Input: input,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := configurationsvc.ChangeRequest{
		RequestID: requestID,
		Domain:    configurationsvc.DomainRooms, Operation: operation,
		Target: target, Input: input,
		ExpectedRevision: plan.CurrentRevision,
		PlanDigest:       plan.PlanDigest,
	}
	approveConfigurationTestChange(
		t, fixture.services, fixture.ownerCtx, actor, request, plan,
	)
	result, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		request,
	)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

type failingRoomDeletionGoalCleaner struct {
	err error
}

type recordingRoomDeletionNotifier struct {
	mu      sync.Mutex
	roomIDs []string
	reasons []string
}

func (*recordingRoomDeletionNotifier) AgentChanged(context.Context, string, string) {}

func (n *recordingRoomDeletionNotifier) RoomChanged(
	_ context.Context,
	roomID string,
	_ string,
	reason string,
) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.roomIDs = append(n.roomIDs, roomID)
	n.reasons = append(n.reasons, reason)
}

func (*recordingRoomDeletionNotifier) RoomMemberChanged(context.Context, string, string, bool) {}

func (n *recordingRoomDeletionNotifier) hasDeletion(roomID string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	for index := range n.roomIDs {
		if n.roomIDs[index] == roomID && n.reasons[index] == "room_deleted" {
			return true
		}
	}
	return false
}

func (f failingRoomDeletionGoalCleaner) DeleteGoalsForRoomConversations(
	context.Context,
	[]string,
) (int, error) {
	return 0, f.err
}

func (f failingRoomDeletionGoalCleaner) DeleteGoalsForRoomMember(
	context.Context,
	string,
	[]string,
) (int, error) {
	return 0, f.err
}

func TestOwnerMainRoomDeleteRequiresApprovalVerifiesAbsenceAndReplays(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	host := fixture.createAgent(t, "Delete Room Host")
	roomContext, err := fixture.services.Core.Room.CreateRoom(
		fixture.ownerCtx,
		protocol.CreateRoomRequest{
			AgentIDs:    []string{host.AgentID},
			Name:        "Conversational delete Room",
			HostAgentID: host.AgentID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	notifier := &recordingRoomDeletionNotifier{}
	fixture.services.Configuration.SetNotifier(notifier)

	hostActor := configurationsvc.Actor{
		OwnerUserID:    host.OwnerUserID,
		AgentID:        host.AgentID,
		ContextKind:    configurationsvc.ContextKindRoom,
		ContextID:      roomContext.Room.ID,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		SessionKey:     protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID),
		LeaseSessionKey: protocol.BuildRoomAgentSessionKey(
			roomContext.Conversation.ID,
			host.AgentID,
			roomContext.Room.RoomType,
		),
	}
	bindConfigurationTestRound(t, fixture.services, &hostActor)
	hostInspection, err := fixture.services.Configuration.Inspect(
		fixture.ownerCtx,
		hostActor,
		[]string{configurationsvc.DomainRooms},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if operations := hostInspection.Domains[configurationsvc.DomainRooms].Access.AllowedOperations; slices.Contains(operations, "delete") {
		t.Fatalf("Room host unexpectedly received rooms.delete: %v", operations)
	}
	if _, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		hostActor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainRooms,
			Operation: "delete",
			Target:    roomContext.Room.ID,
		},
	); err == nil {
		t.Fatal("Room host must not delete even its current Room")
	}
	fixture.services.Runtime.MarkRoundFinished(
		hostActor.LeaseSessionKey,
		hostActor.LeaseRoundID,
	)

	ownerActor := configurationsvc.Actor{
		OwnerUserID: fixture.main.OwnerUserID,
		AgentID:     fixture.main.AgentID,
		SessionKey:  "agent:" + fixture.main.AgentID + ":ws:dm:room-delete-success",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   fixture.main.AgentID,
	}
	bindConfigurationTestRound(t, fixture.services, &ownerActor)
	if _, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		ownerActor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainRooms,
			Operation: "delete",
		},
	); err == nil {
		t.Fatal("owner Room delete must bind an exact room_id target")
	}
	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		ownerActor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainRooms,
			Operation: "delete",
			Target:    roomContext.Room.ID,
			Input:     json.RawMessage(`{}`),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if plan.StateVersion != roomContext.Room.ConfigurationVersion ||
		!plan.RequiresConfirmation ||
		plan.Risk != "destructive" {
		t.Fatalf("Room delete plan does not bind version/approval: %+v", plan)
	}
	stalePlan := plan
	newerDescription := "updated after delete plan"
	updatedRoom, err := fixture.services.Core.Room.UpdateRoom(
		fixture.ownerCtx,
		roomContext.Room.ID,
		protocol.UpdateRoomRequest{
			Description:                  &newerDescription,
			ExpectedConfigurationVersion: &stalePlan.StateVersion,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		ownerActor,
		configurationsvc.ChangeRequest{
			RequestID:        "room-delete-stale-plan-01",
			Domain:           configurationsvc.DomainRooms,
			Operation:        "delete",
			Target:           roomContext.Room.ID,
			Input:            json.RawMessage(`{}`),
			ExpectedRevision: stalePlan.CurrentRevision,
			PlanDigest:       stalePlan.PlanDigest,
		},
	); err == nil {
		t.Fatal("stale Room delete plan must not remove newer configuration")
	}
	if preserved, getErr := fixture.services.Core.Room.GetRoom(
		fixture.ownerCtx,
		roomContext.Room.ID,
	); getErr != nil ||
		preserved.Room.ConfigurationVersion != updatedRoom.Room.ConfigurationVersion ||
		preserved.Room.Description != newerDescription {
		t.Fatalf("stale conversational delete damaged Room: room=%+v err=%v", preserved, getErr)
	}
	plan, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		ownerActor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainRooms,
			Operation: "delete",
			Target:    roomContext.Room.ID,
			Input:     json.RawMessage(`{}`),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := configurationsvc.ChangeRequest{
		RequestID:        "room-delete-success-01",
		Domain:           configurationsvc.DomainRooms,
		Operation:        "delete",
		Target:           roomContext.Room.ID,
		Input:            json.RawMessage(`{}`),
		ExpectedRevision: plan.CurrentRevision,
		PlanDigest:       plan.PlanDigest,
	}
	if _, err = fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		ownerActor,
		request,
	); err == nil {
		t.Fatal("Room delete must reject model-only confirmation without interactive human approval")
	}
	approveConfigurationTestChange(
		t,
		fixture.services,
		fixture.ownerCtx,
		ownerActor,
		request,
		plan,
	)
	applied, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		ownerActor,
		request,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied ||
		!hasConfigurationCheck(applied.Checks, "configuration_target_deleted") ||
		!notifier.hasDeletion(roomContext.Room.ID) {
		t.Fatalf("Room deletion was not verified: %+v", applied)
	}
	if _, err = fixture.services.Core.Room.GetRoom(
		fixture.ownerCtx,
		roomContext.Room.ID,
	); !errors.Is(err, roomsvc.ErrRoomNotFound) {
		t.Fatalf("Room still exists after verified deletion: %v", err)
	}

	replayed, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		ownerActor,
		request,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Applied || !replayed.IdempotentReplay {
		t.Fatalf("Room deletion request_id did not replay safely: %+v", replayed)
	}
}

func TestRoomDeletePostCommitCleanupFailureIsAuditedForReconcile(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	worker := fixture.createAgent(t, "Delete Room Reconcile Worker")
	roomContext, err := fixture.services.Core.Room.CreateRoom(
		fixture.ownerCtx,
		protocol.CreateRoomRequest{
			AgentIDs: []string{worker.AgentID},
			Name:     "Room reconcile",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	fixture.services.Core.Room.SetGoalCleaner(failingRoomDeletionGoalCleaner{
		err: errors.New("injected Room Goal cleanup failure"),
	})
	actor := configurationsvc.Actor{
		OwnerUserID: fixture.main.OwnerUserID,
		AgentID:     fixture.main.AgentID,
		SessionKey:  "agent:" + fixture.main.AgentID + ":ws:dm:room-delete-reconcile",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   fixture.main.AgentID,
	}
	bindConfigurationTestRound(t, fixture.services, &actor)
	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainRooms,
			Operation: "delete",
			Target:    roomContext.Room.ID,
			Input:     json.RawMessage(`{}`),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := configurationsvc.ChangeRequest{
		RequestID:        "room-delete-reconcile-01",
		Domain:           configurationsvc.DomainRooms,
		Operation:        "delete",
		Target:           roomContext.Room.ID,
		Input:            json.RawMessage(`{}`),
		ExpectedRevision: plan.CurrentRevision,
		PlanDigest:       plan.PlanDigest,
	}
	approveConfigurationTestChange(
		t,
		fixture.services,
		fixture.ownerCtx,
		actor,
		request,
		plan,
	)
	if _, err = fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		request,
	); err == nil {
		t.Fatalf("Room post-commit cleanup error classification=%v", err)
	}
	if _, err = fixture.services.Core.Room.GetRoom(
		fixture.ownerCtx,
		roomContext.Room.ID,
	); !errors.Is(err, roomsvc.ErrRoomNotFound) {
		t.Fatalf("Room database deletion did not commit: %v", err)
	}
	records, err := fixture.services.Configuration.ListChanges(
		fixture.ownerCtx,
		actor,
		configurationsvc.DomainRooms,
		10,
	)
	if err != nil {
		t.Fatal(err)
	}
	var matched *configurationsvc.AuditRecord
	for index := range records {
		if records[index].RequestID == request.RequestID {
			matched = &records[index]
			break
		}
	}
	if matched == nil ||
		matched.Status != "reconcile_required" ||
		!strings.Contains(string(matched.Result), `"applied":true`) {
		t.Fatalf("Room reconcile audit record=%+v", matched)
	}
	if _, err = fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		request,
	); err == nil || !strings.Contains(err.Error(), "reconcile") {
		t.Fatalf("reconcile request_id must not execute deletion again: %v", err)
	}
}
