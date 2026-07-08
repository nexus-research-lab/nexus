package goal

import (
	"encoding/json"
	"net/http"

	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
)

// HandleThreadGoalSet 提供 Codex app-server 风格的 thread/goal/set 兼容入口。
func (h *Handlers) HandleThreadGoalSet(writer http.ResponseWriter, request *http.Request) {
	var input goalappserver.ThreadGoalSetParams
	if !h.api.BindJSON(writer, request, &input) {
		return
	}
	item, err := h.goals.SetFromThreadGoalParams(goalsvc.WithActiveGoalContinuationSuppressed(request.Context()), input)
	if err != nil {
		h.writeGoalError(writer, err)
		return
	}
	h.writeCodexGoalJSON(writer, goalappserver.ThreadGoalSetResponse{
		Goal: goalappserver.ThreadGoalFromGoal(*item),
	})
	h.goals.DispatchActiveGoalContinuation(request.Context(), *item)
}

// HandleThreadGoalGet 提供 Codex app-server 风格的 thread/goal/get 兼容入口。
func (h *Handlers) HandleThreadGoalGet(writer http.ResponseWriter, request *http.Request) {
	var input goalappserver.ThreadGoalGetParams
	if !h.api.BindJSON(writer, request, &input) {
		return
	}
	item, err := h.goals.CurrentOptional(request.Context(), input.ThreadID)
	if err != nil {
		h.writeGoalError(writer, err)
		return
	}
	h.writeCodexGoalJSON(writer, goalappserver.ThreadGoalGetResponse{
		Goal: goalappserver.ThreadGoalPointerFromGoal(item),
	})
}

// HandleThreadGoalClear 提供 Codex app-server 风格的 thread/goal/clear 兼容入口。
func (h *Handlers) HandleThreadGoalClear(writer http.ResponseWriter, request *http.Request) {
	var input goalappserver.ThreadGoalClearParams
	if !h.api.BindJSON(writer, request, &input) {
		return
	}
	cleared, err := h.goals.ClearFromThreadGoalParams(request.Context(), input)
	if err != nil {
		h.writeGoalError(writer, err)
		return
	}
	h.writeCodexGoalJSON(writer, goalappserver.ThreadGoalClearResponse{Cleared: cleared})
}

func (h *Handlers) writeCodexGoalJSON(writer http.ResponseWriter, payload any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(payload)
}
