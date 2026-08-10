package configuration_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
)

func TestAgentEmotionConversationControlUsesTrustedScopeAndVersionCAS(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	worker := fixture.createAgent(t, "Emotion Worker")
	other := fixture.createAgent(t, "Other Emotion Worker")
	actor := configurationsvc.Actor{
		OwnerUserID: worker.OwnerUserID,
		AgentID:     worker.AgentID,
		SessionKey:  "agent:" + worker.AgentID + ":ws:dm:emotion",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   worker.AgentID,
	}
	bindConfigurationTestRound(t, fixture.services, &actor)

	inspection, err := fixture.services.Configuration.Inspect(
		fixture.ownerCtx,
		actor,
		[]string{configurationsvc.DomainEmotion},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := inspection.Domains[configurationsvc.DomainEmotion]
	payload, err := json.Marshal(snapshot.Values)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Authority != configurationsvc.AuthorityAgentSelf ||
		snapshot.Scope.Kind != configurationsvc.ScopeKindAgent ||
		snapshot.Scope.ID != worker.AgentID ||
		snapshot.StateVersion != 1 ||
		len(snapshot.Access.AllowedOperations) != 3 {
		t.Fatalf("unexpected self emotion scope: %+v", inspection)
	}
	if strings.Contains(string(payload), worker.WorkspacePath) ||
		strings.Contains(string(payload), "state_path") {
		t.Fatalf("emotion inspection leaked absolute workspace state path: %s", payload)
	}

	input := json.RawMessage(`{
		"mood":"curious",
		"energy":8,
		"valence":7,
		"description":"engaged with this problem"
	}`)
	if _, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainEmotion,
			Operation: "set_base",
			Target:    other.AgentID,
			Input:     input,
		},
	); err == nil {
		t.Fatal("Agent must not target another Agent emotion state")
	}
	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainEmotion,
			Operation: "set_base",
			Input:     input,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if plan.RequiresConfirmation ||
		plan.Scope.ID != worker.AgentID ||
		plan.Target != worker.AgentID ||
		plan.StateVersion != snapshot.StateVersion ||
		plan.RuntimeEffect != "next_round" {
		t.Fatalf("unexpected emotion plan: %+v", plan)
	}
	result, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			RequestID:        "emotion-self-base-001",
			Domain:           configurationsvc.DomainEmotion,
			Operation:        "set_base",
			Input:            input,
			ExpectedRevision: plan.CurrentRevision,
			PlanDigest:       plan.PlanDigest,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConfigurationCheck(result.Checks, "configuration_resource_version_advanced") {
		t.Fatalf("emotion write lacked version proof: %+v", result)
	}
	view, err := fixture.services.Core.Agent.GetAgentRuntimeEmotionView(
		fixture.ownerCtx,
		worker.AgentID,
		"dm:"+actor.SessionKey,
		time.Now(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if view.Version != plan.StateVersion+1 || view.Base.Mood != "curious" {
		t.Fatalf("emotion base did not persist: %+v", view)
	}
}

func TestRoomEmotionControlOnlyTouchesCurrentAgentAndConversation(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	host := fixture.createAgent(t, "Emotion Room Host")
	member := fixture.createAgent(t, "Emotion Room Member")
	roomContext, err := fixture.services.Core.Room.CreateRoom(
		fixture.ownerCtx,
		protocol.CreateRoomRequest{
			AgentIDs: []string{host.AgentID, member.AgentID},
			Name:     "Emotion Room", HostAgentID: host.AgentID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	memberActor := configurationsvc.Actor{
		OwnerUserID:    member.OwnerUserID,
		AgentID:        member.AgentID,
		ContextKind:    configurationsvc.ContextKindRoom,
		ContextID:      roomContext.Room.ID,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		SessionKey:     protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID),
		LeaseSessionKey: protocol.BuildRoomAgentSessionKey(
			roomContext.Conversation.ID,
			member.AgentID,
			roomContext.Room.RoomType,
		),
	}
	bindConfigurationTestRound(t, fixture.services, &memberActor)
	inspection, err := fixture.services.Configuration.Inspect(
		fixture.ownerCtx,
		memberActor,
		[]string{configurationsvc.DomainEmotion},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	emotion := inspection.Domains[configurationsvc.DomainEmotion]
	if inspection.Authority != configurationsvc.AuthorityRoomMember ||
		emotion.Scope.ID != member.AgentID ||
		len(emotion.Access.AllowedOperations) != 2 {
		t.Fatalf("unexpected Room emotion boundary: %+v", inspection)
	}
	if _, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		memberActor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainEmotion,
			Operation: "set_base",
			Input: json.RawMessage(`{
				"mood":"forged","energy":1,"valence":1,"description":"forged"
			}`),
		},
	); err == nil {
		t.Fatal("Room member must not change Agent-wide base emotion")
	}

	input := json.RawMessage(`{
		"mood":"encouraged",
		"valence":8,
		"trigger":"the current Room collaboration went well"
	}`)
	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		memberActor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainEmotion,
			Operation: "set_context",
			Input:     input,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		memberActor,
		configurationsvc.ChangeRequest{
			RequestID:        "emotion-room-context-001",
			Domain:           configurationsvc.DomainEmotion,
			Operation:        "set_context",
			Input:            input,
			ExpectedRevision: plan.CurrentRevision,
			PlanDigest:       plan.PlanDigest,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Scope.ID != member.AgentID ||
		!hasConfigurationCheck(result.Checks, "configuration_resource_version_advanced") {
		t.Fatalf("Room emotion apply escaped current Agent scope: %+v", result)
	}
	view, err := fixture.services.Core.Agent.GetAgentRuntimeEmotionView(
		fixture.ownerCtx,
		member.AgentID,
		"room:"+roomContext.Conversation.ID,
		time.Now(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if view.Context == nil ||
		view.Context.Mood != "encouraged" ||
		view.Context.Trigger != "the current Room collaboration went well" {
		t.Fatalf("Room conversation emotion not persisted: %+v", view)
	}
	hostView, err := fixture.services.Core.Agent.GetAgentRuntimeEmotionView(
		fixture.ownerCtx,
		host.AgentID,
		"room:"+roomContext.Conversation.ID,
		time.Now(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if hostView.Context != nil {
		t.Fatalf("member emotion mutation changed Room host state: %+v", hostView)
	}
}
