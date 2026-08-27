package realtime

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
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

func TestRoomSlotRuntimeInputOptionsUsesDirectTrigger(t *testing.T) {
	roundValue := &activeRoomRound{
		InputOptions: sdkprotocol.OutboundMessageOptions{
			RecallQuery: "wrapped public feed",
		},
	}
	slot := &activeRoomSlot{
		Trigger: roomTrigger{Content: "  用户直接问题  "},
	}

	options := roomSlotRuntimeInputOptions(roundValue, slot)
	if options.RecallQuery != "用户直接问题" {
		t.Fatalf("RecallQuery = %q, want direct slot trigger", options.RecallQuery)
	}
}

func TestRoomSlotRuntimeInputOptionsSkipsGoalContinuation(t *testing.T) {
	roundValue := &activeRoomRound{
		Internal: true,
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose:     "goal_continuation",
			RecallQuery: "must be cleared",
		},
	}
	slot := &activeRoomSlot{
		Trigger: roomTrigger{TriggerType: "goal_continuation", Content: "继续"},
	}

	if query := roomSlotRuntimeInputOptions(roundValue, slot).RecallQuery; query != "" {
		t.Fatalf("RecallQuery = %q, want goal continuation skipped", query)
	}
}

func TestSlotExecutionRecoveryContextOnlyForExplicitUserTrigger(t *testing.T) {
	history := []protocol.Message{{
		"role":        "assistant",
		"agent_id":    "agent-a",
		"is_complete": true,
		"result_summary": map[string]any{
			"subtype":         "error",
			"is_error":        true,
			"terminal_reason": protocol.ProviderFailureContentFiltered,
		},
	}}
	execution := &slotExecution{
		round:   &activeRoomRound{},
		slot:    &activeRoomSlot{AgentID: "agent-a", Trigger: roomTrigger{TriggerType: "public_chat"}},
		history: history,
	}
	inputs := execution.contextualInputs()
	if len(inputs) != 1 || inputs[0].Name != runtimectx.ContextualInputNameRoundRecovery {
		t.Fatalf("真实用户触发应带上目标 Agent 的失败恢复上下文: %+v", inputs)
	}

	execution.slot.Trigger.TriggerType = "public_mention"
	if got := execution.contextualInputs(); len(got) != 0 {
		t.Fatalf("Agent 接力唤醒不应消费用户轮失败恢复语义: %+v", got)
	}

	execution.slot.Trigger.TriggerType = "public_chat"
	execution.round.Internal = true
	if got := execution.contextualInputs(); len(got) != 0 {
		t.Fatalf("内部轮次不应注入用户轮失败恢复语义: %+v", got)
	}
}
