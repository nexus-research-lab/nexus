package websocket

import (
	"context"
	"errors"
	"testing"

	operationsvc "github.com/nexus-research-lab/nexus/internal/service/operation"
)

type stagePresenceRefresherStub struct {
	clientID   string
	err        error
	sessionKey string
	touchCount int
}

func (s *stagePresenceRefresherStub) TouchStagePresence(
	_ context.Context,
	sessionKey string,
	clientID string,
) (*operationsvc.StagePresence, error) {
	s.touchCount++
	s.sessionKey = sessionKey
	s.clientID = clientID
	if s.err != nil {
		return nil, s.err
	}
	return &operationsvc.StagePresence{
		Active:     true,
		ClientID:   clientID,
		SessionKey: sessionKey,
	}, nil
}

func TestControlMessageRefreshStagePresence(t *testing.T) {
	refresher := &stagePresenceRefresherStub{}
	message := &controlMessage{
		ctx:        context.Background(),
		handler:    &Handler{stagePresence: refresher},
		inbound:    map[string]any{"operation_stage_client_id": " stage-client "},
		sessionKey: "agent:nexus:ws:dm:conversation-1",
	}

	if err := message.refreshStagePresence(); err != nil {
		t.Fatalf("refreshStagePresence() error = %v", err)
	}
	if refresher.touchCount != 1 {
		t.Fatalf("touch count = %d, want 1", refresher.touchCount)
	}
	if refresher.sessionKey != message.sessionKey {
		t.Fatalf("session key = %q, want %q", refresher.sessionKey, message.sessionKey)
	}
	if refresher.clientID != "stage-client" {
		t.Fatalf("client ID = %q, want %q", refresher.clientID, "stage-client")
	}
}

func TestControlMessageRefreshStagePresenceWithoutClientIsNoop(t *testing.T) {
	refresher := &stagePresenceRefresherStub{}
	message := &controlMessage{
		ctx:        context.Background(),
		handler:    &Handler{stagePresence: refresher},
		inbound:    map[string]any{},
		sessionKey: "agent:nexus:ws:dm:conversation-1",
	}

	if err := message.refreshStagePresence(); err != nil {
		t.Fatalf("refreshStagePresence() error = %v", err)
	}
	if refresher.touchCount != 0 {
		t.Fatalf("touch count = %d, want 0", refresher.touchCount)
	}
}

func TestControlMessageRefreshStagePresencePropagatesFailure(t *testing.T) {
	wantErr := errors.New("presence unavailable")
	refresher := &stagePresenceRefresherStub{err: wantErr}
	message := &controlMessage{
		ctx:        context.Background(),
		handler:    &Handler{stagePresence: refresher},
		inbound:    map[string]any{"operation_stage_client_id": "stage-client"},
		sessionKey: "agent:nexus:ws:dm:conversation-1",
	}

	if err := message.refreshStagePresence(); !errors.Is(err, wantErr) {
		t.Fatalf("refreshStagePresence() error = %v, want %v", err, wantErr)
	}
}
