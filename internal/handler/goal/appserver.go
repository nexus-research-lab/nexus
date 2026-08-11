// INPUT: 已认证 owner、Codex app-server thread/goal JSON 与 Goal service。
// OUTPUT: owner-scoped set/get/clear 兼容响应，并仅在持久化成功后调度 active Goal continuation。
// POS: HTTP app-server Goal transport；不信任客户端 owner、binding 或 Room 身份字段。
package goal

import (
	"encoding/json"
	"net/http"

	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
)

// HandleThreadGoalSet 提供 Codex app-server 风格的 thread/goal/set 兼容入口。
func (h *Handlers) HandleThreadGoalSet(writer http.ResponseWriter, request *http.Request) {
	var input goalappserver.ThreadGoalSetParams
	if !h.api.BindJSON(writer, request, &input) {
		return
	}
	input.OwnerUserID = authsvc.OwnerUserID(request.Context())
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
	item, err := h.goals.CurrentOptionalForOwner(
		request.Context(),
		input.ThreadID,
		authsvc.OwnerUserID(request.Context()),
	)
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
	input.OwnerUserID = authsvc.OwnerUserID(request.Context())
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
