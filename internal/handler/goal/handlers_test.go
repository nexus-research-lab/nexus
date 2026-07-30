package goal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nexus-research-lab/nexus/internal/config"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

type emptyGoalRepository struct{}

type staticGoalRepository struct {
	emptyGoalRepository
	item protocol.Goal
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
