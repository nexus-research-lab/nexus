package server

import (
	"context"
	"testing"

	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
)

type fakeGoalInterruptDM struct {
	requests []dmsvc.InterruptRequest
}

func (f *fakeGoalInterruptDM) HandleInterrupt(_ context.Context, request dmsvc.InterruptRequest) error {
	f.requests = append(f.requests, request)
	return nil
}

type fakeGoalInterruptRoom struct {
	requests []roomsvc.InterruptRequest
}

func (f *fakeGoalInterruptRoom) HandleInterrupt(_ context.Context, request roomsvc.InterruptRequest) error {
	f.requests = append(f.requests, request)
	return nil
}

func TestGoalInterruptDispatcherRoutesAgentGoalToDM(t *testing.T) {
	dm := &fakeGoalInterruptDM{}
	room := &fakeGoalInterruptRoom{}
	dispatcher := &goalInterruptDispatcher{dm: dm, room: room}

	if err := dispatcher.InterruptGoalRuntime(context.Background(), "agent:nexus:ws:dm:thread-1"); err != nil {
		t.Fatalf("InterruptGoalRuntime() error = %v", err)
	}
	if len(dm.requests) != 1 || dm.requests[0].SessionKey != "agent:nexus:ws:dm:thread-1" {
		t.Fatalf("dm requests = %#v, want agent session interrupt", dm.requests)
	}
	if len(room.requests) != 0 {
		t.Fatalf("room requests = %#v, want none", room.requests)
	}
}

func TestGoalInterruptDispatcherRoutesRoomGoalToRoom(t *testing.T) {
	dm := &fakeGoalInterruptDM{}
	room := &fakeGoalInterruptRoom{}
	dispatcher := &goalInterruptDispatcher{dm: dm, room: room}

	if err := dispatcher.InterruptGoalRuntime(context.Background(), "room:group:conversation-1"); err != nil {
		t.Fatalf("InterruptGoalRuntime() error = %v", err)
	}
	if len(room.requests) != 1 || room.requests[0].SessionKey != "room:group:conversation-1" {
		t.Fatalf("room requests = %#v, want shared room interrupt", room.requests)
	}
	if len(dm.requests) != 0 {
		t.Fatalf("dm requests = %#v, want none", dm.requests)
	}
}

func TestGoalInterruptDispatcherRoutesRoomAgentGoalToSharedRoom(t *testing.T) {
	room := &fakeGoalInterruptRoom{}
	dispatcher := &goalInterruptDispatcher{room: room}

	if err := dispatcher.InterruptGoalRuntime(context.Background(), "agent:agent-1:ws:group:conversation-1"); err != nil {
		t.Fatalf("InterruptGoalRuntime() error = %v", err)
	}
	if len(room.requests) != 1 || room.requests[0].SessionKey != "room:group:conversation-1" {
		t.Fatalf("room requests = %#v, want shared room session interrupt", room.requests)
	}
}
