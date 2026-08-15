// INPUT: owner-main 与 Room host 的可信对话 Actor、Room 删除 plan/approval/request_id。
// OUTPUT: owner-only 权限、Room version CAS、写后不存在证明、幂等与 reconcile 审计。
// POS: nexuscfg Room 删除端到端边界回归测试。
package configuration_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
)

type failingRoomDeletionGoalCleaner struct {
	err error
}

type recordingRoomDeletionNotifier struct {
	mu      sync.Mutex
	roomIDs []string
	reasons []string
}

func (*recordingRoomDeletionNotifier) AgentChanged(context.Context, string, string) {}

func (n *recordingRoomDeletionNotifier) RoomChanged(
	_ context.Context,
	roomID string,
	_ string,
	reason string,
) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.roomIDs = append(n.roomIDs, roomID)
	n.reasons = append(n.reasons, reason)
}

func (*recordingRoomDeletionNotifier) RoomMemberChanged(context.Context, string, string, bool) {}

func (n *recordingRoomDeletionNotifier) hasDeletion(roomID string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	for index := range n.roomIDs {
		if n.roomIDs[index] == roomID && n.reasons[index] == "room_deleted" {
			return true
		}
	}
	return false
}

func (f failingRoomDeletionGoalCleaner) DeleteGoalsForRoomConversations(
	context.Context,
	[]string,
) (int, error) {
	return 0, f.err
}

func (f failingRoomDeletionGoalCleaner) DeleteGoalsForRoomMember(
	context.Context,
	string,
	[]string,
) (int, error) {
	return 0, f.err
}

func TestOwnerMainRoomDeleteRequiresApprovalVerifiesAbsenceAndReplays(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	host := fixture.createAgent(t, "Delete Room Host")
	roomContext, err := fixture.services.Core.Room.CreateRoom(
		fixture.ownerCtx,
		protocol.CreateRoomRequest{
			AgentIDs:    []string{host.AgentID},
			Name:        "Conversational delete Room",
			HostAgentID: host.AgentID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	notifier := &recordingRoomDeletionNotifier{}
	fixture.services.Configuration.SetNotifier(notifier)

	hostActor := configurationsvc.Actor{
		OwnerUserID:    host.OwnerUserID,
		AgentID:        host.AgentID,
		ContextKind:    configurationsvc.ContextKindRoom,
		ContextID:      roomContext.Room.ID,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		SessionKey:     protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID),
		LeaseSessionKey: protocol.BuildRoomAgentSessionKey(
			roomContext.Conversation.ID,
			host.AgentID,
			roomContext.Room.RoomType,
		),
	}
	bindConfigurationTestRound(t, fixture.services, &hostActor)
	hostInspection, err := fixture.services.Configuration.Inspect(
		fixture.ownerCtx,
		hostActor,
		[]string{configurationsvc.DomainRooms},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if operations := hostInspection.Domains[configurationsvc.DomainRooms].Access.AllowedOperations; slices.Contains(operations, "delete") {
		t.Fatalf("Room host unexpectedly received rooms.delete: %v", operations)
	}
	if _, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		hostActor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainRooms,
			Operation: "delete",
			Target:    roomContext.Room.ID,
		},
	); err == nil {
		t.Fatal("Room host must not delete even its current Room")
	}
	fixture.services.Runtime.MarkRoundFinished(
		hostActor.LeaseSessionKey,
		hostActor.LeaseRoundID,
	)

	ownerActor := configurationsvc.Actor{
		OwnerUserID: fixture.main.OwnerUserID,
		AgentID:     fixture.main.AgentID,
		SessionKey:  "agent:" + fixture.main.AgentID + ":ws:dm:room-delete-success",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   fixture.main.AgentID,
	}
	bindConfigurationTestRound(t, fixture.services, &ownerActor)
	if _, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		ownerActor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainRooms,
			Operation: "delete",
		},
	); err == nil {
		t.Fatal("owner Room delete must bind an exact room_id target")
	}
	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		ownerActor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainRooms,
			Operation: "delete",
			Target:    roomContext.Room.ID,
			Input:     json.RawMessage(`{}`),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if plan.StateVersion != roomContext.Room.ConfigurationVersion ||
		!plan.RequiresConfirmation ||
		plan.Risk != "destructive" {
		t.Fatalf("Room delete plan does not bind version/approval: %+v", plan)
	}
	stalePlan := plan
	newerDescription := "updated after delete plan"
	updatedRoom, err := fixture.services.Core.Room.UpdateRoom(
		fixture.ownerCtx,
		roomContext.Room.ID,
		protocol.UpdateRoomRequest{
			Description:                  &newerDescription,
			ExpectedConfigurationVersion: &stalePlan.StateVersion,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		ownerActor,
		configurationsvc.ChangeRequest{
			RequestID:        "room-delete-stale-plan-01",
			Domain:           configurationsvc.DomainRooms,
			Operation:        "delete",
			Target:           roomContext.Room.ID,
			Input:            json.RawMessage(`{}`),
			ExpectedRevision: stalePlan.CurrentRevision,
			PlanDigest:       stalePlan.PlanDigest,
		},
	); err == nil {
		t.Fatal("stale Room delete plan must not remove newer configuration")
	}
	if preserved, getErr := fixture.services.Core.Room.GetRoom(
		fixture.ownerCtx,
		roomContext.Room.ID,
	); getErr != nil ||
		preserved.Room.ConfigurationVersion != updatedRoom.Room.ConfigurationVersion ||
		preserved.Room.Description != newerDescription {
		t.Fatalf("stale conversational delete damaged Room: room=%+v err=%v", preserved, getErr)
	}
	plan, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		ownerActor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainRooms,
			Operation: "delete",
			Target:    roomContext.Room.ID,
			Input:     json.RawMessage(`{}`),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := configurationsvc.ChangeRequest{
		RequestID:        "room-delete-success-01",
		Domain:           configurationsvc.DomainRooms,
		Operation:        "delete",
		Target:           roomContext.Room.ID,
		Input:            json.RawMessage(`{}`),
		ExpectedRevision: plan.CurrentRevision,
		PlanDigest:       plan.PlanDigest,
	}
	if _, err = fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		ownerActor,
		request,
	); err == nil {
		t.Fatal("Room delete must reject model-only confirmation without interactive human approval")
	}
	approveConfigurationTestChange(
		t,
		fixture.services,
		fixture.ownerCtx,
		ownerActor,
		request,
		plan,
	)
	applied, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		ownerActor,
		request,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied ||
		!hasConfigurationCheck(applied.Checks, "configuration_target_deleted") ||
		!notifier.hasDeletion(roomContext.Room.ID) {
		t.Fatalf("Room deletion was not verified: %+v", applied)
	}
	if _, err = fixture.services.Core.Room.GetRoom(
		fixture.ownerCtx,
		roomContext.Room.ID,
	); !errors.Is(err, roomsvc.ErrRoomNotFound) {
		t.Fatalf("Room still exists after verified deletion: %v", err)
	}

	replayed, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		ownerActor,
		request,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Applied || !replayed.IdempotentReplay {
		t.Fatalf("Room deletion request_id did not replay safely: %+v", replayed)
	}
}

func TestRoomDeletePostCommitCleanupFailureIsAuditedForReconcile(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	worker := fixture.createAgent(t, "Delete Room Reconcile Worker")
	roomContext, err := fixture.services.Core.Room.CreateRoom(
		fixture.ownerCtx,
		protocol.CreateRoomRequest{
			AgentIDs: []string{worker.AgentID},
			Name:     "Room reconcile",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	fixture.services.Core.Room.SetGoalCleaner(failingRoomDeletionGoalCleaner{
		err: errors.New("injected Room Goal cleanup failure"),
	})
	actor := configurationsvc.Actor{
		OwnerUserID: fixture.main.OwnerUserID,
		AgentID:     fixture.main.AgentID,
		SessionKey:  "agent:" + fixture.main.AgentID + ":ws:dm:room-delete-reconcile",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   fixture.main.AgentID,
	}
	bindConfigurationTestRound(t, fixture.services, &actor)
	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainRooms,
			Operation: "delete",
			Target:    roomContext.Room.ID,
			Input:     json.RawMessage(`{}`),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := configurationsvc.ChangeRequest{
		RequestID:        "room-delete-reconcile-01",
		Domain:           configurationsvc.DomainRooms,
		Operation:        "delete",
		Target:           roomContext.Room.ID,
		Input:            json.RawMessage(`{}`),
		ExpectedRevision: plan.CurrentRevision,
		PlanDigest:       plan.PlanDigest,
	}
	approveConfigurationTestChange(
		t,
		fixture.services,
		fixture.ownerCtx,
		actor,
		request,
		plan,
	)
	if _, err = fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		request,
	); err == nil {
		t.Fatalf("Room post-commit cleanup error classification=%v", err)
	}
	if _, err = fixture.services.Core.Room.GetRoom(
		fixture.ownerCtx,
		roomContext.Room.ID,
	); !errors.Is(err, roomsvc.ErrRoomNotFound) {
		t.Fatalf("Room database deletion did not commit: %v", err)
	}
	records, err := fixture.services.Configuration.ListChanges(
		fixture.ownerCtx,
		actor,
		configurationsvc.DomainRooms,
		10,
	)
	if err != nil {
		t.Fatal(err)
	}
	var matched *configurationsvc.AuditRecord
	for index := range records {
		if records[index].RequestID == request.RequestID {
			matched = &records[index]
			break
		}
	}
	if matched == nil ||
		matched.Status != "reconcile_required" ||
		!strings.Contains(string(matched.Result), `"applied":true`) {
		t.Fatalf("Room reconcile audit record=%+v", matched)
	}
	if _, err = fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		request,
	); err == nil || !strings.Contains(err.Error(), "reconcile") {
		t.Fatalf("reconcile request_id must not execute deletion again: %v", err)
	}
}
