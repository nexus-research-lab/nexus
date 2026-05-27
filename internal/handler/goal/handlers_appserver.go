package goal

import (
	"encoding/json"
	"net/http"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

// HandleThreadGoalSet 提供 Codex app-server 风格的 thread/goal/set 兼容入口。
func (h *Handlers) HandleThreadGoalSet(writer http.ResponseWriter, request *http.Request) {
	var input protocol.ThreadGoalSetParams
	if !h.api.BindJSON(writer, request, &input) {
		return
	}
	item, err := h.goals.SetFromThreadGoalParams(goalsvc.WithActiveGoalContinuationSuppressed(request.Context()), input)
	if err != nil {
		h.writeGoalError(writer, err)
		return
	}
	h.writeCodexGoalJSON(writer, protocol.ThreadGoalSetResponse{
		Goal: protocol.ThreadGoalFromGoal(*item),
	})
	h.goals.DispatchActiveGoalContinuation(request.Context(), *item)
}

// HandleThreadGoalGet 提供 Codex app-server 风格的 thread/goal/get 兼容入口。
func (h *Handlers) HandleThreadGoalGet(writer http.ResponseWriter, request *http.Request) {
	var input protocol.ThreadGoalGetParams
	if !h.api.BindJSON(writer, request, &input) {
		return
	}
	item, err := h.goals.CurrentOptional(request.Context(), input.ThreadID)
	if err != nil {
		h.writeGoalError(writer, err)
		return
	}
	h.writeCodexGoalJSON(writer, protocol.ThreadGoalGetResponse{
		Goal: protocol.ThreadGoalPointerFromGoal(item),
	})
}

// HandleThreadGoalClear 提供 Codex app-server 风格的 thread/goal/clear 兼容入口。
func (h *Handlers) HandleThreadGoalClear(writer http.ResponseWriter, request *http.Request) {
	var input protocol.ThreadGoalClearParams
	if !h.api.BindJSON(writer, request, &input) {
		return
	}
	cleared, err := h.goals.ClearFromThreadGoalParams(request.Context(), input)
	if err != nil {
		h.writeGoalError(writer, err)
		return
	}
	h.writeCodexGoalJSON(writer, protocol.ThreadGoalClearResponse{Cleared: cleared})
}

func (h *Handlers) writeCodexGoalJSON(writer http.ResponseWriter, payload any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(payload)
}
