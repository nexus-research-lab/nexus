package automation

import (
	"context"
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
	ownerCtx := automationMCPTestOwnerContext("user-1")
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
	ownerCtx := automationMCPTestOwnerContext("user-1")
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
	service.notifyAutomationPermissionRequest(ownerCtx, job, request)

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
				DeliverySessionKey: persisted.Delivery.SessionKey,
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
