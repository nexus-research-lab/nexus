package automation

import (
	"context"
	"errors"
	"slices"
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

type fakeAutomationDeliverySessionResolver struct {
	sessions map[string]protocol.Session
}

func (f fakeAutomationDeliverySessionResolver) ResolveDeliverySession(
	_ context.Context,
	sessionKey string,
) (*protocol.Session, error) {
	item, ok := f.sessions[strings.TrimSpace(sessionKey)]
	if !ok {
		return nil, nil
	}
	result := item
	return &result, nil
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
	otherSession := protocol.BuildAgentSessionKey(
		"agent-2",
		protocol.SessionChannelInternalSegment,
		protocol.RoomTypeDM,
		"operator",
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
			To:      otherSession,
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

	ownSession := protocol.BuildAgentSessionKey(
		"agent-1",
		protocol.SessionChannelInternalSegment,
		protocol.RoomTypeDM,
		"operator",
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
			To:      ownSession,
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
		To:      otherSession,
	}
	if _, err = service.UpdateTask(agentCtx, created.JobID, automationdomain.UpdateJobInput{
		Delivery: &crossDelivery,
		Source:   &source,
	}); err == nil || !strings.Contains(err.Error(), "another agent") {
		t.Fatalf("Agent-origin update should reject another Agent session, got %v", err)
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
			Mode: automationdomain.DeliveryModeNone,
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

func TestServiceAllowsPageDeliveryToAnotherSameOwnerAgent(t *testing.T) {
	workspacePath := newAutomationOwnerWorkspace(t, "user-1", "agent-b")
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		newAutomationTestDB(t),
		nil, nil, nil, nil, nil, nil,
	)
	service.agents = &mutableAutomationAgentAuthority{agents: map[string]protocol.Agent{
		"agent-a": {AgentID: "agent-a", OwnerUserID: "user-1", Status: "active", WorkspacePath: workspacePath},
		"agent-b": {AgentID: "agent-b", OwnerUserID: "user-1", Status: "active", WorkspacePath: workspacePath},
	}}
	recipientSession := protocol.BuildAgentSessionKey(
		"agent-b",
		protocol.SessionChannelInternalSegment,
		protocol.RoomTypeDM,
		"recipient-session",
		"",
	)
	service.SetDeliverySessionResolver(fakeAutomationDeliverySessionResolver{sessions: map[string]protocol.Session{
		recipientSession: {
			SessionKey: recipientSession, AgentID: "agent-b", ChannelType: protocol.SessionChannelInternalSegment,
		},
	}})
	created, err := service.CreateTask(automationMCPTestOwnerContext("user-1"), automationdomain.CreateJobInput{
		Name: "A executes and B receives", AgentID: "agent-a", Instruction: "prepare report",
		Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(60), Timezone: "Asia/Shanghai"},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery: automationdomain.DeliveryTarget{
			Mode: automationdomain.DeliveryModeExplicit, Channel: protocol.SessionChannelInternalSegment,
			To: recipientSession, SessionKey: recipientSession,
		},
		Source:  automationdomain.Source{Kind: automationdomain.SourceKindUserPage},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("same-owner page delivery should be accepted: %v", err)
	}
	if created.AgentID != "agent-a" || created.Delivery.SessionKey != recipientSession ||
		created.DeliveryGrant.Kind != automationdomain.SourceKindUserPage {
		t.Fatalf("execution and recipient identity were collapsed: %+v", created)
	}
}

func TestServiceRejectsNewLegacyInboxAndMissingRealSession(t *testing.T) {
	workspacePath := newAutomationOwnerWorkspace(t, "user-1", "agent-1")
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		newAutomationTestDB(t),
		nil, nil, nil, nil, nil, nil,
	)
	service.agents = &mutableAutomationAgentAuthority{agents: map[string]protocol.Agent{
		"agent-1": {
			AgentID: "agent-1", OwnerUserID: "user-1", Status: "active",
			WorkspacePath: workspacePath,
		},
	}}
	service.SetDeliverySessionResolver(fakeAutomationDeliverySessionResolver{})
	input := automationConfigurationTaskInput("real-session-required")
	input.Source = automationdomain.Source{Kind: automationdomain.SourceKindUserPage}
	ownerCtx := automationMCPTestOwnerContext("user-1")

	inboxKey := protocol.BuildAgentSessionKey(
		"agent-1", protocol.SessionChannelInternalSegment, protocol.RoomTypeDM,
		protocol.AutomationInboxSessionRef, "",
	)
	input.Delivery = automationdomain.DeliveryTarget{
		Mode: automationdomain.DeliveryModeExplicit, Channel: protocol.SessionChannelInternalSegment,
		To: inboxKey, SessionKey: inboxKey,
	}
	if _, err := service.CreateTask(ownerCtx, input); err == nil ||
		!strings.Contains(err.Error(), "legacy-only") {
		t.Fatalf("new synthetic inbox must be rejected, got %v", err)
	}

	missingKey := protocol.BuildAgentSessionKey(
		"agent-1", protocol.SessionChannelInternalSegment, protocol.RoomTypeDM,
		"missing-real-session", "",
	)
	input.Delivery = automationdomain.DeliveryTarget{
		Mode: automationdomain.DeliveryModeExplicit, Channel: protocol.SessionChannelInternalSegment,
		To: missingKey, SessionKey: missingKey,
	}
	if _, err := service.CreateTask(ownerCtx, input); !errors.Is(
		err,
		automationdomain.ErrTaskDeliverySessionUnavailable,
	) {
		t.Fatalf("missing real session must be rejected, got %v", err)
	}
}

func TestServiceValidatesCrossAgentIMAgainstRecipientPairing(t *testing.T) {
	workspacePath := newAutomationOwnerWorkspace(t, "user-1", "agent-b")
	grant := &mutableAutomationDeliveryGrant{allowed: true}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		newAutomationTestDB(t),
		nil, nil, nil, nil, nil, nil,
	)
	service.agents = &mutableAutomationAgentAuthority{agents: map[string]protocol.Agent{
		"agent-a": {AgentID: "agent-a", OwnerUserID: "user-1", Status: "active", WorkspacePath: workspacePath},
		"agent-b": {AgentID: "agent-b", OwnerUserID: "user-1", Status: "active", WorkspacePath: workspacePath},
	}}
	service.SetDeliveryGrantResolver(grant)
	recipientSession := protocol.BuildAgentSessionKey(
		"agent-b", protocol.SessionChannelWeixinPersonal, protocol.RoomTypeDM,
		"wx-user-b", "",
	)
	service.SetDeliverySessionResolver(fakeAutomationDeliverySessionResolver{sessions: map[string]protocol.Session{
		recipientSession: {
			SessionKey: recipientSession, AgentID: "agent-b", ChannelType: protocol.SessionChannelWeixinPersonal,
		},
	}})
	_, err := service.CreateTask(automationMCPTestOwnerContext("user-1"), automationdomain.CreateJobInput{
		Name: "A executes and B receives on IM", AgentID: "agent-a", Instruction: "prepare report",
		Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(60), Timezone: "Asia/Shanghai"},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeLast, SessionKey: recipientSession},
		Source:        automationdomain.Source{Kind: automationdomain.SourceKindUserPage},
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("cross-Agent IM delivery should validate recipient pairing: %v", err)
	}
	if got := grant.agentIDsSnapshot(); !slices.Equal(got, []string{"agent-b"}) {
		t.Fatalf("pairing was checked against executor instead of recipient: %v", got)
	}
}

func TestServiceRejectsPageDeliveryToCrossOwnerAgent(t *testing.T) {
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		newAutomationTestDB(t),
		nil, nil, nil, nil, nil, nil,
	)
	service.agents = &mutableAutomationAgentAuthority{agents: map[string]protocol.Agent{
		"agent-a": {AgentID: "agent-a", OwnerUserID: "user-1", Status: "active"},
		"agent-b": {AgentID: "agent-b", OwnerUserID: "user-2", Status: "active"},
	}}
	recipientSession := protocol.BuildAgentSessionKey(
		"agent-b", protocol.SessionChannelInternalSegment, protocol.RoomTypeDM,
		"recipient-session", "",
	)
	_, err := service.CreateTask(automationMCPTestOwnerContext("user-1"), automationdomain.CreateJobInput{
		Name: "cross owner", AgentID: "agent-a", Instruction: "prepare report",
		Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(60), Timezone: "Asia/Shanghai"},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery: automationdomain.DeliveryTarget{
			Mode: automationdomain.DeliveryModeExplicit, Channel: protocol.SessionChannelInternalSegment,
			To: recipientSession, SessionKey: recipientSession,
		},
		Source:  automationdomain.Source{Kind: automationdomain.SourceKindUserPage},
		Enabled: true,
	})
	if err == nil || !strings.Contains(err.Error(), "must be owned") {
		t.Fatalf("cross-owner recipient must fail closed, got %v", err)
	}
}

func TestServiceOwnerMainGrantIsRevalidatedBeforeDelivery(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	workerWorkspace := newAutomationOwnerWorkspace(t, "user-1", "worker")
	authority := &mutableAutomationAgentAuthority{agents: map[string]protocol.Agent{
		"main": {
			AgentID:       "main",
			OwnerUserID:   "user-1",
			Status:        "active",
			IsMain:        true,
			WorkspacePath: workerWorkspace,
		},
		"worker": {
			AgentID:       "worker",
			OwnerUserID:   "user-1",
			Status:        "active",
			WorkspacePath: workerWorkspace,
		},
	}}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workerWorkspace},
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
	workerSession := protocol.BuildAgentSessionKey(
		"worker",
		protocol.SessionChannelInternalSegment,
		protocol.RoomTypeDM,
		"owner-selected",
		"",
	)
	service.SetDeliverySessionResolver(fakeAutomationDeliverySessionResolver{sessions: map[string]protocol.Session{
		workerSession: {
			SessionKey: workerSession, AgentID: "worker", ChannelType: protocol.SessionChannelInternalSegment,
		},
	}})

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
			Mode:       automationdomain.DeliveryModeExplicit,
			Channel:    protocol.SessionChannelInternalSegment,
			To:         workerSession,
			SessionKey: workerSession,
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
			To:      sourceSession,
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
