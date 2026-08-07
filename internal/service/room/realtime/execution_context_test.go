package realtime

import (
	"context"
	"errors"
	"testing"

	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type fakeRoomExecutionContextProvider struct {
	content          string
	err              error
	actor            orchestrationsvc.ActorContext
	evidenceKinds    []orchestrationsvc.PersistenceEvidenceKind
	evidenceCommands []string
}

func (f *fakeRoomExecutionContextProvider) RuntimeContext(
	_ context.Context,
	actor orchestrationsvc.ActorContext,
) (string, error) {
	f.actor = actor
	return f.content, f.err
}

func (f *fakeRoomExecutionContextProvider) RecordPersistenceEvidence(
	_ context.Context,
	_ orchestrationsvc.ActorContext,
	kind orchestrationsvc.PersistenceEvidenceKind,
	commandID string,
) error {
	f.evidenceKinds = append(f.evidenceKinds, kind)
	f.evidenceCommands = append(f.evidenceCommands, commandID)
	return nil
}

func TestRoomExecutionContextualInputsUseCurrentMemberIdentity(t *testing.T) {
	service := &Service{}
	provider := &fakeRoomExecutionContextProvider{content: "<nexus_execution_context />"}
	service.SetExecutionContextProvider(provider)
	actor := orchestrationsvc.ActorContext{
		OwnerUserID:    "owner-1",
		SessionKey:     "room:group:conversation-1",
		AgentID:        "analyst",
		RoomID:         "room-1",
		ConversationID: "conversation-1",
	}

	inputs, err := service.executionContextualInputs(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 1 || inputs[0].Name != "execution" ||
		inputs[0].Content != "<nexus_execution_context />" {
		t.Fatalf("inputs = %#v", inputs)
	}
	if provider.actor != actor {
		t.Fatalf("actor = %#v, want %#v", provider.actor, actor)
	}
}

func TestRoomExecutionContextualInputsFailClosed(t *testing.T) {
	service := &Service{}
	service.SetExecutionContextProvider(&fakeRoomExecutionContextProvider{err: errors.New("snapshot unavailable")})
	if _, err := service.executionContextualInputs(
		context.Background(),
		orchestrationsvc.ActorContext{},
	); err == nil {
		t.Fatal("provider failure did not fail the slot")
	}
}

func TestRoomCompactBoundaryRecordsTrustedExecutionEvidence(t *testing.T) {
	provider := &fakeRoomExecutionContextProvider{}
	execution := &slotExecution{
		service: &Service{executionContext: provider},
		slot: &activeRoomSlot{
			RuntimeSessionKey: "room-runtime:analyst",
			AgentRoundID:      "agent-round-1",
		},
	}
	actor := orchestrationsvc.ActorContext{AgentID: "analyst"}
	execution.observeExecutionPersistenceEvidence(actor, sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeSystem,
		System: &sdkprotocol.SystemMessage{
			Subtype: "compact_boundary",
		},
	})
	if len(provider.evidenceKinds) != 1 ||
		provider.evidenceKinds[0] != orchestrationsvc.PersistenceEvidenceContextBoundary ||
		provider.evidenceCommands[0] != "runtime:room-runtime:analyst:agent-round-1:compact-boundary" {
		t.Fatalf("evidence = %#v / %#v", provider.evidenceKinds, provider.evidenceCommands)
	}
}
