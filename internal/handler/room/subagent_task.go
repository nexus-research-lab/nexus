package room

import (
	"errors"
	"net/http"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	roompkg "github.com/nexus-research-lab/nexus/internal/service/room"
	sessionpkg "github.com/nexus-research-lab/nexus/internal/service/session"

	"github.com/go-chi/chi/v5"
)

type subagentTaskMessageRequest struct {
	Message string `json:"message"`
}

// HandleConversationSubagentTasks 返回 conversation 中的后台 subagent task。
func (h *Handlers) HandleConversationSubagentTasks(writer http.ResponseWriter, request *http.Request) {
	sessionKey, ok := h.resolveConversationTaskSessionKey(writer, request)
	if !ok {
		return
	}
	list, err := h.sessions.ListSubagentTasks(request.Context(), sessionKey)
	if err != nil {
		h.writeConversationSubagentTaskError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, list)
}

// HandleConversationSubagentTaskMessages 返回 conversation 中 subagent task 的只读 transcript。
func (h *Handlers) HandleConversationSubagentTaskMessages(writer http.ResponseWriter, request *http.Request) {
	sessionKey, ok := h.resolveConversationTaskSessionKey(writer, request)
	if !ok {
		return
	}
	taskID := strings.TrimSpace(chi.URLParam(request, "task_id"))
	item, err := h.sessions.GetSubagentTaskMessages(request.Context(), sessionKey, taskID)
	if err != nil {
		h.writeConversationSubagentTaskError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleStopConversationSubagentTask 停止 conversation 中的后台 subagent task。
func (h *Handlers) HandleStopConversationSubagentTask(writer http.ResponseWriter, request *http.Request) {
	sessionKey, ok := h.resolveConversationTaskSessionKey(writer, request)
	if !ok {
		return
	}
	taskID := strings.TrimSpace(chi.URLParam(request, "task_id"))
	result, err := h.sessions.StopSubagentTask(request.Context(), sessionKey, taskID)
	if err != nil {
		h.writeConversationSubagentTaskError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, result)
}

// HandleSendConversationSubagentTaskMessage 向 conversation 中的 subagent task 发送后续消息。
func (h *Handlers) HandleSendConversationSubagentTaskMessage(writer http.ResponseWriter, request *http.Request) {
	sessionKey, ok := h.resolveConversationTaskSessionKey(writer, request)
	if !ok {
		return
	}
	taskID := strings.TrimSpace(chi.URLParam(request, "task_id"))
	var payload subagentTaskMessageRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	result, err := h.sessions.SendSubagentTaskMessage(request.Context(), sessionKey, taskID, payload.Message)
	if err != nil {
		h.writeConversationSubagentTaskError(writer, err)
		return
	}
	h.api.WriteSuccess(writer, result)
}

func (h *Handlers) resolveConversationTaskSessionKey(writer http.ResponseWriter, request *http.Request) (string, bool) {
	roomID := chi.URLParam(request, "room_id")
	conversationID := chi.URLParam(request, "conversation_id")
	contextValue, err := h.roomService.GetConversationContext(request.Context(), conversationID)
	if errors.Is(err, roompkg.ErrConversationNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return "", false
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return "", false
	}
	if contextValue.Room.ID != roomID {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return "", false
	}

	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	if contextValue.Room.RoomType == protocol.RoomTypeDM {
		primarySession := findPrimaryConversationSession(contextValue.Sessions)
		if primarySession == nil || strings.TrimSpace(primarySession.AgentID) == "" {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return "", false
		}
		sessionKey = protocol.BuildRoomAgentSessionKey(
			conversationID,
			strings.TrimSpace(primarySession.AgentID),
			protocol.RoomTypeDM,
		)
	}
	return sessionKey, true
}

func (h *Handlers) writeConversationSubagentTaskError(writer http.ResponseWriter, err error) {
	if handlershared.IsStructuredSessionKeyError(err) {
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if errors.Is(err, sessionpkg.ErrSessionNotFound) || errors.Is(err, sessionpkg.ErrSubagentTaskNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if errors.Is(err, sessionpkg.ErrSubagentOperationUnsupported) {
		h.api.WriteJSON(writer, http.StatusConflict, map[string]any{
			"code":    "subagent_operation_unsupported",
			"message": "failed",
			"success": false,
			"data": map[string]any{
				"detail": "当前运行时不支持该操作",
			},
		})
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
