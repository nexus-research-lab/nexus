package automation

import (
	"context"
	"strings"
	"sync"
	"testing"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type mutableAutomationAgentAuthority struct {
	mu     sync.Mutex
	agents map[string]protocol.Agent
}

func (f *mutableAutomationAgentAuthority) EnsureReady(context.Context) error {
	return nil
}

func (f *mutableAutomationAgentAuthority) GetAgent(_ context.Context, agentID string) (*protocol.Agent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	value, ok := f.agents[strings.TrimSpace(agentID)]
	if !ok {
		return nil, nil
	}
	result := value
	return &result, nil
}

func (f *mutableAutomationAgentAuthority) setMain(agentID string, isMain bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	value := f.agents[strings.TrimSpace(agentID)]
	value.IsMain = isMain
	f.agents[strings.TrimSpace(agentID)] = value
}

func TestServiceRejectsAgentOriginDeliveryToAnotherAgent(t *testing.T) {
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		newAutomationTestDB(t),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	ownerCtx := automationMCPTestOwnerContext("user-1")
	agentCtx := automationexec.WithActorAgentID(ownerCtx, "agent-1")
	sourceSession := protocol.BuildAgentSessionKey(
		"agent-1",
		protocol.SessionChannelInternalSegment,
		protocol.RoomTypeDM,
		"operator",
		"",
	)
	otherInbox := protocol.BuildAgentSessionKey(
		"agent-2",
		protocol.SessionChannelInternalSegment,
		protocol.RoomTypeDM,
		protocol.AutomationInboxSessionRef,
		"",
	)

	_, err := service.CreateTask(agentCtx, automationdomain.CreateJobInput{
		Name:        "forged-delivery",
		AgentID:     "agent-1",
		Instruction: "send elsewhere",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(60),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetNamed,
			NamedSessionKey: "forged-delivery",
		},
		Delivery: automationdomain.DeliveryTarget{
			Mode:    automationdomain.DeliveryModeExplicit,
			Channel: protocol.SessionChannelInternalSegment,
			To:      otherInbox,
		},
		Source: automationdomain.Source{
			Kind:           automationdomain.SourceKindAgent,
			CreatorAgentID: "agent-1",
			ContextType:    "agent",
			ContextID:      "agent-1",
			SessionKey:     sourceSession,
		},
		Enabled: true,
	})
	if err == nil {
		t.Fatal("Agent-origin create should reject delivery to another Agent")
	}
	if !strings.Contains(err.Error(), "another agent") {
		t.Fatalf("unexpected cross-Agent delivery error: %v", err)
	}

	ownInbox := protocol.BuildAgentSessionKey(
		"agent-1",
		protocol.SessionChannelInternalSegment,
		protocol.RoomTypeDM,
		protocol.AutomationInboxSessionRef,
		"",
	)
	source := automationdomain.Source{
		Kind:           automationdomain.SourceKindAgent,
		CreatorAgentID: "agent-1",
		ContextType:    "agent",
		ContextID:      "agent-1",
		SessionKey:     sourceSession,
	}
	created, err := service.CreateTask(agentCtx, automationdomain.CreateJobInput{
		Name:        "self-delivery",
		AgentID:     "agent-1",
		Instruction: "send to self",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(60),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetNamed,
			NamedSessionKey: "self-delivery",
		},
		Delivery: automationdomain.DeliveryTarget{
			Mode:    automationdomain.DeliveryModeExplicit,
			Channel: protocol.SessionChannelInternalSegment,
			To:      ownInbox,
		},
		Source:  source,
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create self-scoped task: %v", err)
	}
	crossDelivery := automationdomain.DeliveryTarget{
		Mode:    automationdomain.DeliveryModeExplicit,
		Channel: protocol.SessionChannelInternalSegment,
		To:      otherInbox,
	}
	if _, err = service.UpdateTask(agentCtx, created.JobID, automationdomain.UpdateJobInput{
		Delivery: &crossDelivery,
		Source:   &source,
	}); err == nil || !strings.Contains(err.Error(), "another agent") {
		t.Fatalf("Agent-origin update should reject another Agent inbox, got %v", err)
	}
}

func TestServiceRejectsAgentActorWithForgedControlPlaneSource(t *testing.T) {
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		newAutomationTestDB(t),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	ownerCtx := automationMCPTestOwnerContext("user-1")
	agentCtx := automationexec.WithActorAgentID(ownerCtx, "agent-1")

	_, err := service.CreateTask(agentCtx, automationdomain.CreateJobInput{
		Name:        "forged-source",
		AgentID:     "agent-1",
		Instruction: "claim to be CLI",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(60),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetNamed,
			NamedSessionKey: "forged-source",
		},
		Delivery: automationdomain.DeliveryTarget{
			Mode:    automationdomain.DeliveryModeExplicit,
			Channel: protocol.SessionChannelFeishu,
			To:      "oc_ungranted",
		},
		Source:  automationdomain.Source{Kind: automationdomain.SourceKindCLI},
		Enabled: true,
	})
	if err == nil {
		t.Fatal("Agent actor should not bypass delivery scope with a forged CLI source")
	}
	if !strings.Contains(err.Error(), "trusted Agent source") {
		t.Fatalf("unexpected forged-source error: %v", err)
	}
}

func TestServiceOwnerMainGrantIsRevalidatedBeforeDelivery(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	authority := &mutableAutomationAgentAuthority{agents: map[string]protocol.Agent{
		"main": {
			AgentID:       "main",
			OwnerUserID:   "user-1",
			Status:        "active",
			IsMain:        true,
			WorkspacePath: t.TempDir(),
		},
		"worker": {
			AgentID:       "worker",
			OwnerUserID:   "user-1",
			Status:        "active",
			WorkspacePath: t.TempDir(),
		},
	}}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		nil,
		delivery,
	)
	service.agents = authority
	ownerCtx := automationMCPTestOwnerContext("user-1")
	mainCtx := automationexec.WithActorAgentID(ownerCtx, "main")
	mainSession := protocol.BuildAgentSessionKey(
		"main",
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"owner",
		"",
	)

	created, err := service.CreateTask(mainCtx, automationdomain.CreateJobInput{
		Name:        "main-granted-channel",
		AgentID:     "worker",
		Instruction: "send a report",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(60),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetNamed,
			NamedSessionKey: "main-granted-channel",
		},
		Delivery: automationdomain.DeliveryTarget{
			Mode:    automationdomain.DeliveryModeExplicit,
			Channel: protocol.SessionChannelFeishu,
			To:      "oc_owner_selected",
		},
		Source: automationdomain.Source{
			Kind:           automationdomain.SourceKindAgent,
			CreatorAgentID: "main",
			ContextType:    "agent",
			ContextID:      "main",
			SessionKey:     mainSession,
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("current owner-main should grant arbitrary owner-scoped delivery: %v", err)
	}

	authority.setMain("main", false)
	result := service.deliverJobObservation(
		ownerCtx,
		*created,
		"",
		automationexec.ExecutionObservation{ResultText: "sensitive report"},
	)
	if result.Status != automationdomain.DeliveryStatusFailed ||
		result.Error == nil ||
		!strings.Contains(*result.Error, "cannot grant automation delivery") {
		t.Fatalf("revoked owner-main authority should fail closed: %+v", result)
	}
	if calls := delivery.Calls(); len(calls) != 0 {
		t.Fatalf("revoked owner-main authority reached delivery router: %+v", calls)
	}
}

func TestDeliverJobObservationUsesLatestTaskAfterStaleSnapshot(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		nil,
		delivery,
	)
	ownerCtx := automationMCPTestOwnerContext("user-1")
	agentCtx := automationexec.WithActorAgentID(ownerCtx, "agent-1")
	sourceSession := protocol.BuildAgentSessionKey(
		"agent-1",
		protocol.SessionChannelInternalSegment,
		protocol.RoomTypeDM,
		"operator",
		"",
	)
	inbox := protocol.BuildAgentSessionKey(
		"agent-1",
		protocol.SessionChannelInternalSegment,
		protocol.RoomTypeDM,
		protocol.AutomationInboxSessionRef,
		"",
	)
	created, err := service.CreateTask(agentCtx, automationdomain.CreateJobInput{
		Name:        "stale-snapshot",
		AgentID:     "agent-1",
		Instruction: "send once",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(60),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetNamed,
			NamedSessionKey: "stale-snapshot",
		},
		Delivery: automationdomain.DeliveryTarget{
			Mode:    automationdomain.DeliveryModeExplicit,
			Channel: protocol.SessionChannelInternalSegment,
			To:      inbox,
		},
		Source: automationdomain.Source{
			Kind:           automationdomain.SourceKindAgent,
			CreatorAgentID: "agent-1",
			ContextType:    "agent",
			ContextID:      "agent-1",
			SessionKey:     sourceSession,
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create self-scoped task: %v", err)
	}
	staleExecutionSnapshot := *created
	none := automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone}
	if _, err = service.UpdateTask(ownerCtx, created.JobID, automationdomain.UpdateJobInput{
		Delivery: &none,
	}); err != nil {
		t.Fatalf("disable delivery while execution holds a stale snapshot: %v", err)
	}

	result := service.deliverJobObservation(
		ownerCtx,
		staleExecutionSnapshot,
		"",
		automationexec.ExecutionObservation{ResultText: "must not use stale target"},
	)
	if result.Status != automationdomain.DeliveryStatusNotRequired {
		t.Fatalf("delivery should follow latest persisted mode=none: %+v", result)
	}
	if calls := delivery.Calls(); len(calls) != 0 {
		t.Fatalf("stale execution snapshot reached old delivery target: %+v", calls)
	}
}

func TestRoomDeliveryRevalidatesCurrentMembership(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	room := &fakeRoomRunner{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		room,
		nil,
		nil,
		delivery,
	)
	ownerCtx := automationMCPTestOwnerContext("user-1")
	agentCtx := automationexec.WithActorAgentID(ownerCtx, "agent-1")
	roomSession := protocol.BuildRoomSharedSessionKey("conversation-1")
	created, err := service.CreateTask(agentCtx, automationdomain.CreateJobInput{
		Name:        "room-report",
		AgentID:     "agent-1",
		Instruction: "send to current room",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(60),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetNamed,
			NamedSessionKey: "room-report",
		},
		Delivery: automationdomain.DeliveryTarget{
			Mode:    automationdomain.DeliveryModeExplicit,
			Channel: protocol.SessionChannelWebSocket,
			To:      roomSession,
		},
		Source: automationdomain.Source{
			Kind:           automationdomain.SourceKindAgent,
			CreatorAgentID: "agent-1",
			ContextType:    "room",
			ContextID:      "room-1",
			SessionKey:     roomSession,
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("current Room member should grant delivery to that Room: %v", err)
	}

	room.contexts = map[string]*protocol.ConversationContextAggregate{
		"conversation-1": {
			Room:         protocol.RoomRecord{ID: "room-1", RoomType: protocol.RoomTypeGroup},
			Conversation: protocol.ConversationRecord{ID: "conversation-1", RoomID: "room-1"},
		},
	}
	result := service.deliverJobObservation(
		ownerCtx,
		*created,
		"",
		automationexec.ExecutionObservation{ResultText: "former member output"},
	)
	if result.Status != automationdomain.DeliveryStatusFailed ||
		result.Error == nil ||
		!strings.Contains(*result.Error, "no longer a member") {
		t.Fatalf("revoked Room membership should fail closed: %+v", result)
	}
	if calls := delivery.Calls(); len(calls) != 0 {
		t.Fatalf("revoked Room member reached delivery router: %+v", calls)
	}
}
