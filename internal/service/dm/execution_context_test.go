package dm

import (
	"context"
	"errors"
	"testing"

	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type fakeDMExecutionContextProvider struct {
	content          string
	err              error
	actor            orchestrationsvc.ActorContext
	evidenceKinds    []orchestrationsvc.PersistenceEvidenceKind
	evidenceCommands []string
}

func (f *fakeDMExecutionContextProvider) RuntimeContext(
	_ context.Context,
	actor orchestrationsvc.ActorContext,
) (string, error) {
	f.actor = actor
	return f.content, f.err
}

func (f *fakeDMExecutionContextProvider) RecordPersistenceEvidence(
	_ context.Context,
	_ orchestrationsvc.ActorContext,
	kind orchestrationsvc.PersistenceEvidenceKind,
	commandID string,
) error {
	f.evidenceKinds = append(f.evidenceKinds, kind)
	f.evidenceCommands = append(f.evidenceCommands, commandID)
	return nil
}

func TestDMExecutionContextualInputsUseAuthoritativeActorContext(t *testing.T) {
	service := &Service{}
	provider := &fakeDMExecutionContextProvider{content: "<nexus_execution_context />"}
	service.SetExecutionContextProvider(provider)
	actor := orchestrationsvc.ActorContext{
		OwnerUserID: "owner-1",
		SessionKey:  "agent:nexus:ws:dm:chat",
		AgentID:     "agent-1",
		PlanMode:    true,
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

func TestDMExecutionContextualInputsFailClosed(t *testing.T) {
	service := &Service{}
	service.SetExecutionContextProvider(&fakeDMExecutionContextProvider{err: errors.New("snapshot unavailable")})
	if _, err := service.executionContextualInputs(
		context.Background(),
		orchestrationsvc.ActorContext{},
	); err == nil {
		t.Fatal("provider failure did not fail the round")
	}
}

func TestDMCompactBoundaryRecordsTrustedExecutionEvidence(t *testing.T) {
	provider := &fakeDMExecutionContextProvider{}
	runner := &roundRunner{
		service:      &Service{executionContext: provider},
		sessionKey:   "agent:nexus:ws:dm:chat",
		roundID:      "round-1",
		agentRoundID: "agent-round-1",
	}
	actor := orchestrationsvc.ActorContext{AgentID: "agent-1"}
	runner.observeExecutionPersistenceEvidence(actor, sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeSystem,
		System: &sdkprotocol.SystemMessage{
			Subtype: "compact_boundary",
		},
	})
	if len(provider.evidenceKinds) != 1 ||
		provider.evidenceKinds[0] != orchestrationsvc.PersistenceEvidenceContextBoundary ||
		provider.evidenceCommands[0] != "runtime:agent:nexus:ws:dm:chat:agent-round-1:compact-boundary" {
		t.Fatalf("evidence = %#v / %#v", provider.evidenceKinds, provider.evidenceCommands)
	}
}

func TestRoundRunnerRuntimeInputOptionsUsesCanonicalUserText(t *testing.T) {
	runner := &roundRunner{
		content: "  用户原始问题  ",
		inputOptions: sdkprotocol.OutboundMessageOptions{
			RecallQuery: "stale wrapped prompt",
		},
	}

	options := runner.runtimeInputOptions()
	if options.RecallQuery != "用户原始问题" {
		t.Fatalf("RecallQuery = %q, want canonical user text", options.RecallQuery)
	}
}

func TestRoundRunnerRuntimeInputOptionsSkipsInternalInput(t *testing.T) {
	runner := &roundRunner{
		content:  "继续内部任务",
		internal: true,
		inputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose:     "goal_continuation",
			RecallQuery: "must be cleared",
		},
	}

	if query := runner.runtimeInputOptions().RecallQuery; query != "" {
		t.Fatalf("RecallQuery = %q, want internal input skipped", query)
	}
}

func TestRoundRunnerContextualInputsCombinesGoalAndRecovery(t *testing.T) {
	recovery := runtimectx.NewContextualInputBlock(
		runtimectx.ContextualInputNameRoundRecovery,
		"Recorded terminal reason: content_filtered.",
		0,
		nil,
	)
	runner := &roundRunner{
		sessionKey:     "dm:agent-a:session-a",
		goalContext:    "goal context",
		goalIDForUsage: "goal-a",
		recoveryContext: []runtimectx.ContextualInputBlock{
			recovery,
		},
	}
	inputs := runner.contextualInputs()
	if len(inputs) != 2 || inputs[0].Name != goalContextualInputName ||
		inputs[1].Name != runtimectx.ContextualInputNameRoundRecovery {
		t.Fatalf("DM contextual inputs 未同时保留 Goal 与失败恢复上下文: %+v", inputs)
	}
}

func TestRoundRunnerAtomicSlashInputOmitsGoalAndRecovery(t *testing.T) {
	runner := &roundRunner{
		atomicInput:    true,
		sessionKey:     "dm:agent-a:session-a",
		goalContext:    "goal context",
		goalIDForUsage: "goal-a",
		recoveryContext: []runtimectx.ContextualInputBlock{
			runtimectx.NewContextualInputBlock(
				runtimectx.ContextualInputNameRoundRecovery,
				"private recovery",
				0,
				nil,
			),
		},
	}
	if inputs := runner.contextualInputs(); len(inputs) != 0 {
		t.Fatalf("atomic Slash contextual inputs = %+v, want empty", inputs)
	}
}
