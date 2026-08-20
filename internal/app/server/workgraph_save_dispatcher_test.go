package server

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	workgraphworkflowsvc "github.com/nexus-research-lab/nexus/internal/service/workgraphworkflow"
)

type workGraphDMRoundRecorder struct {
	owner   string
	request dmsvc.Request
}

func (r *workGraphDMRoundRecorder) HandleChat(ctx context.Context, request dmsvc.Request) error {
	r.owner = authctx.OwnerUserID(ctx)
	r.request = request
	return nil
}

type workGraphRoomRoundRecorder struct {
	owner   string
	request roomrealtime.ChatRequest
}

func (r *workGraphRoomRoundRecorder) HandleChat(ctx context.Context, request roomrealtime.ChatRequest) error {
	r.owner = authctx.OwnerUserID(ctx)
	r.request = request
	return nil
}

func TestWorkGraphSaveDispatcherUsesHiddenInternalDMRound(t *testing.T) {
	dm := &workGraphDMRoundRecorder{}
	dispatcher := newWorkGraphSaveRoundDispatcher(dm, nil)
	err := dispatcher.DispatchWorkGraphSave(context.Background(), workgraphworkflowsvc.SaveRoundRequest{
		OwnerUserID: "owner-a",
		SessionKey:  "agent:agent-a:websocket:dm:conversation-a",
		PreviewID:   "preview-a",
		Prompt:      "save exact preview",
	})
	if err != nil {
		t.Fatal(err)
	}
	request := dm.request
	if dm.owner != "owner-a" || request.AgentID != "agent-a" || request.Content != "save exact preview" || !request.Internal {
		t.Fatalf("DM background request = %#v, owner=%q", request, dm.owner)
	}
	if !request.InputOptions.HiddenFromUser || !request.InputOptions.Synthetic || request.InputOptions.Purpose != workGraphSaveRoundPurpose || request.InputOptions.Priority != "internal" || request.InputOptions.Metadata["preview_id"] != "preview-a" {
		t.Fatalf("DM input options = %#v", request.InputOptions)
	}
	if request.BroadcastUserMessage {
		t.Fatal("DM background save broadcast a user message")
	}
}

func TestWorkGraphSaveDispatcherUsesHiddenInternalRoomRound(t *testing.T) {
	room := &workGraphRoomRoundRecorder{}
	dispatcher := newWorkGraphSaveRoundDispatcher(nil, room)
	err := dispatcher.DispatchWorkGraphSave(context.Background(), workgraphworkflowsvc.SaveRoundRequest{
		OwnerUserID: "owner-a",
		SessionKey:  "room:group:conversation-a",
		PreviewID:   "preview-a",
		Prompt:      "save exact preview",
	})
	if err != nil {
		t.Fatal(err)
	}
	request := room.request
	if room.owner != "owner-a" || request.ConversationID != "conversation-a" || request.Content != "save exact preview" || !request.Internal {
		t.Fatalf("Room background request = %#v, owner=%q", request, room.owner)
	}
	if !request.InputOptions.HiddenFromUser || !request.InputOptions.Synthetic || request.InputOptions.Purpose != workGraphSaveRoundPurpose || request.InputOptions.Metadata["preview_id"] != "preview-a" {
		t.Fatalf("Room input options = %#v", request.InputOptions)
	}
	if request.BroadcastUserMessage {
		t.Fatal("Room background save broadcast a user message")
	}
}
