// INPUT: owner-main、Room host/member 的可信对话身份与版本化 Room/conversation 变更。
// OUTPUT: Room 创建、conversation 生命周期、人工批准、CAS、写后核验和成员拒绝证明。
// POS: nexuscfg 群聊生命周期与权限边界的端到端回归测试。
package configuration_test

import (
	"encoding/json"
	"errors"
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
