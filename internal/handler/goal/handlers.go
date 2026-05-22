package goal

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
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
	goal, err := h.goals.Current(request.Context(), request.URL.Query().Get("session_key"))
	if err != nil {
		h.writeGoalError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, goal)
}

// HandleCreateGoal 创建当前 Goal。
func (h *Handlers) HandleCreateGoal(writer http.ResponseWriter, request *http.Request) {
	var input protocol.CreateGoalRequest
	if !h.api.BindJSON(writer, request, &input) {
		return
	}
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
	goal, err := h.goals.Clear(request.Context(), chi.URLParam(request, "goal_id"))
	if err != nil {
		h.writeGoalError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, goal)
}

// HandleCompleteGoal 标记 Goal 完成。
func (h *Handlers) HandleCompleteGoal(writer http.ResponseWriter, request *http.Request) {
	var input protocol.CompleteGoalRequest
	if !h.api.BindJSONAllowEmpty(writer, request, &input) {
		return
	}
	goal, err := h.goals.CompleteByModel(request.Context(), chi.URLParam(request, "goal_id"), input)
	if err != nil {
		h.writeGoalError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, goal)
}

// HandleBlockGoal 标记 Goal 阻塞。
func (h *Handlers) HandleBlockGoal(writer http.ResponseWriter, request *http.Request) {
	var input protocol.BlockGoalRequest
	if !h.api.BindJSON(writer, request, &input) {
		return
	}
	goal, err := h.goals.BlockByModel(request.Context(), chi.URLParam(request, "goal_id"), input)
	if err != nil {
		h.writeGoalError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, goal)
}

// HandleGoalEvents 返回 Goal 审计事件。
func (h *Handlers) HandleGoalEvents(writer http.ResponseWriter, request *http.Request) {
	events, err := h.goals.Events(request.Context(), chi.URLParam(request, "goal_id"), 50)
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
	case errors.Is(err, goalsvc.ErrGoalNotFound):
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
	case errors.Is(err, goalsvc.ErrGoalConflict), errors.Is(err, goalsvc.ErrGoalVersionStale):
		h.api.WriteFailure(writer, http.StatusConflict, "请求冲突")
	case errors.Is(err, goalsvc.ErrGoalInvalidInput), errors.Is(err, goalsvc.ErrGoalInvalidState):
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, "请求无效")
	default:
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
	}
}
