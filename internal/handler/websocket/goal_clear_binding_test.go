package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/nexus-research-lab/nexus/internal/config"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
)

type websocketClearGoalRepository struct {
	item    *protocol.Goal
	deleted bool
}

func (r *websocketClearGoalRepository) CreateGoal(context.Context, protocol.Goal) (*protocol.Goal, error) {
	return nil, nil
}

func (r *websocketClearGoalRepository) GetGoal(_ context.Context, goalID string) (*protocol.Goal, error) {
	if r.item == nil || r.item.ID != goalID {
		return nil, nil
	}
	item := *r.item
	return &item, nil
}

func (r *websocketClearGoalRepository) GetCurrentGoal(_ context.Context, sessionKey string) (*protocol.Goal, error) {
	if r.item == nil || r.item.SessionKey != sessionKey {
		return nil, nil
	}
	item := *r.item
	return &item, nil
}

func (*websocketClearGoalRepository) ListGoals(context.Context) ([]protocol.Goal, error) {
	return nil, nil
}

func (*websocketClearGoalRepository) ListRunnableGoals(context.Context, int) ([]protocol.Goal, error) {
	return nil, nil
}

func (*websocketClearGoalRepository) UpdateGoal(context.Context, protocol.Goal, int64) (*protocol.Goal, error) {
	return nil, nil
}

func (*websocketClearGoalRepository) FinalizeGoalUsage(context.Context, protocol.Goal, int64, protocol.GoalEvent) (*protocol.Goal, error) {
	return nil, nil
}

func (r *websocketClearGoalRepository) DeleteGoal(_ context.Context, goalID string) (bool, error) {
	if r.item == nil || r.item.ID != goalID {
		return false, nil
	}
	r.deleted = true
	r.item = nil
	return true, nil
}

func (*websocketClearGoalRepository) AppendEvent(context.Context, protocol.GoalEvent) error {
	return nil
}

func (*websocketClearGoalRepository) ListEvents(context.Context, string, int) ([]protocol.GoalEvent, error) {
	return nil, nil
}

type websocketConfirmedClearResolver struct{}

func (websocketConfirmedClearResolver) ResolveGoalExecutionBinding(
	context.Context,
	protocol.Goal,
) (protocol.GoalExecutionBindingResolution, error) {
	return protocol.GoalExecutionBindingResolution{
		State:               protocol.GoalExecutionBindingStateConfirmed,
		ExecutionID:         "execution-clear-websocket",
		ReservedExecutionID: "execution-clear-websocket",
	}, nil
}

func (websocketConfirmedClearResolver) ExecutionGoalCompletionBlocker(
	context.Context,
	protocol.Goal,
) (string, error) {
	return "", nil
}

func TestThreadGoalClearWebSocketRejectsConfirmedExecutionBinding(t *testing.T) {
	repo := &websocketClearGoalRepository{item: &protocol.Goal{
		ID: "goal-clear-websocket", SessionKey: "agent:nexus:ws:dm:clear-websocket",
		Objective: "keep bound Goal", Status: protocol.GoalStatusActive, Version: 1,
		Metadata: map[string]any{protocol.GoalMetadataOwnerUserID: authsvc.SystemUserID},
	}}
	goals := goalsvc.NewService(config.Config{GoalEnabled: true}, repo)
	goals.SetExecutionGoalCompletionReadiness(websocketConfirmedClearResolver{})
	handler := &Handler{goals: goals}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		connection, err := websocket.Accept(writer, request, nil)
		if err != nil {
			return
		}
		defer func() { _ = connection.Close(websocket.StatusNormalClosure, "test complete") }()
		sender := handlershared.NewWebSocketSender(connection)
		var inbound map[string]any
		if err = wsjson.Read(request.Context(), connection, &inbound); err != nil {
			return
		}
		handler.handleAppServerRPC(request.Context(), sender, inbound)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, _, err := websocket.Dial(
		ctx,
		"ws"+strings.TrimPrefix(server.URL, "http"),
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = connection.Close(websocket.StatusNormalClosure, "test complete") }()
	if err = wsjson.Write(ctx, connection, map[string]any{
		"id": 1, "method": "thread/goal/clear",
		"params": map[string]any{"threadId": repo.item.SessionKey},
	}); err != nil {
		t.Fatal(err)
	}
	var response goalappserver.AppServerJSONRPCError
	if err = wsjson.Read(ctx, connection, &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != goalappserver.AppServerRPCInvalidRequestCode || repo.deleted {
		t.Fatalf("response = %#v deleted=%v, want invalid request without deletion", response, repo.deleted)
	}
}

func TestThreadGoalWebSocketUnauthorizedOperationsDoNotSubscribe(t *testing.T) {
	const (
		goalOwner = "goal-owner"
		attacker  = "other-owner"
		threadID  = "agent:nexus:ws:dm:private-goal"
	)
	tests := []struct {
		name   string
		method string
		params map[string]any
	}{
		{
			name:   "set",
			method: "thread/goal/set",
			params: map[string]any{"threadId": threadID, "objective": "replace private Goal"},
		},
		{
			name:   "get",
			method: "thread/goal/get",
			params: map[string]any{"threadId": threadID},
		},
		{
			name:   "clear",
			method: "thread/goal/clear",
			params: map[string]any{"threadId": threadID},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &websocketClearGoalRepository{item: &protocol.Goal{
				ID:         "goal-private-" + test.name,
				SessionKey: threadID,
				Objective:  "private Goal",
				Status:     protocol.GoalStatusActive,
				Version:    1,
				Metadata: map[string]any{
					protocol.GoalMetadataOwnerUserID: goalOwner,
				},
			}}
			handler := &Handler{
				goals:       goalsvc.NewService(config.Config{GoalEnabled: true}, repo),
				goalRPCSubs: newAppServerGoalRPCRegistry(),
			}
			handled := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				connection, err := websocket.Accept(writer, request, nil)
				if err != nil {
					return
				}
				defer func() { _ = connection.Close(websocket.StatusNormalClosure, "test complete") }()
				sender := handlershared.NewWebSocketSender(connection)
				var inbound map[string]any
				if err = wsjson.Read(request.Context(), connection, &inbound); err != nil {
					return
				}
				ctx := authsvc.WithPrincipal(request.Context(), &authsvc.Principal{UserID: attacker})
				handler.handleAppServerRPC(ctx, sender, inbound)
				close(handled)
			}))
			defer server.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			connection, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = connection.Close(websocket.StatusNormalClosure, "test complete") }()
			if err = wsjson.Write(ctx, connection, map[string]any{
				"id": 1, "method": test.method, "params": test.params,
			}); err != nil {
				t.Fatal(err)
			}
			var response goalappserver.AppServerJSONRPCError
			if err = wsjson.Read(ctx, connection, &response); err != nil {
				t.Fatal(err)
			}
			<-handled
			if response.Error.Code != goalappserver.AppServerRPCInvalidRequestCode {
				t.Fatalf("response = %#v, want owner authorization failure", response)
			}
			handler.goalRPCSubs.mu.RLock()
			subscriptionCount := len(handler.goalRPCSubs.senderTo)
			handler.goalRPCSubs.mu.RUnlock()
			if subscriptionCount != 0 {
				t.Fatalf("subscriptions = %d, want rejected %s to register none", subscriptionCount, test.name)
			}
			if repo.deleted {
				t.Fatal("unauthorized clear deleted Goal")
			}
		})
	}
}
