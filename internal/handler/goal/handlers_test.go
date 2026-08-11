package goal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nexus-research-lab/nexus/internal/config"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

type emptyGoalRepository struct{}

type staticGoalRepository struct {
	emptyGoalRepository
	item protocol.Goal
}

type createdGoalRepository struct {
	emptyGoalRepository
	created *protocol.Goal
}

type mutableGoalRepository struct {
	emptyGoalRepository
	item *protocol.Goal
}

func (r *mutableGoalRepository) GetGoal(_ context.Context, goalID string) (*protocol.Goal, error) {
	if r.item == nil || r.item.ID != goalID {
		return nil, nil
	}
	item := *r.item
	item.Metadata = cloneGoalMetadataForTest(r.item.Metadata)
	return &item, nil
}

func (r *mutableGoalRepository) GetCurrentGoal(_ context.Context, sessionKey string) (*protocol.Goal, error) {
	if r.item == nil || r.item.SessionKey != sessionKey || !protocol.IsCurrentGoalStatus(r.item.Status) {
		return nil, nil
	}
	item := *r.item
	item.Metadata = cloneGoalMetadataForTest(r.item.Metadata)
	return &item, nil
}

func (r *mutableGoalRepository) UpdateGoal(_ context.Context, item protocol.Goal, _ int64) (*protocol.Goal, error) {
	item.Metadata = cloneGoalMetadataForTest(item.Metadata)
	r.item = &item
	result := item
	result.Metadata = cloneGoalMetadataForTest(item.Metadata)
	return &result, nil
}

func cloneGoalMetadataForTest(metadata map[string]any) map[string]any {
	if metadata == nil {
		return nil
	}
	cloned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

type clearGoalRepository struct {
	emptyGoalRepository
	item    *protocol.Goal
	deleted bool
}

func (r *clearGoalRepository) GetGoal(_ context.Context, goalID string) (*protocol.Goal, error) {
	if r.item == nil || r.item.ID != goalID {
		return nil, nil
	}
	item := *r.item
	return &item, nil
}

func (r *clearGoalRepository) GetCurrentGoal(_ context.Context, sessionKey string) (*protocol.Goal, error) {
	if r.item == nil || r.item.SessionKey != sessionKey {
		return nil, nil
	}
	item := *r.item
	return &item, nil
}

func (r *clearGoalRepository) DeleteGoal(_ context.Context, goalID string) (bool, error) {
	if r.item == nil || r.item.ID != goalID {
		return false, nil
	}
	r.deleted = true
	r.item = nil
	return true, nil
}

type confirmedClearBindingResolver struct{}

func (confirmedClearBindingResolver) ResolveGoalExecutionBinding(
	context.Context,
	protocol.Goal,
) (protocol.GoalExecutionBindingResolution, error) {
	return protocol.GoalExecutionBindingResolution{
		State:               protocol.GoalExecutionBindingStateConfirmed,
		ExecutionID:         "execution-clear-handler",
		ReservedExecutionID: "execution-clear-handler",
	}, nil
}

func (confirmedClearBindingResolver) ExecutionGoalCompletionBlocker(
	context.Context,
	protocol.Goal,
) (string, error) {
	return "", nil
}

func (r *createdGoalRepository) CreateGoal(_ context.Context, item protocol.Goal) (*protocol.Goal, error) {
	r.created = &item
	return &item, nil
}

func (emptyGoalRepository) CreateGoal(context.Context, protocol.Goal) (*protocol.Goal, error) {
	return nil, nil
}

func (emptyGoalRepository) GetGoal(context.Context, string) (*protocol.Goal, error) {
	return nil, nil
}

func (emptyGoalRepository) GetCurrentGoal(context.Context, string) (*protocol.Goal, error) {
	return nil, nil
}

func (emptyGoalRepository) ListGoals(context.Context) ([]protocol.Goal, error) {
	return nil, nil
}

func (emptyGoalRepository) ListRunnableGoals(context.Context, int) ([]protocol.Goal, error) {
	return nil, nil
}

func (emptyGoalRepository) UpdateGoal(context.Context, protocol.Goal, int64) (*protocol.Goal, error) {
	return nil, nil
}

func (emptyGoalRepository) FinalizeGoalUsage(context.Context, protocol.Goal, int64, protocol.GoalEvent) (*protocol.Goal, error) {
	return nil, nil
}

func (emptyGoalRepository) DeleteGoal(context.Context, string) (bool, error) {
	return false, nil
}

func (emptyGoalRepository) AppendEvent(context.Context, protocol.GoalEvent) error {
	return nil
}

func (emptyGoalRepository) ListEvents(context.Context, string, int) ([]protocol.GoalEvent, error) {
	return nil, nil
}

func (r staticGoalRepository) GetGoal(_ context.Context, goalID string) (*protocol.Goal, error) {
	if r.item.ID != goalID {
		return nil, nil
	}
	item := r.item
	return &item, nil
}

func (r staticGoalRepository) GetCurrentGoal(_ context.Context, sessionKey string) (*protocol.Goal, error) {
	if r.item.SessionKey != sessionKey {
		return nil, nil
	}
	item := r.item
	return &item, nil
}

func (r staticGoalRepository) ListEvents(_ context.Context, goalID string, _ int) ([]protocol.GoalEvent, error) {
	if r.item.ID != goalID {
		return nil, nil
	}
	return []protocol.GoalEvent{{GoalID: goalID, EventType: "created"}}, nil
}

func TestHandleGetCurrentGoalMissingReturnsSuccessNull(t *testing.T) {
	service := goalsvc.NewService(config.Config{GoalEnabled: true}, emptyGoalRepository{})
	handler := New(handlershared.NewAPI(nil), service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/nexus/v1/goals/current?session_key=agent:nexus:ws:dm:chat",
		nil,
	)
	response := httptest.NewRecorder()

	handler.HandleGetCurrentGoal(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	var payload struct {
		Code    string         `json:"code"`
		Success bool           `json:"success"`
		Data    *protocol.Goal `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Code != "0000" || !payload.Success {
		t.Fatalf("payload = %#v, want success", payload)
	}
	if payload.Data != nil {
		t.Fatalf("data = %#v, want nil", payload.Data)
	}
}

func TestHandleCreateGoalTreatsRequestAsUserInput(t *testing.T) {
	repo := &createdGoalRepository{}
	service := goalsvc.NewService(config.Config{GoalEnabled: true}, repo)
	handler := New(handlershared.NewAPI(nil), service)
	request := httptest.NewRequest(
		http.MethodPost,
		"/nexus/v1/goals",
		strings.NewReader(`{"session_key":"room:group:goal-create","objective":"Start the Room Goal","created_by":"model","room_lead_agent_id":"agent-selected","metadata":{"execution_id":"execution-client-selected","execution_binding_state":"confirmed","explicit_goal_command":"spoofed-command","room_goal_lead_agent_id":"agent-forged","room_goal_lead_agent_name":"Forged"}}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.HandleCreateGoal(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	if repo.created == nil || repo.created.CreatedBy != "user" {
		t.Fatalf("created = %#v, want user-created Goal", repo.created)
	}
	if protocol.GoalReservedExecutionID(*repo.created) != "" {
		t.Fatalf("created = %#v, want no client-selected Execution", repo.created)
	}
	if _, exists := repo.created.Metadata[protocol.GoalMetadataExecutionBindingState]; exists {
		t.Fatalf("created = %#v, want no client-selected binding state", repo.created)
	}
	if got := goalsvc.RoomLeadAgentID(*repo.created); got != "agent-selected" {
		t.Fatalf("Room Goal lead = %q, want explicit verified field", got)
	}
	if got := goalsvc.RoomLeadAgentName(*repo.created); got != "" {
		t.Fatalf("Room Goal lead name = %q, want server-owned directory value", got)
	}
}

func TestGoalHTTPMutationEntrypointsCannotRewriteRoomLeadMetadata(t *testing.T) {
	const ownerUserID = "owner-room-lead"
	tests := []struct {
		name         string
		method       string
		path         string
		body         string
		initial      map[string]any
		wantLeadID   string
		wantLeadName string
	}{
		{
			name:   "REST update preserves verified lead",
			method: http.MethodPatch,
			path:   "/nexus/v1/goals/goal-room-lead",
			body:   `{"metadata":{"room_goal_creator_agent_id":"agent-forged","room_goal_lead_agent_id":"agent-outsider","room_goal_lead_agent_name":"Forged"}}`,
			initial: map[string]any{
				protocol.GoalMetadataOwnerUserID:            ownerUserID,
				protocol.GoalMetadataRoomGoalScope:          "room",
				protocol.GoalMetadataRoomGoalCreatorAgentID: "agent-original",
				protocol.GoalMetadataRoomGoalLeadAgentID:    "agent-original",
				protocol.GoalMetadataRoomGoalLeadAgentName:  "Original",
			},
			wantLeadID: "agent-original", wantLeadName: "Original",
		},
		{
			name:   "REST update cannot claim legacy lead",
			method: http.MethodPatch,
			path:   "/nexus/v1/goals/goal-room-lead",
			body:   `{"metadata":{"room_goal_scope":"room","room_goal_creator_agent_id":"agent-forged","room_goal_lead_agent_id":"agent-self","room_goal_lead_agent_name":"Self"}}`,
			initial: map[string]any{
				protocol.GoalMetadataOwnerUserID: ownerUserID,
			},
		},
		{
			name:   "app-server set ignores metadata and preserves verified lead",
			method: http.MethodPost,
			path:   "/nexus/v1/app-server/thread/goal/set",
			body:   `{"threadId":"room:group:lead-entry","objective":"updated objective","metadata":{"room_goal_lead_agent_id":"agent-outsider","room_goal_lead_agent_name":"Forged"}}`,
			initial: map[string]any{
				protocol.GoalMetadataOwnerUserID:            ownerUserID,
				protocol.GoalMetadataRoomGoalScope:          "room",
				protocol.GoalMetadataRoomGoalCreatorAgentID: "agent-original",
				protocol.GoalMetadataRoomGoalLeadAgentID:    "agent-original",
				protocol.GoalMetadataRoomGoalLeadAgentName:  "Original",
			},
			wantLeadID: "agent-original", wantLeadName: "Original",
		},
		{
			name:   "app-server set cannot claim legacy lead",
			method: http.MethodPost,
			path:   "/nexus/v1/app-server/thread/goal/set",
			body:   `{"threadId":"room:group:lead-entry","objective":"updated objective","metadata":{"room_goal_scope":"room","room_goal_lead_agent_id":"agent-self","room_goal_lead_agent_name":"Self"}}`,
			initial: map[string]any{
				protocol.GoalMetadataOwnerUserID: ownerUserID,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &mutableGoalRepository{item: &protocol.Goal{
				ID:         "goal-room-lead",
				SessionKey: "room:group:lead-entry",
				Objective:  "original objective",
				Status:     protocol.GoalStatusActive,
				Version:    1,
				Metadata:   cloneGoalMetadataForTest(test.initial),
			}}
			service := goalsvc.NewService(config.Config{GoalEnabled: true}, repo)
			handler := New(handlershared.NewAPI(nil), service)
			router := chi.NewRouter()
			router.Patch("/nexus/v1/goals/{goal_id}", handler.HandleUpdateGoal)
			router.Post("/nexus/v1/app-server/thread/goal/set", handler.HandleThreadGoalSet)

			request := goalHandlerRequestForOwner(test.method, test.path, test.body, ownerUserID)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s, want 200", response.Code, response.Body.String())
			}
			if got := protocol.GoalMetadataString(repo.item.Metadata, protocol.GoalMetadataRoomGoalCreatorAgentID); got != protocol.GoalMetadataString(test.initial, protocol.GoalMetadataRoomGoalCreatorAgentID) {
				t.Fatalf("creator = %q, want original server-owned value", got)
			}
			if got := protocol.GoalMetadataString(repo.item.Metadata, protocol.GoalMetadataRoomGoalScope); got != protocol.GoalMetadataString(test.initial, protocol.GoalMetadataRoomGoalScope) {
				t.Fatalf("scope = %q, want original server-owned value", got)
			}
			if got := protocol.GoalMetadataString(repo.item.Metadata, protocol.GoalMetadataRoomGoalLeadAgentID); got != test.wantLeadID {
				t.Fatalf("lead id = %q, want %q", got, test.wantLeadID)
			}
			if got := protocol.GoalMetadataString(repo.item.Metadata, protocol.GoalMetadataRoomGoalLeadAgentName); got != test.wantLeadName {
				t.Fatalf("lead name = %q, want %q", got, test.wantLeadName)
			}
		})
	}
}

func TestHandleGetGoalUsageReturnsFinalizedAggregateByID(t *testing.T) {
	finalizedAt := time.Date(2026, 7, 27, 13, 0, 0, 0, time.UTC)
	service := goalsvc.NewService(config.Config{GoalEnabled: true}, staticGoalRepository{
		item: protocol.Goal{
			ID:         "goal-final",
			SessionKey: "agent:nexus:ws:dm:final",
			Status:     protocol.GoalStatusComplete,
			Usage: protocol.GoalUsage{
				InputTokens:       10,
				OutputTokens:      2,
				ActualTotalTokens: 42,
			},
			TimeUsedSeconds:  5,
			UsageFinalized:   true,
			UsageFinalizedAt: &finalizedAt,
			UpdatedAt:        finalizedAt,
			Metadata: map[string]any{
				protocol.GoalMetadataOwnerUserID: "__system__",
			},
		},
	})
	handler := New(handlershared.NewAPI(nil), service)
	router := chi.NewRouter()
	router.Get("/nexus/v1/goals/{goal_id}/usage", handler.HandleGetGoalUsage)
	request := httptest.NewRequest(http.MethodGet, "/nexus/v1/goals/goal-final/usage", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	var payload struct {
		Code    string                    `json:"code"`
		Success bool                      `json:"success"`
		Data    *protocol.GoalUsageReport `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Code != "0000" || !payload.Success || payload.Data == nil {
		t.Fatalf("payload = %#v, want finalized usage success", payload)
	}
	if payload.Data.GoalID != "goal-final" || !payload.Data.UsageFinalized ||
		payload.Data.Usage.ActualTokens() != 42 || payload.Data.Usage.BudgetTokens() != 12 {
		t.Fatalf("data = %#v, want exact finalized aggregate", payload.Data)
	}
}

func TestGoalReadHandlersRequireMatchingDurableOwner(t *testing.T) {
	const (
		ownerUserID = "owner-goal-read"
		otherUserID = "other-goal-read"
		goalID      = "goal-owner-read"
		threadID    = "agent:nexus:ws:dm:owner-read"
	)
	service := goalsvc.NewService(config.Config{GoalEnabled: true}, staticGoalRepository{
		item: protocol.Goal{
			ID:         goalID,
			SessionKey: threadID,
			Objective:  "Keep owner-scoped Goal private",
			Status:     protocol.GoalStatusActive,
			Version:    1,
			Metadata: map[string]any{
				protocol.GoalMetadataOwnerUserID: ownerUserID,
			},
		},
	})
	service.SetExecutionGoalCompletionReadiness(confirmedClearBindingResolver{})
	handler := New(handlershared.NewAPI(nil), service)
	router := chi.NewRouter()
	router.Get("/nexus/v1/goals/current", handler.HandleGetCurrentGoal)
	router.Get("/nexus/v1/goals/{goal_id}/execution-binding", handler.HandleGetGoalExecutionBinding)
	router.Get("/nexus/v1/goals/{goal_id}/usage", handler.HandleGetGoalUsage)
	router.Get("/nexus/v1/goals/{goal_id}/events", handler.HandleGoalEvents)
	router.Post("/nexus/v1/app-server/thread/goal/get", handler.HandleThreadGoalGet)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "REST current", method: http.MethodGet, path: "/nexus/v1/goals/current?session_key=" + threadID},
		{name: "REST execution binding", method: http.MethodGet, path: "/nexus/v1/goals/" + goalID + "/execution-binding"},
		{name: "REST usage", method: http.MethodGet, path: "/nexus/v1/goals/" + goalID + "/usage"},
		{name: "REST events", method: http.MethodGet, path: "/nexus/v1/goals/" + goalID + "/events"},
		{
			name:   "app-server HTTP get",
			method: http.MethodPost,
			path:   "/nexus/v1/app-server/thread/goal/get",
			body:   `{"threadId":"` + threadID + `"}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name+" same owner", func(t *testing.T) {
			request := goalHandlerRequestForOwner(test.method, test.path, test.body, ownerUserID)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s, want 200", response.Code, response.Body.String())
			}
		})
		t.Run(test.name+" other owner", func(t *testing.T) {
			request := goalHandlerRequestForOwner(test.method, test.path, test.body, otherUserID)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d body=%s, want 403", response.Code, response.Body.String())
			}
		})
	}

	t.Run("ownerless legacy Goal is not exposed", func(t *testing.T) {
		ownerless := New(handlershared.NewAPI(nil), goalsvc.NewService(
			config.Config{GoalEnabled: true},
			staticGoalRepository{item: protocol.Goal{
				ID:         "goal-ownerless-read",
				SessionKey: "agent:nexus:ws:dm:ownerless-read",
				Objective:  "legacy Goal without owner provenance",
				Status:     protocol.GoalStatusActive,
			}},
		))
		request := goalHandlerRequestForOwner(
			http.MethodGet,
			"/nexus/v1/goals/current?session_key=agent:nexus:ws:dm:ownerless-read",
			"",
			ownerUserID,
		)
		response := httptest.NewRecorder()
		ownerless.HandleGetCurrentGoal(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("status = %d body=%s, want ownerless Goal hidden with 403", response.Code, response.Body.String())
		}
	})
}

func TestHandleGetGoalExecutionBindingExposesOnlyConfirmedExactExecution(t *testing.T) {
	const ownerUserID = "owner-binding-view"
	service := goalsvc.NewService(config.Config{GoalEnabled: true}, staticGoalRepository{
		item: protocol.Goal{
			ID:         "goal-binding-view",
			SessionKey: "agent:nexus:ws:dm:binding-view",
			Objective:  "Read binding view",
			Status:     protocol.GoalStatusActive,
			Version:    1,
			Metadata: map[string]any{
				protocol.GoalMetadataOwnerUserID: ownerUserID,
			},
		},
	})
	service.SetExecutionGoalCompletionReadiness(confirmedClearBindingResolver{})
	handler := New(handlershared.NewAPI(nil), service)
	router := chi.NewRouter()
	router.Get(
		"/nexus/v1/goals/{goal_id}/execution-binding",
		handler.HandleGetGoalExecutionBinding,
	)
	request := goalHandlerRequestForOwner(
		http.MethodGet,
		"/nexus/v1/goals/goal-binding-view/execution-binding",
		"",
		ownerUserID,
	)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", response.Code, response.Body.String())
	}
	var payload struct {
		Data protocol.GoalExecutionBindingView `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.State != protocol.GoalExecutionBindingStateConfirmed ||
		payload.Data.ExecutionID != "execution-clear-handler" {
		t.Fatalf("binding view = %#v, want confirmed exact execution", payload.Data)
	}
	if strings.Contains(response.Body.String(), "reserved_execution_id") {
		t.Fatalf("response leaked reservation provenance: %s", response.Body.String())
	}
}

func goalHandlerRequestForOwner(method, path, body, ownerUserID string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return request.WithContext(authsvc.WithPrincipal(request.Context(), &authsvc.Principal{
		UserID: ownerUserID,
	}))
}

func TestHandleClearGoalRejectsConfirmedExecutionBinding(t *testing.T) {
	repo := &clearGoalRepository{item: &protocol.Goal{
		ID: "goal-clear-rest", SessionKey: "agent:nexus:ws:dm:clear-rest",
		Objective: "keep bound Goal", Status: protocol.GoalStatusActive, Version: 1,
		Metadata: map[string]any{protocol.GoalMetadataOwnerUserID: authsvc.SystemUserID},
	}}
	service := goalsvc.NewService(config.Config{GoalEnabled: true}, repo)
	service.SetExecutionGoalCompletionReadiness(confirmedClearBindingResolver{})
	handler := New(handlershared.NewAPI(nil), service)
	router := chi.NewRouter()
	router.Post("/nexus/v1/goals/{goal_id}/clear", handler.HandleClearGoal)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(
		http.MethodPost,
		"/nexus/v1/goals/goal-clear-rest/clear",
		nil,
	))
	if response.Code != http.StatusUnprocessableEntity || repo.deleted {
		t.Fatalf("status = %d deleted=%v body=%s, want fail-closed 422", response.Code, repo.deleted, response.Body.String())
	}
}

func TestHandleThreadGoalClearRejectsConfirmedExecutionBinding(t *testing.T) {
	repo := &clearGoalRepository{item: &protocol.Goal{
		ID: "goal-clear-http", SessionKey: "agent:nexus:ws:dm:clear-http",
		Objective: "keep bound Goal", Status: protocol.GoalStatusActive, Version: 1,
		Metadata: map[string]any{protocol.GoalMetadataOwnerUserID: authsvc.SystemUserID},
	}}
	service := goalsvc.NewService(config.Config{GoalEnabled: true}, repo)
	service.SetExecutionGoalCompletionReadiness(confirmedClearBindingResolver{})
	handler := New(handlershared.NewAPI(nil), service)
	response := httptest.NewRecorder()
	handler.HandleThreadGoalClear(response, httptest.NewRequest(
		http.MethodPost,
		"/nexus/v1/app-server/thread/goal/clear",
		strings.NewReader(`{"threadId":"agent:nexus:ws:dm:clear-http"}`),
	))
	if response.Code != http.StatusUnprocessableEntity || repo.deleted {
		t.Fatalf("status = %d deleted=%v body=%s, want fail-closed 422", response.Code, repo.deleted, response.Body.String())
	}
}

func TestWriteGoalErrorPreservesConflictAndInvalidStateTransportStatus(t *testing.T) {
	handler := New(handlershared.NewAPI(nil), nil)
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "version stale", err: goalsvc.ErrGoalVersionStale, wantStatus: http.StatusConflict},
		{name: "objective revision stale", err: goalsvc.ErrGoalRevisionStale, wantStatus: http.StatusConflict},
		{name: "execution binding conflict", err: goalsvc.ErrGoalExecutionBindingConflict, wantStatus: http.StatusConflict},
		{name: "goal conflict", err: goalsvc.ErrGoalConflict, wantStatus: http.StatusConflict},
		{name: "invalid state", err: goalsvc.ErrGoalInvalidState, wantStatus: http.StatusUnprocessableEntity},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.writeGoalError(response, test.err)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d body=%s, want %d", response.Code, response.Body.String(), test.wantStatus)
			}
		})
	}
}
