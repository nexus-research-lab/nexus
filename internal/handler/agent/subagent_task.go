package agent

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	sessionpkg "github.com/nexus-research-lab/nexus/internal/service/session"

	"github.com/go-chi/chi/v5"
)

type subagentTaskMessageRequest struct {
	Message string `json:"message"`
}

// HandleSessionSubagentTasks 返回 session 中的后台 subagent task。
func (h *Handlers) HandleSessionSubagentTasks(writer http.ResponseWriter, request *http.Request) {
	sessionKey := sessionTaskSessionKeyParam(request)
	items, err := h.sessions.ListSubagentTasks(request.Context(), sessionKey)
	if err != nil {
		h.writeSubagentTaskError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, map[string]any{"items": items})
}

// HandleSessionSubagentTaskMessages 返回 subagent task 的只读 transcript。
func (h *Handlers) HandleSessionSubagentTaskMessages(writer http.ResponseWriter, request *http.Request) {
	sessionKey := sessionTaskSessionKeyParam(request)
	taskID := strings.TrimSpace(chi.URLParam(request, "task_id"))
	item, err := h.sessions.GetSubagentTaskMessages(request.Context(), sessionKey, taskID)
	if err != nil {
		h.writeSubagentTaskError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleStopSessionSubagentTask 停止 session 中的后台 subagent task。
func (h *Handlers) HandleStopSessionSubagentTask(writer http.ResponseWriter, request *http.Request) {
	sessionKey := sessionTaskSessionKeyParam(request)
	taskID := strings.TrimSpace(chi.URLParam(request, "task_id"))
	result, err := h.sessions.StopSubagentTask(request.Context(), sessionKey, taskID)
	if err != nil {
		h.writeSubagentTaskError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, result)
}

// HandleSendSessionSubagentTaskMessage 向 session 中的 subagent task 发送后续消息。
func (h *Handlers) HandleSendSessionSubagentTaskMessage(writer http.ResponseWriter, request *http.Request) {
	sessionKey := sessionTaskSessionKeyParam(request)
	taskID := strings.TrimSpace(chi.URLParam(request, "task_id"))
	var payload subagentTaskMessageRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	result, err := h.sessions.SendSubagentTaskMessage(request.Context(), sessionKey, taskID, payload.Message)
	if err != nil {
		h.writeSubagentTaskError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, result)
}

func sessionTaskSessionKeyParam(request *http.Request) string {
	raw := strings.TrimSpace(chi.URLParam(request, "session_key"))
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return raw
	}
	return strings.TrimSpace(decoded)
}

func (h *Handlers) writeSubagentTaskError(writer http.ResponseWriter, err error) {
	if handlershared.IsStructuredSessionKeyError(err) {
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if errors.Is(err, sessionpkg.ErrSessionNotFound) || errors.Is(err, sessionpkg.ErrSubagentTaskNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if errors.Is(err, sessionpkg.ErrSubagentRuntimeUnavailable) {
		h.api.WriteFailure(writer, http.StatusConflict, "任务当前不在线或已结束")
		return
	}
	if errors.Is(err, sessionpkg.ErrSubagentTaskNotRunning) {
		h.api.WriteFailure(writer, http.StatusConflict, "任务当前不在运行中")
		return
	}
	h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
}
