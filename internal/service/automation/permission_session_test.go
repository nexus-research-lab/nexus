package automation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

type automationPermissionEventRecorder struct {
	mu     sync.Mutex
	events []protocol.EventMessage
}

func (r *automationPermissionEventRecorder) NotifyAutomationPermissionEvent(
	_ context.Context,
	event protocol.EventMessage,
) {
	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
}

func (r *automationPermissionEventRecorder) snapshot() []protocol.EventMessage {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]protocol.EventMessage(nil), r.events...)
}

func TestAutomationPermissionProjectsToRecipientDMAndResolvesFromComposer(t *testing.T) {
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{
		permission:               permission,
		requiredTool:             "WebSearch",
		skipPermissionAfterFirst: true,
	}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		newAutomationTestDB(t),
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	recipientSession := protocol.BuildRoomAgentSessionKey(
		"dm-recipient-conversation",
		"agent-recipient",
		protocol.RoomTypeDM,
	)
	roomID := "dm-recipient-room"
	conversationID := "dm-recipient-conversation"
	service.SetDeliverySessionResolver(fakeAutomationDeliverySessionResolver{sessions: map[string]protocol.Session{
		recipientSession: {
			SessionKey:     recipientSession,
			AgentID:        "agent-recipient",
			ChannelType:    protocol.SessionChannelWebSocket,
			RoomID:         &roomID,
			ConversationID: &conversationID,
		},
	}})
	recorder := &automationPermissionEventRecorder{}
	service.SetPermissionSessionEventNotifier(recorder)
	ownerCtx := automationCommandTestOwnerContext("user-1")
	task, err := service.CreateTask(ownerCtx, automationdomain.CreateJobInput{
		Name:        "接收会话审批",
		AgentID:     "agent-executor",
		Instruction: "搜索最新资料",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind: automationdomain.SessionTargetIsolated,
		},
		Delivery: automationdomain.DeliveryTarget{
			Mode:       automationdomain.DeliveryModeExplicit,
			Channel:    protocol.SessionChannelWebSocket,
			To:         recipientSession,
			SessionKey: recipientSession,
		},
		Source:  automationdomain.Source{Kind: automationdomain.SourceKindUserPage},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if _, err = service.RunTaskNow(ownerCtx, task.JobID); err != nil {
		t.Fatalf("RunTaskNow() error = %v", err)
	}

	var request automationdomain.AutomationPermissionRequest
	waitFor(t, 2*time.Second, func() bool {
		requests, listErr := service.ListPermissionRequests(
			ownerCtx,
			automationdomain.PermissionRequestStatusPending,
			task.JobID,
		)
		if listErr != nil || len(requests) != 1 {
			return false
		}
		request = requests[0]
		return len(recorder.snapshot()) == 1
	})
	events := recorder.snapshot()
	if request.DeliverySessionKey != recipientSession ||
		len(events) != 1 ||
		events[0].EventType != protocol.EventTypePermissionRequest ||
		events[0].SessionKey != recipientSession ||
		events[0].RoomID != roomID ||
		events[0].ConversationID != conversationID ||
		events[0].AgentID != "agent-recipient" ||
		events[0].Data["request_source"] != "automation" ||
		events[0].Data["automation_task_name"] != task.Name ||
		events[0].Data["automation_allow_task"] != true {
		t.Fatalf("recipient permission event = %+v, request = %+v", events, request)
	}

	replayed, err := service.ListSessionPermissionEvents(ownerCtx, recipientSession)
	if err != nil || len(replayed) != 1 ||
		replayed[0].Data["request_id"] != request.RequestID {
		t.Fatalf("ListSessionPermissionEvents() = %+v, %v", replayed, err)
	}
	wrongSession := protocol.BuildAgentSessionKey(
		"agent-recipient",
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"wrong-conversation",
		"",
	)
	handled, err := service.ResolveSessionPermissionResponse(ownerCtx, wrongSession, map[string]any{
		"request_id": request.RequestID,
		"decision":   "allow",
	})
	if err != nil || handled {
		t.Fatalf("wrong recipient must not handle request: handled=%v err=%v", handled, err)
	}

	handled, err = service.ResolveSessionPermissionResponse(ownerCtx, recipientSession, map[string]any{
		"request_id":       request.RequestID,
		"decision":         "allow",
		"automation_scope": "task",
	})
	if err != nil || !handled {
		t.Fatalf("recipient decision failed: handled=%v err=%v", handled, err)
	}
	stored, err := service.repository.GetPermissionRequest(
		ownerCtx,
		task.OwnerUserID,
		request.RequestID,
	)
	if err != nil || stored.Status != automationdomain.PermissionRequestStatusApproved ||
		stored.Decision != automationdomain.PermissionDecisionAllowTask {
		t.Fatalf("stored decision = %+v, %v", stored, err)
	}
	events = recorder.snapshot()
	if len(events) != 2 ||
		events[1].EventType != protocol.EventTypePermissionRequestResolved ||
		events[1].SessionKey != recipientSession ||
		events[1].Data["request_id"] != request.RequestID {
		t.Fatalf("resolved recipient event = %+v", events)
	}
	if replayed, err = service.ListSessionPermissionEvents(ownerCtx, recipientSession); err != nil || len(replayed) != 0 {
		t.Fatalf("resolved request must not replay: %+v, %v", replayed, err)
	}
	requestIDs, err := service.PendingPermissionRequestIDsForRoom(ownerCtx, roomID, conversationID)
	if err != nil || len(requestIDs) != 0 {
		t.Fatalf("resolved request must leave room snapshot: %+v, %v", requestIDs, err)
	}
}

func TestAutomationPermissionProjectsToConfiguredRoom(t *testing.T) {
	permission := permissionctx.NewContext()
	conversationID := "room-recipient-conversation"
	roomID := "room-recipient"
	room := &fakeRoomRunner{
		permission: permission,
		contexts: map[string]*protocol.ConversationContextAggregate{
			conversationID: {
				Room:         protocol.RoomRecord{ID: roomID, RoomType: protocol.RoomTypeGroup},
				Conversation: protocol.ConversationRecord{ID: conversationID, RoomID: roomID},
			},
		},
	}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		newAutomationTestDB(t),
		nil,
		nil,
		room,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	recorder := &automationPermissionEventRecorder{}
	service.SetPermissionSessionEventNotifier(recorder)
	ownerCtx := automationCommandTestOwnerContext("user-1")
	roomSession := protocol.BuildRoomSharedSessionKey(conversationID)
	job := automationdomain.ScheduledTask{
		JobID:       "task-room-recipient",
		OwnerUserID: "user-1",
		Name:        "Room 收件任务",
		AgentID:     "agent-executor",
		Delivery: automationdomain.DeliveryTarget{
			Mode:       automationdomain.DeliveryModeExplicit,
			SessionKey: roomSession,
		},
		PermissionPolicy: automationdomain.TaskPermissionPolicy{
			Version:  taskPermissionPolicyVersion,
			Revision: 1,
		},
		PermissionState: automationdomain.TaskPermissionStateReady,
	}
	job, request := persistAutomationPermissionSessionFixture(t, service, ownerCtx, job, "permission-room-recipient")
	cancelledCtx, cancel := context.WithCancel(ownerCtx)
	cancel()
	service.publishScheduledPermissionRequest(cancelledCtx, scheduledPermissionScope{
		Job: job, RunID: request.RunID, SessionKey: request.SessionKey, RoundID: request.RoundID,
	}, request)

	events := recorder.snapshot()
	if len(events) != 1 || events[0].SessionKey != roomSession ||
		events[0].RoomID != roomID || events[0].ConversationID != conversationID ||
		events[0].AgentID != job.AgentID {
		t.Fatalf("Room recipient event = %+v", events)
	}
	requestIDs, err := service.PendingPermissionRequestIDsForRoom(ownerCtx, roomID, conversationID)
	if err != nil || len(requestIDs) != 1 || requestIDs[0] != request.RequestID {
		t.Fatalf("Room pending snapshot = %+v, %v", requestIDs, err)
	}
}

func TestAutomationPermissionProjectsToExternalRecipientSession(t *testing.T) {
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		newAutomationTestDB(t),
		nil,
		nil,
		nil,
		permissionctx.NewContext(),
		&fakeWorkspaceReader{},
		nil,
	)
	recipientSession := protocol.BuildAgentAccountSessionKey(
		"agent-recipient",
		protocol.SessionChannelWeixinPersonal,
		protocol.RoomTypeDM,
		"account-1",
		"user-1",
		"",
	)
	service.SetDeliverySessionResolver(fakeAutomationDeliverySessionResolver{sessions: map[string]protocol.Session{
		recipientSession: {
			SessionKey:  recipientSession,
			AgentID:     "agent-recipient",
			ChannelType: protocol.SessionChannelWeixinPersonal,
			ChatType:    protocol.RoomTypeDM,
			ExternalIdentity: &protocol.ExternalSessionIdentity{
				ChannelType:    protocol.SessionChannelWeixinPersonal,
				PairingStatus:  "active",
				CurrentPairing: true,
			},
		},
	}})
	recorder := &automationPermissionEventRecorder{}
	service.SetPermissionSessionEventNotifier(recorder)
	ownerCtx := automationCommandTestOwnerContext("user-1")
	job := automationdomain.ScheduledTask{
		JobID:       "task-external-recipient",
		OwnerUserID: "user-1",
		Name:        "外部 IM 收件任务",
		AgentID:     "agent-executor",
		Delivery: automationdomain.DeliveryTarget{
			Mode:       automationdomain.DeliveryModeLast,
			SessionKey: recipientSession,
		},
		Source: automationdomain.Source{
			Kind:       automationdomain.SourceKindUserPage,
			SessionKey: protocol.BuildAgentSessionKey("agent-executor", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "source", ""),
		},
		PermissionPolicy: automationdomain.TaskPermissionPolicy{
			Version: taskPermissionPolicyVersion, Revision: 1,
		},
		PermissionState: automationdomain.TaskPermissionStateReady,
	}
	job, request := persistAutomationPermissionSessionFixture(
		t, service, ownerCtx, job, "permission-external-recipient",
	)
	service.publishScheduledPermissionRequest(ownerCtx, scheduledPermissionScope{
		Job: job, RunID: request.RunID,
	}, request)

	events := recorder.snapshot()
	if request.DeliverySessionKey != recipientSession || len(events) != 1 ||
		events[0].SessionKey != recipientSession || events[0].AgentID != "agent-recipient" ||
		events[0].EventType != protocol.EventTypePermissionRequest {
		t.Fatalf("external recipient permission event = %+v, request = %+v", events, request)
	}
	replayed, err := service.ListSessionPermissionEvents(ownerCtx, recipientSession)
	if err != nil || len(replayed) != 1 || replayed[0].Data["request_id"] != request.RequestID {
		t.Fatalf("external recipient replay = %+v, %v", replayed, err)
	}
}

func TestAutomationPermissionRejectsInactiveExternalRecipientSession(t *testing.T) {
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		newAutomationTestDB(t),
		nil,
		nil,
		nil,
		permissionctx.NewContext(),
		&fakeWorkspaceReader{},
		nil,
	)
	recipientSession := protocol.BuildAgentAccountSessionKey(
		"agent-recipient",
		protocol.SessionChannelWeixinPersonal,
		protocol.RoomTypeDM,
		"account-1",
		"user-1",
		"",
	)
	service.SetDeliverySessionResolver(fakeAutomationDeliverySessionResolver{sessions: map[string]protocol.Session{
		recipientSession: {
			SessionKey:  recipientSession,
			AgentID:     "agent-recipient",
			ChannelType: protocol.SessionChannelWeixinPersonal,
			ChatType:    protocol.RoomTypeDM,
			ExternalIdentity: &protocol.ExternalSessionIdentity{
				ChannelType:    protocol.SessionChannelWeixinPersonal,
				PairingStatus:  "revoked",
				CurrentPairing: false,
			},
		},
	}})

	_, recognized, err := service.resolveAutomationPermissionSessionRoute(
		automationCommandTestOwnerContext("user-1"),
		automationdomain.ScheduledTask{AgentID: "agent-executor"},
		automationdomain.AutomationPermissionRequest{DeliverySessionKey: recipientSession},
	)
	if !recognized || !errors.Is(err, automationdomain.ErrTaskDeliverySessionUnavailable) {
		t.Fatalf("inactive external recipient route recognized = %v, err = %v", recognized, err)
	}
}

func TestAutomationPermissionApprovalSessionPrefersRunRecipientThenSource(t *testing.T) {
	recipientAtRunStart := protocol.BuildAgentSessionKey(
		"agent-recipient", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "recipient-at-start", "",
	)
	currentRecipient := protocol.BuildAgentSessionKey(
		"agent-recipient", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "recipient-current", "",
	)
	sourceSession := protocol.BuildAgentSessionKey(
		"agent-executor", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "source", "",
	)
	job := automationdomain.ScheduledTask{
		Delivery: automationdomain.DeliveryTarget{
			Mode: automationdomain.DeliveryModeExplicit, SessionKey: currentRecipient,
		},
		Source: automationdomain.Source{SessionKey: sourceSession},
	}
	run := automationdomain.ScheduledTaskRun{DeliveryTarget: &automationdomain.DeliveryTarget{
		Mode: automationdomain.DeliveryModeExplicit, SessionKey: recipientAtRunStart,
	}}
	if got := automationPermissionRunRecipientSessionKey(job, run); got != recipientAtRunStart {
		t.Fatalf("run recipient = %q, want frozen %q", got, recipientAtRunStart)
	}
	run.DeliveryTarget = &automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone}
	if got := automationPermissionRunRecipientSessionKey(job, run); got != sourceSession {
		t.Fatalf("no-recipient approval session = %q, want source %q", got, sourceSession)
	}
	job.Source.SessionKey = ""
	if got := automationPermissionRunRecipientSessionKey(job, run); got != "" {
		t.Fatalf("approval session without recipient or source = %q", got)
	}
}

func TestAutomationPermissionUsesSourceSessionWhenRecipientIsMissing(t *testing.T) {
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		newAutomationTestDB(t),
		nil,
		nil,
		nil,
		permissionctx.NewContext(),
		&fakeWorkspaceReader{},
		nil,
	)
	sourceSession := protocol.BuildAgentSessionKey(
		"agent-executor", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "source-fallback", "",
	)
	service.SetDeliverySessionResolver(fakeAutomationDeliverySessionResolver{sessions: map[string]protocol.Session{
		sourceSession: {
			SessionKey: sourceSession,
			AgentID:    "agent-executor",
		},
	}})
	recorder := &automationPermissionEventRecorder{}
	service.SetPermissionSessionEventNotifier(recorder)
	ownerCtx := automationCommandTestOwnerContext("user-1")
	job := automationdomain.ScheduledTask{
		JobID:       "task-source-fallback",
		OwnerUserID: "user-1",
		Name:        "来源会话审批兜底",
		AgentID:     "agent-executor",
		Delivery:    automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Source: automationdomain.Source{
			Kind: automationdomain.SourceKindUserPage, SessionKey: sourceSession,
		},
		PermissionPolicy: automationdomain.TaskPermissionPolicy{
			Version: taskPermissionPolicyVersion, Revision: 1,
		},
		PermissionState: automationdomain.TaskPermissionStateReady,
	}
	job, request := persistAutomationPermissionSessionFixture(
		t, service, ownerCtx, job, "permission-source-fallback",
	)
	service.publishScheduledPermissionRequest(ownerCtx, scheduledPermissionScope{
		Job: job, RunID: request.RunID,
	}, request)

	events := recorder.snapshot()
	if request.DeliverySessionKey != sourceSession || len(events) != 1 ||
		events[0].SessionKey != sourceSession {
		t.Fatalf("source fallback permission event = %+v, request = %+v", events, request)
	}
}

func persistAutomationPermissionSessionFixture(
	t *testing.T,
	service *Service,
	ctx context.Context,
	job automationdomain.ScheduledTask,
	requestID string,
) (automationdomain.ScheduledTask, automationdomain.AutomationPermissionRequest) {
	t.Helper()
	persisted, err := service.repository.UpsertScheduledTask(ctx, job)
	if err != nil {
		t.Fatalf("UpsertScheduledTask() error = %v", err)
	}
	runID := "run-" + requestID
	if err = service.repository.InsertRunPending(ctx, automationstore.RunPendingInput{
		RunID:                    runID,
		JobID:                    persisted.JobID,
		OwnerUserID:              persisted.OwnerUserID,
		PermissionPolicyRevision: persisted.PermissionPolicy.Revision,
	}); err != nil {
		t.Fatalf("InsertRunPending() error = %v", err)
	}
	request, _, err := service.repository.CreatePermissionRequestAndBlockRun(
		ctx,
		automationstore.PermissionRequestCreateInput{
			Request: automationdomain.AutomationPermissionRequest{
				RequestID:          requestID,
				OwnerUserID:        persisted.OwnerUserID,
				JobID:              persisted.JobID,
				RunID:              runID,
				PolicyRevision:     persisted.PermissionPolicy.Revision,
				Kind:               automationdomain.PermissionRequestKindTool,
				Capability:         automationdomain.PermissionCapability{ToolName: "WebSearch", Effect: automationdomain.PermissionEffectRead, InputFingerprint: requestID},
				Description:        "需要搜索资料",
				DeliverySessionKey: automationPermissionApprovalSessionKey(persisted.Delivery, persisted.Source),
				ResumeSafe:         true,
			},
			TaskState:  automationdomain.TaskPermissionStateAwaitingApproval,
			BlockState: automationdomain.RunBlockStateAwaitingApproval,
		},
	)
	if err != nil {
		t.Fatalf("CreatePermissionRequestAndBlockRun() error = %v", err)
	}
	updated, err := service.repository.GetScheduledTask(ctx, persisted.OwnerUserID, persisted.JobID)
	if err != nil || updated == nil {
		t.Fatalf("GetScheduledTask() = %+v, %v", updated, err)
	}
	return *updated, *request
}
