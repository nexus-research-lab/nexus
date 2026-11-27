// delivery_test.go 覆盖自动化任务投递权威、成功、重试、死信、初始可靠性与 IM 反馈测试。
package automation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
	"github.com/nexus-research-lab/nexus/internal/storage/imdelivery"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	_ "modernc.org/sqlite"
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
	ownerCtx := automationCommandTestOwnerContext("user-1")
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
	ownerCtx := automationCommandTestOwnerContext("user-1")
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
	ownerCtx := automationCommandTestOwnerContext("user-1")

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
	_, err := service.CreateTask(automationCommandTestOwnerContext("user-1"), automationdomain.CreateJobInput{
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
	ownerCtx := automationCommandTestOwnerContext("user-1")
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
	ownerCtx := automationCommandTestOwnerContext("user-1")
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
	if created.Delivery.AgentID != "agent-1" {
		t.Fatalf("blank Room reply agent should resolve to host: %+v", created.Delivery)
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

func TestRoomDeliveryPersistsAndUsesIndependentReplyAgent(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	room := &fakeRoomRunner{contexts: map[string]*protocol.ConversationContextAggregate{
		"conversation-1": {
			Room: protocol.RoomRecord{
				ID: "room-1", RoomType: protocol.RoomTypeGroup, HostAgentID: "agent-1",
			},
			Members: []protocol.MemberRecord{
				{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-1"},
				{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-2"},
			},
			Conversation: protocol.ConversationRecord{ID: "conversation-1", RoomID: "room-1"},
			Sessions: []protocol.SessionRecord{
				{ConversationID: "conversation-1", AgentID: "agent-1", Status: "active"},
				{ConversationID: "conversation-1", AgentID: "agent-2", Status: "active"},
			},
		},
	}}
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
	ownerCtx := automationCommandTestOwnerContext("user-1")
	roomSession := protocol.BuildRoomSharedSessionKey("conversation-1")
	created, err := service.CreateTask(ownerCtx, automationdomain.CreateJobInput{
		Name:        "room-report",
		AgentID:     "agent-1",
		Instruction: "send as selected room member",
		Schedule: automationdomain.Schedule{
			Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(60), Timezone: "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery: automationdomain.DeliveryTarget{
			Mode: automationdomain.DeliveryModeExplicit, Channel: protocol.SessionChannelWebSocket,
			To: roomSession, SessionKey: roomSession, AgentID: "agent-2",
		},
		Source:  automationdomain.Source{Kind: automationdomain.SourceKindUserPage},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create Room delivery with independent reply agent: %v", err)
	}
	if created.AgentID != "agent-1" || created.Delivery.AgentID != "agent-2" {
		t.Fatalf("execution and reply identities should remain independent: %+v", created)
	}
	result := service.deliverJobObservation(
		ownerCtx,
		*created,
		"",
		automationexec.ExecutionObservation{RunID: "run-1", ResultText: "room result"},
	)
	if result.Status != automationdomain.DeliveryStatusSucceeded {
		t.Fatalf("deliver Room result: %+v", result)
	}
	if got := delivery.AgentIDs(); len(got) != 1 || got[0] != "agent-2" {
		t.Fatalf("Room result projected as agent %q, want agent-2", got)
	}
}

func TestServiceAutoRetryDeliveryDeadLettersDisabledTask(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		delivery,
	)
	base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "paused-delivery",
		AgentID:     "agent-1",
		Instruction: "send report",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "UTC",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: "reports"},
		Delivery:      fakeStructuredDelivery("agent-1", "paused-delivery"),
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	if _, err = service.UpdateTaskStatus(context.Background(), task.JobID, false); err != nil {
		t.Fatalf("停用任务失败: %v", err)
	}

	runID := "run-disabled-delivery"
	dueAt := base.Add(time.Minute)
	deliveryError := "feishu temporary outage"
	if _, err = db.Exec(`
INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind,
    delivery_mode, delivery_to, delivery_status, delivery_error,
    delivery_attempts, delivery_next_attempt_at, scheduled_for, finished_at,
    result_text, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID,
		task.JobID,
		task.OwnerUserID,
		automationdomain.RunStatusSucceeded,
		automationdomain.TriggerKindScheduled, automationdomain.DeliveryModeExplicit,
		"explicit:feishu:oc_group",
		automationdomain.DeliveryStatusFailed,
		deliveryError,
		1,
		dueAt,
		dueAt.Add(-time.Minute),
		dueAt,
		"日报正文",
		1,
	); err != nil {
		t.Fatalf("准备 disabled due delivery run 失败: %v", err)
	}

	service.nowFn = func() time.Time { return dueAt.Add(time.Second) }
	service.retryDueDeliveries(context.Background(), dueAt.Add(time.Second))

	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("读取自动重试跳过后的 run 失败: runs=%+v err=%v", runs, err)
	}
	updated := runs[0]
	if updated.RunID != runID ||
		updated.DeliveryStatus != automationdomain.DeliveryStatusFailed ||
		updated.DeliveryDeadLetterAt == nil ||
		updated.DeliveryNextAttemptAt != nil {
		t.Fatalf("停用任务的 due delivery 应进入死信并清理下一次重试: %+v", updated)
	}
	if updated.DeliveryAttempts != 1 {
		t.Fatalf("停用任务不应发生新的投递尝试，attempts=%d", updated.DeliveryAttempts)
	}
	if len(delivery.Calls()) != 0 {
		t.Fatalf("停用任务不应继续自动投递，calls=%+v", delivery.Calls())
	}
	dueRuns, err := service.repository.ListDueDeliveryRetries(context.Background(), dueAt.Add(2*time.Second), maxAutoDeliveryAttempts, deliveryRetryBatchLimit)
	if err != nil {
		t.Fatalf("重新读取 due delivery 失败: %v", err)
	}
	for _, dueRun := range dueRuns {
		if dueRun.RunID == runID {
			t.Fatalf("死信后的 disabled delivery 不应再次进入自动重试队列: %+v", dueRuns)
		}
	}

	events, err := service.ListTaskEvents(context.Background(), task.JobID, 20)
	if err != nil {
		t.Fatalf("读取自动重试跳过事件失败: %v", err)
	}
	for _, event := range events {
		if event.Action == automationdomain.TaskEventActionAutoRetryDelivery &&
			event.RunID == runID &&
			event.Detail["auto_retry_skipped_reason"] == "task_disabled" {
			return
		}
	}
	t.Fatalf("停用任务的自动重试跳过应写入审计事件: %+v", events)
}

func TestDeleteTaskDeadLettersPendingDeliveryRetries(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		delivery,
	)
	base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	service.nowFn = func() time.Time { return base }
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "delete-with-failed-delivery",
		AgentID:     "agent-1",
		Instruction: "send report",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "UTC",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: "reports"},
		Delivery:      fakeStructuredDelivery("agent-1", "delete-with-failed-delivery"),
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	runID := "run-delete-delivery"
	nextAttemptAt := base.Add(10 * time.Minute)
	deliveryError := "feishu temporary outage"
	if _, err = db.Exec(`
INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind,
    delivery_mode, delivery_to, delivery_status, delivery_error,
    delivery_attempts, delivery_next_attempt_at, scheduled_for, finished_at,
    result_text, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID,
		task.JobID,
		task.OwnerUserID,
		automationdomain.RunStatusSucceeded,
		automationdomain.TriggerKindScheduled, automationdomain.DeliveryModeExplicit,
		"explicit:feishu:oc_group",
		automationdomain.DeliveryStatusFailed,
		deliveryError,
		1,
		nextAttemptAt,
		base,
		base.Add(time.Minute),
		"日报正文",
		1,
	); err != nil {
		t.Fatalf("准备待补投递 run 失败: %v", err)
	}

	deletedAt := base.Add(time.Minute)
	service.nowFn = func() time.Time { return deletedAt }
	if _, err = service.DeleteTask(context.Background(), task.JobID); err != nil {
		t.Fatalf("DeleteTask 失败: %v", err)
	}

	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("删除后读取 run ledger 失败: runs=%+v err=%v", runs, err)
	}
	updated := runs[0]
	if updated.RunID != runID ||
		updated.DeliveryStatus != automationdomain.DeliveryStatusFailed ||
		updated.DeliveryDeadLetterAt == nil ||
		updated.DeliveryNextAttemptAt != nil ||
		updated.DeliveryError == nil ||
		!strings.Contains(*updated.DeliveryError, "deleted") {
		t.Fatalf("删除任务应立即把待补投递 run 标记为死信: %+v", updated)
	}
	if updated.DeliveryAttempts != 1 {
		t.Fatalf("删除任务不应新增投递尝试，attempts=%d", updated.DeliveryAttempts)
	}
	dueRuns, err := service.repository.ListDueDeliveryRetries(context.Background(), deletedAt.Add(time.Hour), maxAutoDeliveryAttempts, deliveryRetryBatchLimit)
	if err != nil {
		t.Fatalf("读取 due delivery 失败: %v", err)
	}
	for _, dueRun := range dueRuns {
		if dueRun.RunID == runID {
			t.Fatalf("删除任务后的死信 run 不应进入自动重试队列: %+v", dueRuns)
		}
	}
	if len(delivery.Calls()) != 0 {
		t.Fatalf("删除任务不应触发投递，calls=%+v", delivery.Calls())
	}
	report, err := service.GetDailyReport(context.Background(), automationdomain.ScheduledTaskDailyReportInput{
		Date:     "2026-05-21",
		Timezone: "UTC",
		JobID:    task.JobID,
	})
	if err != nil {
		t.Fatalf("删除任务后读取日报失败: %v", err)
	}
	if len(report.Tasks) != 1 {
		t.Fatalf("删除任务日报应返回一条任务明细: %+v", report)
	}
	dailyTask := report.Tasks[0]
	if !dailyTask.Deleted ||
		!containsString(dailyTask.Signals, "deleted") ||
		!containsString(dailyTask.Signals, "delivery_attention") ||
		!containsString(dailyTask.DeliveryDeadLetterRunIDs, runID) ||
		containsString(dailyTask.ManualRedeliveryRunIDs, runID) ||
		!containsString(dailyTask.SuggestedTools, runtimeAutomationInspectSuggestion) ||
		containsString(dailyTask.SuggestedTools, runtimeAutomationApplySuggestion) {
		t.Fatalf("删除任务日报应保留失败信号但不建议不可执行补投递: %+v", dailyTask)
	}

	events, err := service.ListTaskEvents(context.Background(), task.JobID, 20)
	if err != nil {
		t.Fatalf("删除后读取事件失败: %v", err)
	}
	for _, event := range events {
		if event.Action != automationdomain.TaskEventActionDelete {
			continue
		}
		values, ok := event.Detail["dead_lettered_delivery_run_ids"].([]any)
		if !ok || len(values) != 1 || values[0] != runID {
			t.Fatalf("delete 事件应记录被死信的投递 run: %+v", event.Detail)
		}
		return
	}
	t.Fatalf("删除任务应写入 delete 事件: %+v", events)
}

func TestServiceRetryRunDeliveryMarksDeadLetterAfterMaxAttempts(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		nil,
	)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "dead-letter",
		AgentID:     "agent-1",
		Instruction: "send report",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: "reports"},
		Delivery:      fakeStructuredDelivery("agent-1", "dead-letter"),
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	if _, err = db.Exec(`
INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind,
    delivery_mode, delivery_to, delivery_status, delivery_attempts,
    result_text, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"run-dead",
		task.JobID,
		task.OwnerUserID,
		automationdomain.RunStatusSucceeded,
		automationdomain.TriggerKindScheduled, automationdomain.DeliveryModeExplicit,
		"feishu:oc_group",
		automationdomain.DeliveryStatusFailed,
		maxAutoDeliveryAttempts-1,
		"日报正文",
		1,
	); err != nil {
		t.Fatalf("准备 run 失败: %v", err)
	}

	run, err := service.RetryRunDelivery(context.Background(), task.JobID, "run-dead")
	if err != nil {
		t.Fatalf("RetryRunDelivery 失败: %v", err)
	}
	if run.DeliveryStatus != automationdomain.DeliveryStatusFailed || run.DeliveryDeadLetterAt == nil || run.DeliveryNextAttemptAt != nil {
		t.Fatalf("达到最大重试后应进入死信且不再安排自动重试: %+v", run)
	}
	if run.DeliveryAttempts != maxAutoDeliveryAttempts {
		t.Fatalf("delivery_attempts = %d, 期望 %d", run.DeliveryAttempts, maxAutoDeliveryAttempts)
	}
}

func TestServiceRetryRunDeliveryRejectsAlreadyDeliveredRun(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		delivery,
	)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "delivered",
		AgentID:     "agent-1",
		Instruction: "send report",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: "reports"},
		Delivery:      fakeStructuredDelivery("agent-1", "already-delivered"),
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	if _, err = db.Exec(`
INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind,
    delivery_mode, delivery_to, delivery_status, delivery_attempts,
    result_text, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"run-delivered",
		task.JobID,
		task.OwnerUserID,
		automationdomain.RunStatusSucceeded,
		automationdomain.TriggerKindScheduled, automationdomain.DeliveryModeExplicit,
		"feishu:oc_group",
		automationdomain.DeliveryStatusSucceeded,
		1,
		"日报正文",
		1,
	); err != nil {
		t.Fatalf("准备 run 失败: %v", err)
	}

	_, err = service.RetryRunDelivery(context.Background(), task.JobID, "run-delivered")
	if err == nil || !strings.Contains(err.Error(), "delivery_status must be failed") {
		t.Fatalf("期望拒绝重复补投已成功 run，实际 err=%v", err)
	}
	if calls := delivery.Calls(); len(calls) != 0 {
		t.Fatalf("不应重复调用投递，calls=%+v", calls)
	}
	var attempts int
	if err = db.QueryRow(`SELECT delivery_attempts FROM automation_task_runs WHERE run_id = ?`, "run-delivered").Scan(&attempts); err != nil {
		t.Fatalf("读取 delivery_attempts 失败: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("delivery_attempts 不应变化，实际 %d", attempts)
	}
}

func TestDeliveryRetryExactClaimAllowsOnlyOneAttemptAndLeavesUnknownOutOfAutoRetry(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, &fakeWorkspaceReader{}, delivery)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "exact-delivery-claim",
		AgentID:     "agent-1",
		Instruction: "deliver once",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "UTC",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: "claim"},
		Delivery:      fakeStructuredDelivery("agent-1", "claim"),
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	runID := "run-exact-delivery-claim"
	if _, err = db.Exec(`INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind, delivery_status,
    delivery_attempts, result_text, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID, task.JobID, task.OwnerUserID, automationdomain.RunStatusSucceeded,
		automationdomain.TriggerKindManual, automationdomain.DeliveryStatusFailed, 1, "result", 1,
	); err != nil {
		t.Fatalf("准备 failed delivery 失败: %v", err)
	}

	type claimResult struct {
		attemptID string
		err       error
	}
	results := make(chan claimResult, 2)
	var start sync.WaitGroup
	start.Add(1)
	for _, attemptID := range []string{"attempt-a", "attempt-b"} {
		attemptID := attemptID
		go func() {
			start.Wait()
			claimErr := service.repository.ClaimRunDeliveryAttempt(context.Background(), automationstore.RunDeliveryAttemptClaimInput{
				OwnerUserID:                  task.OwnerUserID,
				JobID:                        task.JobID,
				RunID:                        runID,
				ExpectedDeliveryAttempts:     1,
				ExpectedConfigurationVersion: &task.ConfigurationVersion,
				ExpectedStatus:               automationdomain.DeliveryStatusFailed,
				AttemptID:                    attemptID,
			})
			results <- claimResult{attemptID: attemptID, err: claimErr}
		}()
	}
	start.Done()
	first := <-results
	second := <-results
	var winner string
	for _, result := range []claimResult{first, second} {
		if result.err == nil {
			if winner != "" {
				t.Fatalf("并发 claim 出现多个赢家: first=%v second=%v", first.err, second.err)
			}
			winner = result.attemptID
			continue
		}
		if !errors.Is(result.err, automationdomain.ErrDeliveryRetryConflict) {
			t.Fatalf("claim loser error = %v", result.err)
		}
	}
	if winner == "" {
		t.Fatalf("并发 claim 没有赢家: first=%v second=%v", first.err, second.err)
	}

	claimed, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
	if err != nil {
		t.Fatalf("读取 claimed run 失败: %v", err)
	}
	if claimed.DeliveryStatus != automationdomain.DeliveryStatusRetrying || claimed.DeliveryAttempts != 2 {
		t.Fatalf("claim 未原子推进状态和 attempts: %+v", claimed)
	}
	if err = service.repository.DeadLetterFailedRunDelivery(
		context.Background(), task.OwnerUserID, task.JobID, runID, 1, nil, time.Now(),
	); !errors.Is(err, automationdomain.ErrDeliveryRetryConflict) {
		t.Fatalf("过期自动 worker 不得覆盖 retrying claim: %v", err)
	}
	due, err := service.repository.ListDueDeliveryRetries(context.Background(), time.Now().Add(24*time.Hour), maxAutoDeliveryAttempts, 20)
	if err != nil || len(due) != 0 {
		t.Fatalf("retrying 不得进入自动重试: due=%+v err=%v", due, err)
	}
	if err = validateDeliveryRetry(*claimed); !errors.Is(err, automationdomain.ErrDeliveryRetryUnverified) {
		t.Fatalf("普通人工重试必须要求先核对未知结果: %v", err)
	}
	if _, err = service.RetryRunDelivery(context.Background(), task.JobID, runID); !errors.Is(err, automationdomain.ErrDeliveryRetryUnverified) {
		t.Fatalf("普通 retry API 不得重放未知外投: %v", err)
	}
	if calls := delivery.Calls(); len(calls) != 0 {
		t.Fatalf("普通 retry API 不得产生外投: %+v", calls)
	}

	if err = service.repository.CompleteRunDeliveryAttempt(context.Background(), automationstore.RunDeliveryAttemptCompletionInput{
		OwnerUserID:    task.OwnerUserID,
		JobID:          task.JobID,
		RunID:          runID,
		AttemptID:      "loser-attempt",
		DeliveryStatus: automationdomain.DeliveryStatusSucceeded,
	}); !errors.Is(err, automationdomain.ErrDeliveryRetryCompletionUnconfirmed) {
		t.Fatalf("非持有者不得完成 attempt: %v", err)
	}
	confirmed, err := service.RetryUnverifiedRunDeliveryAtVersion(
		context.Background(), task.JobID, runID, task.ConfigurationVersion, 2,
	)
	if err != nil {
		t.Fatalf("用户核对后的 exact retry 失败: %v", err)
	}
	if confirmed.DeliveryStatus != automationdomain.DeliveryStatusSucceeded || confirmed.DeliveryAttempts != 3 {
		t.Fatalf("显式核对后的投递状态不正确: %+v", confirmed)
	}
	if calls := delivery.Calls(); len(calls) != 1 {
		t.Fatalf("显式核对后必须且只能产生一次外投: %+v", calls)
	}
}

func TestVersionedDeliveryRetryRejectsStaleExpectedAttemptsBeforeExternalCall(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, &fakeWorkspaceReader{}, delivery)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name: "stale delivery attempts", AgentID: "agent-1", Instruction: "deliver",
		Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC"},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: "attempt-fence"},
		Delivery:      fakeStructuredDelivery("agent-1", "attempt-fence"), Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask(): %v", err)
	}
	runID := "run-stale-delivery-attempts"
	if _, err = db.Exec(`INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind, delivery_status,
    delivery_attempts, result_text, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID, task.JobID, task.OwnerUserID, automationdomain.RunStatusSucceeded,
		automationdomain.TriggerKindManual, automationdomain.DeliveryStatusFailed, 2, "result", 1,
	); err != nil {
		t.Fatalf("prepare failed delivery: %v", err)
	}

	if _, err = service.RetryRunDeliveryAtVersionAndAttempts(
		context.Background(), task.JobID, runID, task.ConfigurationVersion, 1,
	); !errors.Is(err, automationdomain.ErrDeliveryRetryConflict) {
		t.Fatalf("stale delivery attempts must conflict: %v", err)
	}
	if calls := delivery.Calls(); len(calls) != 0 {
		t.Fatalf("stale delivery attempts must fail before external call: %+v", calls)
	}
	if _, err = db.Exec(`INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind, delivery_status,
    delivery_attempts, result_text, attempts, finished_at
) VALUES ('run-current-delivery-authority', ?, ?, ?, ?, ?, 0, 'newer result', 1, ?)`,
		task.JobID, task.OwnerUserID, automationdomain.RunStatusSucceeded,
		automationdomain.TriggerKindManual, automationdomain.DeliveryStatusPending, time.Now().UTC(),
	); err != nil {
		t.Fatalf("prepare latest completed run: %v", err)
	}
	if _, err = db.Exec(`UPDATE automation_scheduled_tasks
SET last_completed_run_id = 'run-current-delivery-authority', last_delivery_status = 'pending'
WHERE job_id = ?`, task.JobID); err != nil {
		t.Fatalf("bind latest completed authority: %v", err)
	}
	if _, err = service.RetryRunDeliveryAtVersionAndAttempts(
		context.Background(), task.JobID, runID, task.ConfigurationVersion, 2,
	); err != nil {
		t.Fatalf("retry historical failed run: %v", err)
	}
	updatedTask, err := service.GetTask(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("load task after historical retry: %v", err)
	}
	if updatedTask.LastDeliveryStatus != automationdomain.DeliveryStatusPending {
		t.Fatalf("historical service retry overwrote latest task summary: %+v", updatedTask)
	}
	if calls := delivery.Calls(); len(calls) != 1 {
		t.Fatalf("historical retry should update its own run exactly once: %+v", calls)
	}
}

func TestServiceRevalidatesIMPairingAtCreateDeliveryAndRetry(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	delivery := &fakeDeliveryRouter{err: fmt.Errorf("temporary IM send failure")}
	grant := &mutableAutomationDeliveryGrant{allowed: true}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		&fakeDMRunner{permission: permission, resultText: "定时结果"},
		nil,
		permission,
		&fakeWorkspaceReader{},
		delivery,
	)
	service.SetDeliveryGrantResolver(grant)
	ownerCtx := contextForOwner(context.Background(), "user-1")
	sessionKey := protocol.BuildAgentAccountSessionKey(
		"agent-1",
		protocol.SessionChannelWeixinPersonal,
		"dm",
		"weixin-account",
		"weixin-user",
		"",
	)
	create := func(name string) (*automationdomain.ScheduledTask, error) {
		return service.CreateTask(ownerCtx, automationdomain.CreateJobInput{
			Name:        name,
			AgentID:     "agent-1",
			Instruction: "执行后回传",
			Schedule: automationdomain.Schedule{
				Kind:            automationdomain.ScheduleKindEvery,
				IntervalSeconds: intRef(3600),
				Timezone:        "Asia/Shanghai",
			},
			SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: name},
			Delivery: automationdomain.DeliveryTarget{
				Mode:       automationdomain.DeliveryModeLast,
				SessionKey: sessionKey,
			},
			Source:  automationdomain.Source{Kind: automationdomain.SourceKindCLI, SessionKey: sessionKey},
			Enabled: true,
		})
	}
	task, err := create("pairing-runtime")
	if err != nil {
		t.Fatalf("active pairing 下创建任务失败: %v", err)
	}
	service.delivery = nil
	if _, err = service.RunTaskNow(ownerCtx, task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	var run automationdomain.ScheduledTaskRun
	waitFor(t, 2*time.Second, func() bool {
		runs, listErr := service.ListTaskRuns(ownerCtx, task.JobID)
		if listErr != nil || len(runs) != 1 || runs[0].DeliveryStatus != automationdomain.DeliveryStatusFailed {
			return false
		}
		run = runs[0]
		return true
	})
	if calls := delivery.Calls(); len(calls) != 0 {
		t.Fatalf("缺少 router 的确定失败不得调用 IM adapter: %+v", calls)
	}

	service.delivery = delivery
	grant.setAllowed(false)
	if _, err = create("pairing-create-revoked"); !errors.Is(err, automationdomain.ErrTaskDeliverySessionUnavailable) {
		t.Fatalf("pairing 撤销后新建任务必须 fail closed: %v", err)
	}
	delivery.err = nil
	retried, err := service.RetryRunDelivery(ownerCtx, task.JobID, run.RunID)
	if err != nil {
		t.Fatalf("重试 ledger 更新失败: %v", err)
	}
	if retried.DeliveryStatus != automationdomain.DeliveryStatusFailed ||
		retried.DeliveryError == nil ||
		!strings.Contains(*retried.DeliveryError, automationdomain.ErrTaskDeliverySessionUnavailable.Error()) {
		t.Fatalf("pairing 撤销后重试必须记录授权失败: %+v", retried)
	}
	if calls := delivery.Calls(); len(calls) != 0 {
		t.Fatalf("撤销 pairing 后重试不得到达 IM adapter: %+v", calls)
	}
}

func TestServiceDeliveryRouterErrorDoesNotFailExecutionAndRequiresVerification(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	delivery := &fakeDeliveryRouter{err: fmt.Errorf("feishu send message failed: bad chat_id")}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		&fakeDMRunner{permission: permission, resultText: "今日新闻摘要"},
		nil,
		permission,
		&fakeWorkspaceReader{},
		delivery,
	)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "news",
		AgentID:     "agent-1",
		Instruction: "search news",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: "news"},
		Delivery:      fakeStructuredDelivery("agent-1", "bad-delivery-session"),
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	if _, err = service.RunTaskNow(context.Background(), task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		current, taskErr := service.GetTask(context.Background(), task.JobID)
		return listErr == nil && taskErr == nil && len(items) > 0 &&
			items[0].DeliveryStatus == automationdomain.DeliveryStatusRetrying &&
			current.LastRunStatus == automationdomain.RunStatusSucceeded && !current.Running
	})

	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("读取 run 失败: runs=%+v err=%v", runs, err)
	}
	if runs[0].Status != automationdomain.RunStatusSucceeded {
		t.Fatalf("投递失败不应把执行状态改成 failed: %+v", runs[0])
	}
	if runs[0].DeliveryError == nil || !strings.Contains(*runs[0].DeliveryError, "bad chat_id") {
		t.Fatalf("delivery_error 未记录失败原因: %+v", runs[0])
	}
	if runs[0].DeliveryAttempts != 1 {
		t.Fatalf("delivery_attempts = %d, 期望 1", runs[0].DeliveryAttempts)
	}
	if runs[0].DeliveryNextAttemptAt != nil || runs[0].DeliveryDeadLetterAt != nil {
		t.Fatalf("结果未知时不得安排自动重试或伪造死信: %+v", runs[0])
	}
	deadline, err := service.repository.NextDeliveryRetryAt(
		context.Background(),
		maxAutoDeliveryAttempts,
	)
	if err != nil || deadline != nil {
		t.Fatalf("unverified delivery 不得进入自动 deadline: %v, err=%v", deadline, err)
	}
	updatedTask, err := service.GetTask(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("读取任务失败: %v", err)
	}
	if updatedTask.LastRunStatus != automationdomain.RunStatusSucceeded || updatedTask.FailureStreak != 0 {
		t.Fatalf("投递失败不应触发任务级执行失败退避: %+v", updatedTask)
	}
	if updatedTask.LastDeliveryStatus != automationdomain.DeliveryStatusRetrying {
		t.Fatalf("last_delivery_status 未记录需要人工核对: %+v", updatedTask)
	}
	delivery.err = nil
	service.retryDueDeliveries(context.Background(), time.Now().Add(24*time.Hour))
	if _, err = service.RetryRunDelivery(context.Background(), task.JobID, runs[0].RunID); !errors.Is(err, automationdomain.ErrDeliveryRetryUnverified) {
		t.Fatalf("普通人工 retry 也必须先核对接收端: %v", err)
	}
	if calls := delivery.Calls(); len(calls) != 1 {
		t.Fatalf("未知结果后不得产生第二次外投: %+v", calls)
	}
}

func TestVersionedDeliveryRetryKeepsTheRouteFromItsCheckedTaskSnapshot(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{err: fmt.Errorf("temporary delivery failure")}
	permission := permissionctx.NewContext()
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		&fakeDMRunner{permission: permission, resultText: "需要补投递的结果"},
		nil,
		permission,
		&fakeWorkspaceReader{},
		delivery,
	)
	originalDelivery := fakeStructuredDelivery("agent-1", "versioned-route-original")
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "版本化补投递",
		AgentID:     "agent-1",
		Instruction: "生成结果",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: "versioned-route"},
		Delivery:      originalDelivery,
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	service.delivery = nil
	if _, err = service.RunTaskNow(context.Background(), task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	var failedRun automationdomain.ScheduledTaskRun
	waitFor(t, 2*time.Second, func() bool {
		runs, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		if listErr != nil || len(runs) != 1 || runs[0].DeliveryStatus != automationdomain.DeliveryStatusFailed {
			return false
		}
		failedRun = runs[0]
		return true
	})
	checkedSnapshot, err := service.GetTask(context.Background(), task.JobID)
	if err != nil || checkedSnapshot == nil {
		t.Fatalf("读取已核对任务快照失败: task=%+v err=%v", checkedSnapshot, err)
	}
	service.delivery = delivery
	newDelivery := fakeStructuredDelivery("agent-1", "versioned-route-newer")
	if _, err = service.UpdateTask(
		context.Background(),
		task.JobID,
		automationdomain.UpdateJobInput{Delivery: &newDelivery},
	); err != nil {
		t.Fatalf("并发更新投递目标失败: %v", err)
	}
	if _, err = service.RetryRunDeliveryAtVersion(
		context.Background(),
		task.JobID,
		failedRun.RunID,
		checkedSnapshot.ConfigurationVersion,
	); !errors.Is(err, automationdomain.ErrConfigurationVersionConflict) {
		t.Fatalf("旧版本重试必须在投递前失败: %v", err)
	}
	if calls := delivery.Calls(); len(calls) != 0 {
		t.Fatalf("版本冲突不应触发额外投递: %+v", calls)
	}
	delivery.mu.Lock()
	delivery.err = nil
	delivery.mu.Unlock()
	if _, err = service.retryLoadedRunDelivery(
		context.Background(),
		task.OwnerUserID,
		*checkedSnapshot,
		failedRun,
		true,
	); !errors.Is(err, automationdomain.ErrDeliveryRetryConflict) {
		t.Fatalf("过期服务快照必须被 DB 配置 fence 拒绝: %v", err)
	}
	calls := delivery.Calls()
	if len(calls) != 0 {
		t.Fatalf("DB 配置 fence 失败时不得产生外投副作用: calls=%+v", calls)
	}
}

func TestServiceRunDueOnceRetriesDueDelivery(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		delivery,
	)
	base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	service.nowFn = func() time.Time { return base }
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "auto-redelivery",
		AgentID:     "agent-1",
		Instruction: "send report",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "UTC",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: "reports"},
		Delivery:      fakeStructuredDelivery("agent-1", "auto-redelivery"),
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	runID := "run-due-delivery"
	dueAt := base.Add(5 * time.Minute)
	scheduledFor := base.Add(-time.Minute)
	deliveryError := "feishu temporary outage"
	if _, err = db.Exec(`
INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind,
    delivery_mode, delivery_to, delivery_status, delivery_error,
    delivery_attempts, delivery_next_attempt_at, scheduled_for, finished_at,
    result_text, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID,
		task.JobID,
		task.OwnerUserID,
		automationdomain.RunStatusSucceeded,
		automationdomain.TriggerKindScheduled, automationdomain.DeliveryModeExplicit,
		"explicit:feishu:oc_old",
		automationdomain.DeliveryStatusFailed,
		deliveryError,
		1,
		dueAt,
		scheduledFor,
		scheduledFor.Add(time.Minute),
		"日报正文",
		1,
	); err != nil {
		t.Fatalf("准备 due delivery run 失败: %v", err)
	}

	service.nowFn = func() time.Time { return dueAt.Add(time.Second) }
	service.runDueOnce()

	waitFor(t, 2*time.Second, func() bool {
		runs, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil &&
			len(runs) > 0 &&
			runs[0].RunID == runID &&
			runs[0].DeliveryStatus == automationdomain.DeliveryStatusSucceeded
	})
	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("读取调度自动重试后的 run 失败: runs=%+v err=%v", runs, err)
	}
	redelivered := runs[0]
	if redelivered.DeliveryAttempts != 2 || redelivered.DeliveryError != nil || redelivered.DeliveryNextAttemptAt != nil {
		t.Fatalf("调度 tick 自动重试后状态不正确: %+v", redelivered)
	}
	if redelivered.DeliveryTo != "explicit:internal:"+task.Delivery.SessionKey {
		t.Fatalf("自动重试应使用任务当前投递目标，实际 delivery_to=%q", redelivered.DeliveryTo)
	}
	calls := delivery.Calls()
	if len(calls) != 1 || calls[0].To != task.Delivery.SessionKey {
		t.Fatalf("调度 tick 应自动投递到当前目标，calls=%+v", calls)
	}
	events, err := service.ListTaskEvents(context.Background(), task.JobID, 20)
	if err != nil {
		t.Fatalf("读取自动重试事件失败: %v", err)
	}
	for _, event := range events {
		if event.Action == automationdomain.TaskEventActionAutoRetryDelivery && event.RunID == runID {
			return
		}
	}
	t.Fatalf("调度 tick 自动重试应写入审计事件: %+v", events)
}

func TestServiceRunTaskNowDeliversToRememberedWebSocketRoute(t *testing.T) {
	workspacePath := newAutomationOwnerWorkspace(t, authctx.SystemUserID, "agent-1")
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{
		permission: permission,
		resultText: "巡检完成：CPU 使用率正常",
	}
	router := channels.NewRouter(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		&testAgentResolver{workspacePath: workspacePath},
		permission,
	)
	store := workspacestore.NewSessionFileStore(workspacePath)
	sessionKey := protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "delivery", "")
	now := time.Now().UTC()
	if _, err := store.UpsertSession(workspacePath, protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      "agent-1",
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "Delivery",
		Options:      map[string]any{},
		IsActive:     true,
	}); err != nil {
		t.Fatalf("准备目标会话失败: %v", err)
	}
	if err := router.RememberWebSocketRoute(context.Background(), sessionKey); err != nil {
		t.Fatalf("RememberWebSocketRoute 失败: %v", err)
	}

	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{},
		router,
	)

	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "主动巡检播报",
		AgentID:     "agent-1",
		Instruction: "执行巡检并输出结果",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetNamed,
			NamedSessionKey: "ops-bot",
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeLast},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	if _, err = service.RunTaskNow(context.Background(), task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		if listErr != nil || len(items) == 0 {
			return false
		}
		return items[0].Status == automationdomain.RunStatusSucceeded &&
			items[0].DeliveryStatus == automationdomain.DeliveryStatusSucceeded
	})

	sessionValue, _, err := store.FindSession([]string{workspacePath}, sessionKey)
	if err != nil {
		t.Fatalf("读取投递目标 session 失败: %v", err)
	}
	if sessionValue == nil {
		t.Fatalf("投递目标 session 不存在")
	}
	assertDeliveredAgentMessage(t, workspacePath, *sessionValue, "巡检完成：CPU 使用率正常", "投递目标")
	updatedTask, err := service.GetTask(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("读取任务运行态失败: %v", err)
	}
	if updatedTask == nil || updatedTask.LastDeliveryStatus != automationdomain.DeliveryStatusSucceeded {
		t.Fatalf("last_delivery_status 未记录投递成功: %+v", updatedTask)
	}
	deliveredRun := assertRunDeliveredTo(t, service, task.JobID, "explicit:websocket:"+sessionKey)
	if deliveredRun.ArtifactPath == nil || strings.TrimSpace(*deliveredRun.ArtifactPath) == "" {
		t.Fatalf("run 应记录产物路径: %+v", deliveredRun)
	}
	artifact, err := os.ReadFile(filepath.Join(workspacePath, filepath.FromSlash(*deliveredRun.ArtifactPath)))
	if err != nil {
		t.Fatalf("读取 run artifact 失败: %v", err)
	}
	if !strings.Contains(string(artifact), "Delivery Status At Completion: pending") ||
		!strings.Contains(string(artifact), "Frozen Delivery Target: last") {
		t.Fatalf("run artifact 应记录外投前的 durable pending 与冻结目标，最终结果以 run ledger 为准: %s", string(artifact))
	}
}

func TestServiceRunTaskNowDeliversLegacyAgentAutomationInbox(t *testing.T) {
	workspacePath := newAutomationOwnerWorkspace(t, authctx.SystemUserID, "agent-1")
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{
		permission: permission,
		resultText: "今日新闻摘要",
	}
	router := channels.NewRouter(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		&testAgentResolver{workspacePath: workspacePath},
		permission,
	)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{},
		router,
	)

	inboxKey := protocol.BuildAgentSessionKey(
		"agent-1",
		protocol.SessionChannelInternalSegment,
		"dm",
		protocol.AutomationInboxSessionRef,
		"",
	)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "新闻投递到智能体",
		AgentID:     "agent-1",
		Instruction: "搜索新闻并输出摘要",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetNamed,
			NamedSessionKey: "news",
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	legacyTask := *task
	legacyTask.Delivery = automationdomain.DeliveryTarget{
		Mode:       automationdomain.DeliveryModeExplicit,
		Channel:    protocol.SessionChannelInternalSegment,
		To:         inboxKey,
		SessionKey: inboxKey,
	}
	legacyTask.DeliveryGrant = automationdomain.Source{Kind: automationdomain.SourceKindUserPage}.Normalized()
	task, err = service.repository.UpsertScheduledTask(context.Background(), legacyTask)
	if err != nil {
		t.Fatalf("准备历史收件箱任务失败: %v", err)
	}

	if _, err = service.RunTaskNow(context.Background(), task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		if listErr != nil || len(items) == 0 {
			return false
		}
		return items[0].DeliveryStatus == automationdomain.DeliveryStatusSucceeded
	})

	store := workspacestore.NewSessionFileStore(workspacePath)
	sessionValue, _, err := store.FindSession([]string{workspacePath}, inboxKey)
	if err != nil {
		t.Fatalf("读取智能体收件箱 session 失败: %v", err)
	}
	if sessionValue == nil {
		t.Fatal("投递到智能体时应自动创建定时任务收件箱")
	}
	if sessionValue.AgentID != "agent-1" {
		t.Fatalf("收件箱应归属目标智能体，实际 %+v", sessionValue)
	}
	if sessionValue.Title != "定时任务收件箱" || sessionValue.ChannelType != protocol.SessionChannelInternalSegment {
		t.Fatalf("收件箱元数据不正确: %+v", sessionValue)
	}

	assertDeliveredAgentMessage(t, workspacePath, *sessionValue, "今日新闻摘要", "智能体收件箱")
	assertRunDeliveredTo(t, service, task.JobID, "explicit:internal:"+inboxKey)
}

func TestAutomationCommandCreateRunAndInspectDeliversToCurrentSession(t *testing.T) {
	workspacePath := newAutomationOwnerWorkspace(t, "user-1", "agent-1")
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{
		permission: permission,
		resultText: "今日新闻摘要",
	}
	router := channels.NewRouter(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		&testAgentResolver{workspacePath: workspacePath},
		permission,
	)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{},
		router,
	)
	sctx := command.Actor{
		AgentID:            "agent-1",
		AgentName:          "新闻智能体",
		OwnerUserID:        "user-1",
		SessionKey:         protocol.BuildAgentSessionKey("agent-1", protocol.SessionChannelInternalSegment, "dm", "operator", ""),
		SessionLabel:       "用户对话",
		SourceContextType:  "agent",
		SourceContextID:    "agent-1",
		SourceContextLabel: "新闻智能体",
		DefaultTimezone:    "Asia/Shanghai",
	}
	recipientSessionKey := sctx.SessionKey
	prepareAutomationDeliverySession(t, workspacePath, "user-1", "agent-1", recipientSessionKey)

	createResult, isError := callAutomationCommand(t, service, sctx, "create_scheduled_task", map[string]any{
		"name":                       "新闻投递到智能体",
		"instruction":                "每天搜索新闻并输出摘要",
		"execution_mode":             "dedicated",
		"named_session_key":          "news-search",
		"reply_mode":                 "selected",
		"selected_reply_session_key": recipientSessionKey,
		"schedule": map[string]any{
			"kind":       "daily",
			"daily_time": "09:00",
			"timezone":   "Asia/Shanghai",
		},
	})
	if isError {
		t.Fatalf("create_scheduled_task 不应失败: %s", automationCommandText(t, createResult))
	}
	created := decodeAutomationCommandJSON[automationdomain.ScheduledTask](t, createResult)
	if created.AgentID != "agent-1" {
		t.Fatalf("CLI 创建任务应归属调用智能体，实际 %+v", created)
	}
	if created.Delivery.Mode != automationdomain.DeliveryModeExplicit ||
		created.Delivery.Channel != protocol.SessionChannelInternalSegment ||
		created.Delivery.To != recipientSessionKey {
		t.Fatalf("CLI selected 应绑定真实当前会话，实际 %+v", created.Delivery)
	}
	if created.Source.Kind != automationdomain.SourceKindAgent || created.Source.CreatorAgentID != "agent-1" {
		t.Fatalf("CLI 创建任务应记录 Agent 来源，实际 %+v", created.Source)
	}

	runResult, isError := callAutomationCommand(t, service, sctx, "run_scheduled_task", map[string]any{
		"query": "新闻投递到智能体",
	})
	if isError {
		t.Fatalf("run_scheduled_task by query 不应失败: %s", automationCommandText(t, runResult))
	}
	runNow := decodeAutomationCommandJSON[automationdomain.ExecutionResult](t, runResult)
	if runNow.JobID != created.JobID {
		t.Fatalf("query 应定位到刚创建的任务，run=%+v created=%+v", runNow, created)
	}

	ownerCtx := automationCommandTestOwnerContext(sctx.OwnerUserID)
	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(ownerCtx, created.JobID)
		if listErr != nil || len(items) == 0 {
			return false
		}
		return items[0].DeliveryStatus == automationdomain.DeliveryStatusSucceeded
	})

	store := workspacestore.NewSessionFileStore(workspacePath)
	sessionValue, _, err := store.FindSession([]string{workspacePath}, recipientSessionKey)
	if err != nil {
		t.Fatalf("读取 CLI 真实当前 session 失败: %v", err)
	}
	if sessionValue == nil {
		t.Fatal("CLI 创建并运行后应写入真实当前会话")
	}
	if sessionValue.AgentID != "agent-1" {
		t.Fatalf("CLI 投递会话应归属目标智能体，实际 %+v", sessionValue)
	}
	assertDeliveredAgentMessage(t, workspacePath, *sessionValue, "今日新闻摘要", "CLI 真实当前会话")
	assertRunDeliveredToContext(t, ownerCtx, service, created.JobID, "explicit:internal:"+recipientSessionKey)

	statusResult, isError := callAutomationCommand(t, service, sctx, "inspect_scheduled_task", map[string]any{
		"query":       "新闻投递到智能体",
		"run_limit":   5,
		"event_limit": 5,
	})
	if isError {
		t.Fatalf("inspect_scheduled_task by query 不应失败: %s", automationCommandText(t, statusResult))
	}
	status := decodeAutomationCommandJSON[automationdomain.ScheduledTaskStatus](t, statusResult)
	if status.Job.JobID != created.JobID || status.Job.LastDeliveryStatus != automationdomain.DeliveryStatusSucceeded {
		t.Fatalf("CLI 状态应能看到任务最新投递成功，实际 %+v", status.Job)
	}
	if len(status.RecentRuns) == 0 || status.RecentRuns[0].DeliveryTo != "explicit:internal:"+recipientSessionKey {
		t.Fatalf("CLI 状态应返回最近投递目标，实际 %+v", status.RecentRuns)
	}
}

func TestRunTaskNowSkipsDuplicateExplicitDeliveryWhenTargetMatchesExecutionSession(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	delivery := &fakeDeliveryRouter{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		&fakeDMRunner{permission: permission, resultText: "done"},
		nil,
		permission,
		&fakeWorkspaceReader{},
		delivery,
	)
	sessionKey := protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "existing-chat", "")
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "dup-delivery",
		AgentID:     "agent-1",
		Instruction: "run once",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(60),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: sessionKey,
		},
		Delivery: automationdomain.DeliveryTarget{
			Mode:    automationdomain.DeliveryModeExplicit,
			Channel: "websocket",
			To:      sessionKey,
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	if _, err = service.RunTaskNow(context.Background(), task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil && len(items) > 0 && items[0].Status == automationdomain.RunStatusSucceeded
	})

	if len(delivery.Calls()) != 0 {
		t.Fatalf("execution 会话与显式回传目标一致时不应重复投递")
	}
}

func TestUnconfirmedScriptTreeTerminationNeverCallsDeliveryRouter(t *testing.T) {
	service, db, delivery, task := newInitialDeliveryTestService(t, "script-tree-unconfirmed")
	runID := "run-script-tree-unconfirmed"
	insertActiveDeliveryRun(t, db, service, task, runID, task.Delivery, "result")
	terminationErr := errors.New("process group drain proof failed")

	err := service.commitScriptObservation(
		context.Background(),
		task,
		runID,
		time.Now(),
		automationexec.ExecutionObservation{
			Status:       automationdomain.RunStatusSucceeded,
			MessageCount: 1,
			ResultText:   "must not be delivered",
		},
		terminationErr,
	)
	if !errors.Is(err, terminationErr) {
		t.Fatalf("termination proof failure must remain visible: %v", err)
	}
	if calls := delivery.Calls(); len(calls) != 0 {
		t.Fatalf("unconfirmed process tree must never enter delivery: %+v", calls)
	}
	run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
	if err != nil {
		t.Fatalf("load terminal script run: %v", err)
	}
	if run.Status != automationdomain.RunStatusFailed ||
		run.DeliveryStatus != automationdomain.DeliveryStatusNotAttempted ||
		run.ErrorMessage == nil || !strings.Contains(*run.ErrorMessage, "manual review") {
		t.Fatalf("unconfirmed process tree must persist safe terminal facts: %+v", run)
	}
}

func TestTerminalCommitCASMissAndDeletionProduceNoDelivery(t *testing.T) {
	if !shouldCommitDeletingRunTerminal(automationdomain.ErrTaskDeleting) ||
		!shouldCommitDeletingRunTerminal(automationdomain.ErrRunCompletionConflict) {
		t.Fatal("deletion race sentinels 必须进入 exact suppressed terminal 分支")
	}
	for _, test := range []struct {
		name          string
		prepare       func(*testing.T, *sql.DB, automationdomain.ScheduledTask, string)
		wantCommitted bool
	}{
		{
			name: "run already terminal",
			prepare: func(t *testing.T, db *sql.DB, task automationdomain.ScheduledTask, runID string) {
				t.Helper()
				if _, err := db.Exec(`UPDATE automation_task_runs SET status = ? WHERE run_id = ?`, automationdomain.RunStatusCancelled, runID); err != nil {
					t.Fatalf("准备 terminal CAS miss: %v", err)
				}
			},
		},
		{
			name: "task deleting",
			prepare: func(t *testing.T, db *sql.DB, task automationdomain.ScheduledTask, _ string) {
				t.Helper()
				if _, err := db.Exec(`UPDATE automation_scheduled_tasks
SET deletion_state = 'deleting', deletion_token = 'delete-token' WHERE job_id = ?`, task.JobID); err != nil {
					t.Fatalf("准备 deleting task: %v", err)
				}
			},
			wantCommitted: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, db, delivery, task := newInitialDeliveryTestService(t, "terminal-cas-"+strings.ReplaceAll(test.name, " ", "-"))
			runID := "run-" + strings.ReplaceAll(test.name, " ", "-")
			insertActiveDeliveryRun(t, db, service, task, runID, task.Delivery, "result")
			test.prepare(t, db, task, runID)

			observation := successfulRunFinish(runID, automationdomain.DeliveryStatusPending)
			updated, committed, err := service.commitObservedRunTerminal(context.Background(), task, observation)
			if committed != test.wantCommitted {
				t.Fatalf("terminal CAS miss = committed %v, want %v (err=%v)", committed, test.wantCommitted, err)
			}
			if test.wantCommitted {
				if err != nil || updated == nil || updated.DeliveryStatus != automationdomain.DeliveryStatusNotAttempted ||
					updated.DeliveryDeadLetterAt == nil {
					t.Fatalf("deleting terminal 必须 suppressed+deadletter: run=%+v err=%v", updated, err)
				}
			} else if !errors.Is(err, automationdomain.ErrRunCompletionConflict) {
				t.Fatalf("ordinary terminal CAS miss error = %v", err)
			}
			if calls := delivery.Calls(); len(calls) != 0 {
				t.Fatalf("terminal commit 失败后不得外投: %+v", calls)
			}
			if test.wantCommitted {
				service.mu.Lock()
				_, retained := service.jobStates[task.JobID]
				service.mu.Unlock()
				if retained {
					t.Fatal("suppressed deleting terminal 不得把 deleting task 留在 scheduler 内存 catalog")
				}
			}
		})
	}
}

func TestPendingInitialDeliveryConcurrentClaimAndCrashRecovery(t *testing.T) {
	service, db, delivery, task := newInitialDeliveryTestService(t, "pending-concurrent")
	runID := "run-pending-concurrent"
	insertTerminalPendingDeliveryRun(t, db, service, task, runID, task.Delivery, "durable result")
	if _, err := db.Exec(`UPDATE automation_scheduled_tasks SET enabled = FALSE WHERE job_id = ?`, task.JobID); err != nil {
		t.Fatalf("停用任务: %v", err)
	}
	task.Enabled = false
	run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
	if err != nil {
		t.Fatalf("读取 pending run: %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for range 2 {
		go func() {
			ready.Done()
			<-start
			_, deliveryErr := service.deliverPendingRun(context.Background(), task, *run)
			results <- deliveryErr
		}()
	}
	ready.Wait()
	close(start)
	firstErr := <-results
	secondErr := <-results
	if firstErr != nil && secondErr != nil {
		t.Fatalf("pending 并发 claim 至少一个应成功: %v / %v", firstErr, secondErr)
	}
	if calls := delivery.Calls(); len(calls) != 1 {
		t.Fatalf("pending 并发只能外投一次: %+v", calls)
	}
	updated, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
	if err != nil {
		t.Fatalf("读取投递结果: %v", err)
	}
	if updated.DeliveryStatus != automationdomain.DeliveryStatusSucceeded || updated.DeliveryAttempts != 1 {
		t.Fatalf("pending crash recovery 未完成 durable 状态: %+v", updated)
	}
}

func TestDeletingTerminalRequiresExactPrivateClaimIdentity(t *testing.T) {
	service, db, delivery, task := newInitialDeliveryTestService(t, "deleting-terminal-identity")
	runID := "run-deleting-terminal-identity"
	insertActiveDeliveryRun(t, db, service, task, runID, task.Delivery, "result")
	if _, err := db.Exec(`UPDATE automation_scheduled_tasks
SET deletion_state = 'review_required', deletion_token = 'authoritative-token' WHERE job_id = ?`, task.JobID); err != nil {
		t.Fatalf("准备 deletion claim: %v", err)
	}
	finish := successfulRunFinish(runID, automationdomain.DeliveryStatusPending)
	for _, input := range []automationstore.DeletingRunTerminalCommitInput{
		{OwnerUserID: task.OwnerUserID, JobID: task.JobID, DeletionToken: "wrong-token", Finish: finish},
		{OwnerUserID: "wrong-owner", JobID: task.JobID, DeletionToken: "authoritative-token", Finish: finish},
	} {
		if err := service.repository.CommitDeletingRunTerminal(context.Background(), input); !errors.Is(err, automationdomain.ErrRunCompletionConflict) {
			t.Fatalf("wrong deletion identity 必须拒绝: %+v err=%v", input, err)
		}
	}
	run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
	if err != nil {
		t.Fatalf("读取未变更 run: %v", err)
	}
	if run.Status != automationdomain.RunStatusRunning || run.DeliveryStatus == automationdomain.DeliveryStatusNotAttempted {
		t.Fatalf("错误 deletion identity 不得修改 run: %+v", run)
	}
	if len(delivery.Calls()) != 0 {
		t.Fatalf("deleting storage CAS 不得外投: %+v", delivery.Calls())
	}
}

func TestTerminalCommitRejectsWrongOwnerJobAndNonTerminalStatus(t *testing.T) {
	service, db, delivery, task := newInitialDeliveryTestService(t, "terminal-identity")
	runID := "run-terminal-identity"
	insertActiveDeliveryRun(t, db, service, task, runID, task.Delivery, "result")
	finishedAt := time.Now().UTC()
	for _, input := range []automationstore.RunTerminalCommitInput{
		{
			OwnerUserID: "wrong-owner", JobID: task.JobID,
			Finish: automationstore.RunFinishInput{RunID: runID, Status: automationdomain.RunStatusSucceeded, FinishedAt: finishedAt},
		},
		{
			OwnerUserID: task.OwnerUserID, JobID: "wrong-job",
			Finish: automationstore.RunFinishInput{RunID: runID, Status: automationdomain.RunStatusSucceeded, FinishedAt: finishedAt},
		},
		{
			OwnerUserID: task.OwnerUserID, JobID: task.JobID,
			Finish: automationstore.RunFinishInput{RunID: runID, Status: automationdomain.RunStatusRunning, FinishedAt: finishedAt},
		},
	} {
		if _, err := service.repository.CommitRunTerminalAndRuntime(context.Background(), input); !errors.Is(err, automationdomain.ErrRunCompletionConflict) {
			t.Fatalf("错误 terminal 身份/状态必须拒绝: %+v err=%v", input, err)
		}
	}
	run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
	if err != nil || run.Status != automationdomain.RunStatusRunning {
		t.Fatalf("错误 terminal claim 不得修改 run: run=%+v err=%v", run, err)
	}
	if len(delivery.Calls()) != 0 {
		t.Fatalf("错误 terminal claim 不得外投: %+v", delivery.Calls())
	}
}

func TestScriptLaunchRegistrationFailureUsesAtomicTerminalPath(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil,
		permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
	)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name: "script registration conflict", AgentID: "agent-1", Instruction: "printf result",
		ExecutionKind: automationdomain.ExecutionKindScript,
		Schedule: automationdomain.Schedule{
			Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone}, Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	runID := "run-script-registration-conflict"
	service.idFactory = func(prefix string) string {
		if prefix == "run" {
			return runID
		}
		return prefix + "-fixed"
	}
	service.scriptAttempts[runID] = &scriptAttempt{
		ownerUserID: task.OwnerUserID,
		jobID:       task.JobID,
		done:        make(chan struct{}),
	}
	t.Cleanup(func() { delete(service.scriptAttempts, runID) })

	if _, err = service.RunTaskNow(context.Background(), task.JobID); err == nil ||
		!strings.Contains(err.Error(), "already registered") {
		t.Fatalf("预置 script registration 应拒绝 launch: %v", err)
	}
	run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
	if err != nil || run.Status != automationdomain.RunStatusFailed ||
		run.DeliveryStatus != automationdomain.DeliveryStatusNotAttempted {
		t.Fatalf("script launch failure 必须原子落 terminal: run=%+v err=%v", run, err)
	}
	current, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || current == nil || current.RunningRunID != "" || current.Running ||
		current.LastRunStatus != automationdomain.RunStatusFailed {
		t.Fatalf("script launch failure 不得留下 running task: task=%+v err=%v", current, err)
	}
}

func TestOrphanPendingRecoveryDeadLettersWithoutDelivery(t *testing.T) {
	service, db, delivery, task := newInitialDeliveryTestService(t, "orphan-pending")
	runID := "run-orphan-pending"
	insertTerminalPendingDeliveryRun(t, db, service, task, runID, task.Delivery, "orphan result")
	if _, err := db.Exec(`DELETE FROM automation_scheduled_tasks WHERE job_id = ?`, task.JobID); err != nil {
		t.Fatalf("删除 task definition: %v", err)
	}
	run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
	if err != nil {
		t.Fatalf("读取 orphan pending: %v", err)
	}
	if err = service.retryDueRunDelivery(context.Background(), *run); err != nil {
		t.Fatalf("收口 orphan pending: %v", err)
	}
	updated, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
	if err != nil {
		t.Fatalf("读取 orphan 收口结果: %v", err)
	}
	if updated.DeliveryStatus != automationdomain.DeliveryStatusNotAttempted || updated.DeliveryDeadLetterAt == nil {
		t.Fatalf("orphan pending 必须 not_attempted+deadletter: %+v", updated)
	}
	if len(delivery.Calls()) != 0 {
		t.Fatalf("orphan pending 不得外投: %+v", delivery.Calls())
	}
}

func TestPendingDeliveryUsesFrozenRouteAndHonorsCurrentModeNone(t *testing.T) {
	t.Run("frozen route", func(t *testing.T) {
		service, db, delivery, task := newInitialDeliveryTestService(t, "frozen-route")
		frozen := fakeStructuredDelivery("agent-1", "frozen")
		current := fakeStructuredDelivery("agent-1", "current")
		runID := "run-frozen-route"
		insertTerminalPendingDeliveryRun(t, db, service, task, runID, frozen, "frozen result")
		if _, err := db.Exec(`UPDATE automation_scheduled_tasks
SET delivery_to = ?, delivery_session_key = ?, configuration_version = configuration_version + 1
WHERE job_id = ?`, current.To, current.SessionKey, task.JobID); err != nil {
			t.Fatalf("更新当前路由: %v", err)
		}
		run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
		if err != nil {
			t.Fatalf("读取 pending run: %v", err)
		}
		if _, err = service.deliverPendingRun(context.Background(), task, *run); err != nil {
			t.Fatalf("冻结路由首投失败: %v", err)
		}
		calls := delivery.Calls()
		if len(calls) != 1 || calls[0].SessionKey != frozen.SessionKey {
			t.Fatalf("首投必须使用 run 冻结路由: %+v", calls)
		}
	})

	t.Run("current mode none", func(t *testing.T) {
		service, db, delivery, task := newInitialDeliveryTestService(t, "current-none")
		runID := "run-current-none"
		insertTerminalPendingDeliveryRun(t, db, service, task, runID, task.Delivery, "result")
		if _, err := db.Exec(`UPDATE automation_scheduled_tasks
SET delivery_mode = ?, configuration_version = configuration_version + 1
WHERE job_id = ?`, automationdomain.DeliveryModeNone, task.JobID); err != nil {
			t.Fatalf("关闭当前投递: %v", err)
		}
		run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, runID)
		if err != nil {
			t.Fatalf("读取 pending run: %v", err)
		}
		updated, err := service.deliverPendingRun(context.Background(), task, *run)
		if err != nil {
			t.Fatalf("关闭投递后的 durable completion: %v", err)
		}
		if updated.DeliveryStatus != automationdomain.DeliveryStatusNotRequired || len(delivery.Calls()) != 0 {
			t.Fatalf("当前 mode none 必须跳过 router: run=%+v calls=%+v", updated, delivery.Calls())
		}
	})
}

func TestTerminalCommitReplacesCompletedOverlapRuntimeDeterministically(t *testing.T) {
	service, db, _, task := newInitialDeliveryTestService(t, "overlap-runtime")
	firstStarted := time.Now().UTC().Add(-2 * time.Minute)
	secondStarted := firstStarted.Add(time.Minute)
	for _, item := range []struct {
		runID     string
		startedAt time.Time
	}{
		{runID: "run-overlap-old", startedAt: firstStarted},
		{runID: "run-overlap-new", startedAt: secondStarted},
	} {
		if err := service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
			RunID: item.runID, JobID: task.JobID, OwnerUserID: task.OwnerUserID,
			TriggerKind: automationdomain.TriggerKindManual, DeliveryTarget: cloneDeliveryTargetPointer(task.Delivery),
		}); err != nil {
			t.Fatalf("创建 overlap run %s: %v", item.runID, err)
		}
		if err := service.repository.MarkRunRunning(context.Background(), item.runID, item.startedAt); err != nil {
			t.Fatalf("启动 overlap run %s: %v", item.runID, err)
		}
	}
	if _, err := db.Exec(`UPDATE automation_scheduled_tasks
SET running_run_id = ?, running_started_at = ? WHERE job_id = ?`, "run-overlap-old", firstStarted, task.JobID); err != nil {
		t.Fatalf("准备 overlap runtime: %v", err)
	}
	finishedAt := time.Now().UTC()
	result, err := service.repository.CommitRunTerminalAndRuntime(context.Background(), automationstore.RunTerminalCommitInput{
		OwnerUserID:                  task.OwnerUserID,
		JobID:                        task.JobID,
		ExpectedConfigurationVersion: task.ConfigurationVersion,
		Finish: automationstore.RunFinishInput{
			RunID: "run-overlap-old", Status: automationdomain.RunStatusSucceeded, FinishedAt: finishedAt,
			ResultText: stringPointer("result"), DeliveryStatus: automationdomain.DeliveryStatusPending,
		},
		Runtime: automationstore.JobRuntimeUpdateInput{
			JobID: task.JobID, LastRunAt: &finishedAt, LastRunStatus: automationdomain.RunStatusSucceeded,
			LastDeliveryStatus: automationdomain.DeliveryStatusPending,
		},
	})
	if err != nil || !result.Committed || !result.RuntimeUpdated {
		t.Fatalf("提交 overlap terminal: result=%+v err=%v", result, err)
	}
	current, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil {
		t.Fatalf("读取 overlap task: %v", err)
	}
	if current.RunningRunID != "run-overlap-new" || current.RunningStartedAt == nil || !current.RunningStartedAt.Equal(secondStarted) {
		t.Fatalf("完成旧 run 后必须回填确定的新 active run: %+v", current)
	}

	// A stale observation may finish after the task has moved on, but its older
	// finished_at must not replace the newer completion summary.
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID: "run-overlap-stale", JobID: task.JobID, OwnerUserID: task.OwnerUserID,
		TriggerKind: automationdomain.TriggerKindManual, DeliveryTarget: cloneDeliveryTargetPointer(task.Delivery),
	}); err != nil {
		t.Fatalf("创建 stale overlap run: %v", err)
	}
	if err = service.repository.MarkRunRunning(context.Background(), "run-overlap-stale", firstStarted.Add(-time.Minute)); err != nil {
		t.Fatalf("启动 stale overlap run: %v", err)
	}
	staleFinishedAt := finishedAt.Add(-time.Hour)
	result, err = service.repository.CommitRunTerminalAndRuntime(context.Background(), automationstore.RunTerminalCommitInput{
		OwnerUserID:                  task.OwnerUserID,
		JobID:                        task.JobID,
		ExpectedConfigurationVersion: task.ConfigurationVersion,
		Finish: automationstore.RunFinishInput{
			RunID: "run-overlap-stale", Status: automationdomain.RunStatusSucceeded, FinishedAt: staleFinishedAt,
			ResultText: stringPointer("stale"), DeliveryStatus: automationdomain.DeliveryStatusPending,
		},
		Runtime: automationstore.JobRuntimeUpdateInput{
			JobID: task.JobID, LastRunAt: &staleFinishedAt, LastRunStatus: automationdomain.RunStatusSucceeded,
			LastDeliveryStatus: automationdomain.DeliveryStatusPending,
		},
	})
	if err != nil || !result.Committed || result.RuntimeUpdated {
		t.Fatalf("提交非当前 stale overlap: result=%+v err=%v", result, err)
	}
	var lastCompletedRunID string
	var storedLastRunAt time.Time
	if err = db.QueryRow(`SELECT last_completed_run_id, last_run_at FROM automation_scheduled_tasks WHERE job_id = ?`, task.JobID).
		Scan(&lastCompletedRunID, &storedLastRunAt); err != nil {
		t.Fatalf("读取单调完成摘要: %v", err)
	}
	if lastCompletedRunID != "run-overlap-old" || !storedLastRunAt.Equal(finishedAt) {
		t.Fatalf("旧 finished_at 不得覆盖新完成摘要: last_completed=%s last_run_at=%v", lastCompletedRunID, storedLastRunAt)
	}
}

func newInitialDeliveryTestService(t *testing.T, name string) (*Service, *sql.DB, *fakeDeliveryRouter, automationdomain.ScheduledTask) {
	t.Helper()
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, &fakeWorkspaceReader{}, delivery)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        name,
		AgentID:     "agent-1",
		Instruction: "deliver durable result",
		Schedule: automationdomain.Schedule{
			Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: name},
		Delivery:      fakeStructuredDelivery("agent-1", name),
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	return service, db, delivery, *task
}

func insertActiveDeliveryRun(
	t *testing.T,
	db *sql.DB,
	service *Service,
	task automationdomain.ScheduledTask,
	runID string,
	target automationdomain.DeliveryTarget,
	result string,
) {
	t.Helper()
	if err := service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID: runID, JobID: task.JobID, OwnerUserID: task.OwnerUserID,
		TriggerKind: automationdomain.TriggerKindManual, DeliveryTarget: cloneDeliveryTargetPointer(target),
	}); err != nil {
		t.Fatalf("InsertRunPending: %v", err)
	}
	startedAt := time.Now().UTC().Add(-time.Second)
	if err := service.repository.MarkRunRunning(context.Background(), runID, startedAt); err != nil {
		t.Fatalf("MarkRunRunning: %v", err)
	}
	if _, err := db.Exec(`UPDATE automation_task_runs SET result_text = ? WHERE run_id = ?`, result, runID); err != nil {
		t.Fatalf("写入 run result: %v", err)
	}
	if _, err := db.Exec(`UPDATE automation_scheduled_tasks
SET running_run_id = ?, running_started_at = ? WHERE job_id = ?`, runID, startedAt, task.JobID); err != nil {
		t.Fatalf("写入 task runtime: %v", err)
	}
}

func insertTerminalPendingDeliveryRun(
	t *testing.T,
	db *sql.DB,
	service *Service,
	task automationdomain.ScheduledTask,
	runID string,
	target automationdomain.DeliveryTarget,
	result string,
) {
	t.Helper()
	if err := service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID: runID, JobID: task.JobID, OwnerUserID: task.OwnerUserID,
		TriggerKind: automationdomain.TriggerKindManual, Status: automationdomain.RunStatusSucceeded,
		DeliveryTarget: cloneDeliveryTargetPointer(target), DeliveryStatus: automationdomain.DeliveryStatusPending,
	}); err != nil {
		t.Fatalf("InsertRunPending terminal: %v", err)
	}
	finishedAt := time.Now().UTC().Add(-time.Second)
	if _, err := db.Exec(`UPDATE automation_task_runs
SET result_text = ?, finished_at = ?, delivery_attempts = 0 WHERE run_id = ?`, result, finishedAt, runID); err != nil {
		t.Fatalf("写入 terminal pending result: %v", err)
	}
}

func successfulRunFinish(runID string, deliveryStatus string) automationstore.RunFinishInput {
	finishedAt := time.Now().UTC()
	return automationstore.RunFinishInput{
		RunID: runID, Status: automationdomain.RunStatusSucceeded, FinishedAt: finishedAt,
		ResultText: stringPointer("result"), ResultSummary: stringPointer("result"), DeliveryStatus: deliveryStatus,
	}
}

func TestIMFeedbackPolicyUsesExactRunAndCurrentRestrictions(t *testing.T) {
	service, db, _, task := newInitialDeliveryTestService(t, "im-feedback")
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: task.OwnerUserID, Role: authctx.RoleOwner})
	source := imdelivery.Source{Kind: "automation", AgentID: task.AgentID, JobID: task.JobID, RunID: "im-run", SessionKey: "source-session", RoundID: "source-round"}
	if err := service.repository.InsertRunPending(ctx, automationstore.RunPendingInput{RunID: source.RunID, JobID: source.JobID, OwnerUserID: task.OwnerUserID, SessionKey: source.SessionKey, RoundID: source.RoundID, PermissionPolicyRevision: task.PermissionPolicy.Revision, Status: automationdomain.RunStatusSucceeded}); err != nil {
		t.Fatal(err)
	}
	policy, err := service.IMFeedbackPolicy(ctx, source)
	if err != nil || policy == nil || !reflect.DeepEqual(policy, taskRuntimeToolPolicy(task)) {
		t.Fatalf("policy %+v %v", policy, err)
	}
	for _, mutate := range []func(*imdelivery.Source){func(s *imdelivery.Source) { s.RoundID = "other" }, func(s *imdelivery.Source) { s.SessionKey = "other" }, func(s *imdelivery.Source) { s.AgentID = "other" }, func(s *imdelivery.Source) { s.RunID = "other" }} {
		bad := source
		mutate(&bad)
		if _, err := service.IMFeedbackPolicy(ctx, bad); err == nil {
			t.Fatal("foreign producer accepted")
		}
	}
	if _, err = db.Exec(`UPDATE automation_task_runs SET permission_policy_revision=permission_policy_revision+1 WHERE run_id=?`, source.RunID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.IMFeedbackPolicy(ctx, source); err == nil {
		t.Fatal("stale policy accepted")
	}
	run, err := service.repository.GetRun(ctx, task.OwnerUserID, task.JobID, source.RunID)
	if err != nil || run.Status != automationdomain.RunStatusSucceeded {
		t.Fatalf("feedback changed old run %+v %v", run, err)
	}
}
