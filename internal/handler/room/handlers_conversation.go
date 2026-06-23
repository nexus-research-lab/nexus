package room

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	roompkg "github.com/nexus-research-lab/nexus/internal/service/room"
	sessionpkg "github.com/nexus-research-lab/nexus/internal/service/session"

	"github.com/go-chi/chi/v5"
)

// HandleConversationMessages 返回会话消息分页。
func (h *Handlers) HandleConversationMessages(writer http.ResponseWriter, request *http.Request) {
	roomID := chi.URLParam(request, "room_id")
	conversationID := chi.URLParam(request, "conversation_id")
	limit := 0
	if rawLimit := strings.TrimSpace(request.URL.Query().Get("limit")); rawLimit != "" {
		parsedLimit, parseErr := strconv.Atoi(rawLimit)
		if parseErr != nil || parsedLimit <= 0 {
			h.api.WriteFailure(writer, http.StatusBadRequest, "limit 参数错误")
			return
		}
		limit = parsedLimit
	}
	beforeRoundID := strings.TrimSpace(request.URL.Query().Get("before_round_id"))
	beforeRoundTimestamp := int64(0)
	if rawBeforeTimestamp := strings.TrimSpace(request.URL.Query().Get("before_round_timestamp")); rawBeforeTimestamp != "" {
		parsedBeforeTimestamp, parseErr := strconv.ParseInt(rawBeforeTimestamp, 10, 64)
		if parseErr != nil || parsedBeforeTimestamp <= 0 {
			h.api.WriteFailure(writer, http.StatusBadRequest, "before_round_timestamp 参数错误")
			return
		}
		beforeRoundTimestamp = parsedBeforeTimestamp
	}

	contextValue, err := h.roomService.GetConversationContext(request.Context(), conversationID)
	if errors.Is(err, roompkg.ErrConversationNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	if contextValue.Room.ID != roomID {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}

	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	if contextValue.Room.RoomType == protocol.RoomTypeDM {
		primarySession := findPrimaryConversationSession(contextValue.Sessions)
		if primarySession == nil || strings.TrimSpace(primarySession.AgentID) == "" {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		sessionKey = protocol.BuildRoomAgentSessionKey(
			conversationID,
			strings.TrimSpace(primarySession.AgentID),
			protocol.RoomTypeDM,
		)
	}

	items, err := h.sessions.GetSessionMessagesPage(request.Context(), sessionKey, sessionpkg.MessagePageRequest{
		Limit:                limit,
		BeforeRoundID:        beforeRoundID,
		BeforeRoundTimestamp: beforeRoundTimestamp,
	})
	if handlershared.IsStructuredSessionKeyError(err) {
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, items)
}

// HandleUploadConversationAttachment 上传 Room conversation 级公共附件。
func (h *Handlers) HandleUploadConversationAttachment(writer http.ResponseWriter, request *http.Request) {
	file, header, err := request.FormFile("file")
	if err != nil {
		h.api.WriteFailure(writer, http.StatusBadRequest, "缺少上传文件")
		return
	}
	defer file.Close()

	item, err := h.roomService.UploadConversationAttachment(
		request.Context(),
		chi.URLParam(request, "room_id"),
		chi.URLParam(request, "conversation_id"),
		header.Filename,
		request.FormValue("path"),
		file,
	)
	if errors.Is(err, roompkg.ErrRoomNotFound) || errors.Is(err, roompkg.ErrConversationNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "路径") ||
			strings.Contains(err.Error(), "限制") ||
			strings.Contains(err.Error(), "DM conversation") {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

func findPrimaryConversationSession(sessions []protocol.SessionRecord) *protocol.SessionRecord {
	for index := range sessions {
		if sessions[index].IsPrimary {
			return &sessions[index]
		}
	}
	if len(sessions) == 0 {
		return nil
	}
	return &sessions[0]
}

// HandleCreateConversation 创建 conversation。
func (h *Handlers) HandleCreateConversation(writer http.ResponseWriter, request *http.Request) {
	var payload protocol.CreateConversationRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.roomService.CreateConversation(request.Context(), chi.URLParam(request, "room_id"), payload)
	if errors.Is(err, roompkg.ErrRoomNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if handlershared.IsClientMessageError(err) {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.broadcastRoomResync(request.Context(), item.Room.ID, item.Conversation.ID, "conversation_created")
	h.api.WriteSuccess(writer, item)
}

// HandleUpdateConversation 更新 conversation。
func (h *Handlers) HandleUpdateConversation(writer http.ResponseWriter, request *http.Request) {
	var payload protocol.UpdateConversationRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.roomService.UpdateConversation(
		request.Context(),
		chi.URLParam(request, "room_id"),
		chi.URLParam(request, "conversation_id"),
		payload,
	)
	if errors.Is(err, roompkg.ErrRoomNotFound) || errors.Is(err, roompkg.ErrConversationNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if handlershared.IsClientMessageError(err) {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.broadcastRoomResync(request.Context(), item.Room.ID, item.Conversation.ID, "conversation_updated")
	h.api.WriteSuccess(writer, item)
}

// HandleDeleteConversation 删除 conversation。
func (h *Handlers) HandleDeleteConversation(writer http.ResponseWriter, request *http.Request) {
	roomID := chi.URLParam(request, "room_id")
	conversationID := chi.URLParam(request, "conversation_id")
	if h.roomRealtime != nil {
		_ = h.roomRealtime.InterruptConversation(request.Context(), conversationID, "对话已删除")
	}
	item, err := h.roomService.DeleteConversation(request.Context(), roomID, conversationID)
	if errors.Is(err, roompkg.ErrRoomNotFound) || errors.Is(err, roompkg.ErrConversationNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if handlershared.IsClientMessageError(err) {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.broadcastRoomResync(request.Context(), roomID, conversationID, "conversation_deleted")
	h.api.WriteSuccess(writer, item)
}

// HandleCloseConversationRuntime 关闭 conversation 标签页背后的运行态。
func (h *Handlers) HandleCloseConversationRuntime(writer http.ResponseWriter, request *http.Request) {
	roomID := chi.URLParam(request, "room_id")
	conversationID := chi.URLParam(request, "conversation_id")
	if h.roomRealtime != nil {
		_ = h.roomRealtime.InterruptConversation(request.Context(), conversationID, "对话标签页已关闭")
	}
	err := h.roomService.CloseConversationRuntime(request.Context(), roomID, conversationID)
	if errors.Is(err, roompkg.ErrRoomNotFound) || errors.Is(err, roompkg.ErrConversationNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, map[string]bool{"closed": true})
}
