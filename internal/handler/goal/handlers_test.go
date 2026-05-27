package goal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

type emptyGoalRepository struct{}

func (emptyGoalRepository) CreateGoal(context.Context, protocol.Goal) (*protocol.Goal, error) {
	return nil, nil
}

func (emptyGoalRepository) GetGoal(context.Context, string) (*protocol.Goal, error) {
	return nil, nil
}

func (emptyGoalRepository) GetCurrentGoal(context.Context, string) (*protocol.Goal, error) {
	return nil, nil
}

func (emptyGoalRepository) ListRunnableGoals(context.Context, int) ([]protocol.Goal, error) {
	return nil, nil
}

func (emptyGoalRepository) UpdateGoal(context.Context, protocol.Goal, int64) (*protocol.Goal, error) {
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
