// INPUT: authenticated Goal HTTP create/read/update/lifecycle requests and user-controlled metadata.
// OUTPUT: owner-scoped Goal responses plus a server-derived binding read view; client-owned metadata never decides WorkGraph state.
// POS: Goal HTTP transport adapter; binding lifecycle authority remains in service/goal and orchestration.
package goal

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
)

// Handlers 封装 Goal HTTP handlers。
type Handlers struct {
	api   *handlershared.API
	goals *goalsvc.Service
}

// New 创建 Goal handlers。
func New(api *handlershared.API, goals *goalsvc.Service) *Handlers {
	return &Handlers{api: api, goals: goals}
}

// HandleGetCurrentGoal 返回 session 当前 Goal。
func (h *Handlers) HandleGetCurrentGoal(writer http.ResponseWriter, request *http.Request) {
	goal, err := h.goals.CurrentOptionalForOwner(
		request.Context(),
		request.URL.Query().Get("session_key"),
		authsvc.OwnerUserID(request.Context()),
	)
	if err != nil {
		h.writeGoalError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, goal)
}

// HandleGetGoalUsage 返回指定 Goal 的聚合 usage 与 finalization fence。
func (h *Handlers) HandleGetGoalUsage(writer http.ResponseWriter, request *http.Request) {
	report, err := h.goals.UsageByGoalIDForOwner(
		request.Context(),
		chi.URLParam(request, "goal_id"),
		authsvc.OwnerUserID(request.Context()),
	)
	if err != nil {
		h.writeGoalError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, report)
}

// HandleGetGoalExecutionBinding returns the database-derived binding state for
// the authenticated Goal owner. Reservation provenance remains server-only;
// execution_id is exposed only for one confirmed exact bilateral binding.
func (h *Handlers) HandleGetGoalExecutionBinding(
	writer http.ResponseWriter,
	request *http.Request,
) {
	resolution, err := h.goals.ExecutionBindingForOwner(
		request.Context(),
		chi.URLParam(request, "goal_id"),
		authsvc.OwnerUserID(request.Context()),
	)
	if err != nil {
		h.writeGoalError(writer, err)
		return
	}
	view := protocol.GoalExecutionBindingView{State: resolution.State}
	if resolution.State == protocol.GoalExecutionBindingStateConfirmed {
		view.ExecutionID = resolution.ExecutionID
	}
	h.api.WriteSuccess(writer, view)
}

// HandleCreateGoal 创建当前 Goal。
func (h *Handlers) HandleCreateGoal(writer http.ResponseWriter, request *http.Request) {
	var input protocol.CreateGoalRequest
	if !h.api.BindJSON(writer, request, &input) {
		return
	}
	input.CreatedBy = "user"
	input.AgentID = ""
	input.OwnerUserID = authsvc.OwnerUserID(request.Context())
	goal, err := h.goals.Create(request.Context(), input)
	if err != nil {
		h.writeGoalError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, goal)
}

// HandleUpdateGoal 更新当前 Goal。
func (h *Handlers) HandleUpdateGoal(writer http.ResponseWriter, request *http.Request) {
	var input protocol.UpdateGoalRequest
	if !h.api.BindJSON(writer, request, &input) {
		return
	}
	input.OwnerUserID = authsvc.OwnerUserID(request.Context())
	goal, err := h.goals.Update(request.Context(), chi.URLParam(request, "goal_id"), input)
	if err != nil {
		h.writeGoalError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, goal)
}

// HandlePauseGoal 暂停 Goal。
func (h *Handlers) HandlePauseGoal(writer http.ResponseWriter, request *http.Request) {
	goal, err := h.goals.Pause(request.Context(), chi.URLParam(request, "goal_id"))
	if err != nil {
		h.writeGoalError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, goal)
}

// HandleResumeGoal 恢复 Goal。
func (h *Handlers) HandleResumeGoal(writer http.ResponseWriter, request *http.Request) {
	goal, err := h.goals.Resume(request.Context(), chi.URLParam(request, "goal_id"))
	if err != nil {
		h.writeGoalError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, goal)
}

// HandleClearGoal 清除 Goal。
func (h *Handlers) HandleClearGoal(writer http.ResponseWriter, request *http.Request) {
	cleared, err := h.goals.Clear(request.Context(), chi.URLParam(request, "goal_id"))
	if err != nil {
		h.writeGoalError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, goalappserver.ThreadGoalClearResponse{Cleared: cleared})
}

// HandleGoalEvents 返回 Goal 审计事件。
func (h *Handlers) HandleGoalEvents(writer http.ResponseWriter, request *http.Request) {
	events, err := h.goals.EventsForOwner(
		request.Context(),
		chi.URLParam(request, "goal_id"),
		50,
		authsvc.OwnerUserID(request.Context()),
	)
	if err != nil {
		h.writeGoalError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, events)
}

func (h *Handlers) writeGoalError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, goalsvc.ErrGoalDisabled):
		h.api.WriteFailure(writer, http.StatusForbidden, "Goal 功能未启用")
	case errors.Is(err, goalsvc.ErrGoalForbidden):
		h.api.WriteFailure(writer, http.StatusForbidden, "当前用户无权访问或修改该 Goal")
	case errors.Is(err, goalsvc.ErrGoalNotFound):
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
	case errors.Is(err, goalsvc.ErrGoalConflict),
		errors.Is(err, goalsvc.ErrGoalVersionStale),
		errors.Is(err, goalsvc.ErrGoalRevisionStale),
		errors.Is(err, goalsvc.ErrGoalExecutionBindingConflict):
		h.api.WriteFailure(writer, http.StatusConflict, "请求冲突")
	case errors.Is(err, goalsvc.ErrGoalInvalidInput), errors.Is(err, goalsvc.ErrGoalInvalidState):
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, "请求无效")
	default:
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
	}
}
