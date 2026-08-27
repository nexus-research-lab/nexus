package workgraph

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	workgraphworkflowsvc "github.com/nexus-research-lab/nexus/internal/service/workgraphworkflow"
)

type isolatedRoundRecorder struct {
	owner          string
	sessionRequest dmsvc.TransientSessionRequest
	request        dmsvc.Request
}

func (r *isolatedRoundRecorder) CreateTransientSession(
	ctx context.Context,
	request dmsvc.TransientSessionRequest,
) (*protocol.Session, error) {
	r.owner = authctx.OwnerUserID(ctx)
	r.sessionRequest = request
	return &protocol.Session{SessionKey: request.TargetSessionKey, AgentID: request.AgentID}, nil
}

func (r *isolatedRoundRecorder) HandleChat(ctx context.Context, request dmsvc.Request) error {
	r.owner = authctx.OwnerUserID(ctx)
	r.request = request
	return nil
}

func TestSaveDispatcherUsesIsolatedHiddenInternalDMRound(t *testing.T) {
	dm := &isolatedRoundRecorder{}
	dispatcher := NewSaveRoundDispatcher(dm)
	err := dispatcher.DispatchWorkGraphSave(context.Background(), workgraphworkflowsvc.SaveRoundRequest{
		OwnerUserID:      "owner-a",
		AgentID:          "agent-a",
		SourceSessionKey: "agent:agent-a:websocket:dm:conversation-a",
		PreviewID:        "preview-a",
		Prompt:           "save exact preview",
	})
	if err != nil {
		t.Fatal(err)
	}
	request := dm.request
	if dm.owner != "owner-a" || request.AgentID != "agent-a" || request.Content != "save exact preview" || !request.Internal {
		t.Fatalf("DM background request = %#v, owner=%q", request, dm.owner)
	}
	if request.SessionKey == request.WorkGraphSaveSourceSessionKey ||
		request.WorkGraphSaveSourceSessionKey != "agent:agent-a:websocket:dm:conversation-a" ||
		request.SessionKey != dm.sessionRequest.TargetSessionKey {
		t.Fatalf("isolated save identities: session=%q source=%q create=%#v", request.SessionKey, request.WorkGraphSaveSourceSessionKey, dm.sessionRequest)
	}
	parsed := protocol.ParseSessionKey(request.SessionKey)
	if parsed.Channel != protocol.SessionChannelInternalSegment || parsed.AgentID != "agent-a" ||
		dm.sessionRequest.Purpose != protocol.SessionPurposeWorkGraphDistillation {
		t.Fatalf("isolated Session = %#v parsed=%#v", dm.sessionRequest, parsed)
	}
	if !request.InputOptions.HiddenFromUser || !request.InputOptions.Synthetic || request.InputOptions.Purpose != saveRoundPurpose || request.InputOptions.Priority != "internal" || request.InputOptions.Metadata["preview_id"] != "preview-a" {
		t.Fatalf("DM input options = %#v", request.InputOptions)
	}
	if request.BroadcastUserMessage {
		t.Fatal("DM background save broadcast a user message")
	}
}

func TestSaveDispatcherKeepsRoomSourceOnlyAsCommandScope(t *testing.T) {
	dm := &isolatedRoundRecorder{}
	dispatcher := NewSaveRoundDispatcher(dm)
	err := dispatcher.DispatchWorkGraphSave(context.Background(), workgraphworkflowsvc.SaveRoundRequest{
		OwnerUserID:      "owner-a",
		AgentID:          "agent-a",
		SourceSessionKey: "room:group:conversation-a",
		PreviewID:        "preview-a",
		Prompt:           "save exact preview",
	})
	if err != nil {
		t.Fatal(err)
	}
	request := dm.request
	if dm.owner != "owner-a" || request.SessionKey == "room:group:conversation-a" ||
		request.WorkGraphSaveSourceSessionKey != "room:group:conversation-a" || request.Content != "save exact preview" || !request.Internal {
		t.Fatalf("isolated Room-source request = %#v, owner=%q", request, dm.owner)
	}
	if !request.InputOptions.HiddenFromUser || !request.InputOptions.Synthetic || request.InputOptions.Purpose != saveRoundPurpose || request.InputOptions.Metadata["preview_id"] != "preview-a" {
		t.Fatalf("isolated input options = %#v", request.InputOptions)
	}
	if request.BroadcastUserMessage {
		t.Fatal("isolated background save broadcast a user message")
	}
}
