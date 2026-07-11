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
	pageRequest, err := conversationMessagePageRequest(request)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	sessionKey, ok := h.resolveConversationSessionKey(writer, request, roomID, conversationID)
	if !ok {
		return
	}
	items, err := h.sessions.GetSessionMessagesPage(request.Context(), sessionKey, pageRequest)
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

func conversationMessagePageRequest(request *http.Request) (sessionpkg.MessagePageRequest, error) {
	limit, err := positiveQueryInt(request, "limit")
	if err != nil {
		return sessionpkg.MessagePageRequest{}, err
	}
	beforeRoundTimestamp, err := positiveQueryInt64(request, "before_round_timestamp")
	if err != nil {
		return sessionpkg.MessagePageRequest{}, err
	}
	aroundLimit, err := positiveQueryInt(request, "around_limit")
	if err != nil {
		return sessionpkg.MessagePageRequest{}, err
	}
	return sessionpkg.MessagePageRequest{
		Limit:                limit,
		BeforeRoundID:        strings.TrimSpace(request.URL.Query().Get("before_round_id")),
		BeforeRoundTimestamp: beforeRoundTimestamp,
		AroundRoundID:        strings.TrimSpace(request.URL.Query().Get("around_round_id")),
		AroundLimit:          aroundLimit,
	}, nil
}

func positiveQueryInt(request *http.Request, name string) (int, error) {
	raw := strings.TrimSpace(request.URL.Query().Get(name))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, errors.New(name + " 参数错误")
	}
	return value, nil
}

func positiveQueryInt64(request *http.Request, name string) (int64, error) {
	raw := strings.TrimSpace(request.URL.Query().Get(name))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, errors.New(name + " 参数错误")
	}
	return value, nil
}

// HandleConversationTurns 返回 Room conversation 的 ConversationTurn 分页。
func (h *Handlers) HandleConversationTurns(writer http.ResponseWriter, request *http.Request) {
	roomID := chi.URLParam(request, "room_id")
	conversationID := chi.URLParam(request, "conversation_id")
	limit, err := positiveQueryInt(request, "limit")
	if err != nil {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}

	sessionKey, ok := h.resolveConversationSessionKey(writer, request, roomID, conversationID)
	if !ok {
		return
	}
	page, err := h.sessions.GetSessionTurnsPage(request.Context(), sessionKey, sessionpkg.TurnPageRequest{
		Limit:         limit,
		BeforeRoundID: strings.TrimSpace(request.URL.Query().Get("before_round_id")),
		AroundRoundID: strings.TrimSpace(request.URL.Query().Get("around_round_id")),
		Sort:          strings.TrimSpace(request.URL.Query().Get("sort")),
		View:          strings.TrimSpace(request.URL.Query().Get("view")),
	})
	if handlershared.IsStructuredSessionKeyError(err) {
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, page)
}

// resolveConversationSessionKey 校验 conversation 归属并解析历史读取用 session key。
func (h *Handlers) resolveConversationSessionKey(
	writer http.ResponseWriter,
	request *http.Request,
	roomID string,
	conversationID string,
) (string, bool) {
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
