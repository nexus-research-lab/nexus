package room

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	gatewayshared "github.com/nexus-research-lab/nexus/internal/gateway/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	roompkg "github.com/nexus-research-lab/nexus/internal/service/room"
	sessionpkg "github.com/nexus-research-lab/nexus/internal/service/session"

	"github.com/go-chi/chi/v5"
)

type roomEventBroadcaster func(context.Context, string, protocol.EventType, map[string]any)
type roomResyncBroadcaster func(context.Context, string, string, string)
type roomRegistryRemover func(string)

// Handlers 封装 room 域 HTTP handlers。
type Handlers struct {
	api                   *gatewayshared.API
	roomService           *roompkg.Service
	roomRealtime          *roompkg.RealtimeService
	sessions              *sessionpkg.Service
	broadcastRoomEvent    roomEventBroadcaster
	broadcastRoomResync   roomResyncBroadcaster
	removeRoomSubscribers roomRegistryRemover
}

// New 创建 room 域 handlers。
func New(
	api *gatewayshared.API,
	roomService *roompkg.Service,
	roomRealtime *roompkg.RealtimeService,
	sessions *sessionpkg.Service,
	broadcastRoomEvent roomEventBroadcaster,
	broadcastRoomResync roomResyncBroadcaster,
	removeRoomSubscribers roomRegistryRemover,
) *Handlers {
	return &Handlers{
		api:                   api,
		roomService:           roomService,
		roomRealtime:          roomRealtime,
		sessions:              sessions,
		broadcastRoomEvent:    broadcastRoomEvent,
		broadcastRoomResync:   broadcastRoomResync,
		removeRoomSubscribers: removeRoomSubscribers,
	}
}

// HandleListRooms 返回 room 列表。
func (h *Handlers) HandleListRooms(writer http.ResponseWriter, request *http.Request) {
	limit := 20
	if raw := strings.TrimSpace(request.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	items, err := h.roomService.ListRooms(request.Context(), limit)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, items)
}

// HandleGetRoom 返回单个 room。
func (h *Handlers) HandleGetRoom(writer http.ResponseWriter, request *http.Request) {
	item, err := h.roomService.GetRoom(request.Context(), chi.URLParam(request, "room_id"))
	if errors.Is(err, roompkg.ErrRoomNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleGetRoomContexts 返回 room 上下文。
func (h *Handlers) HandleGetRoomContexts(writer http.ResponseWriter, request *http.Request) {
	items, err := h.roomService.GetRoomContexts(request.Context(), chi.URLParam(request, "room_id"))
	if errors.Is(err, roompkg.ErrRoomNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, items)
}

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
	if gatewayshared.IsStructuredSessionKeyError(err) {
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, items)
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

// HandleCreateRoom 创建 room。
func (h *Handlers) HandleCreateRoom(writer http.ResponseWriter, request *http.Request) {
	var payload protocol.CreateRoomRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.roomService.CreateRoom(request.Context(), payload)
	if errors.Is(err, agentpkg.ErrAgentNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if gatewayshared.IsClientMessageError(err) {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleUpdateRoom 更新 room。
func (h *Handlers) HandleUpdateRoom(writer http.ResponseWriter, request *http.Request) {
	var payload protocol.UpdateRoomRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.roomService.UpdateRoom(request.Context(), chi.URLParam(request, "room_id"), payload)
	if errors.Is(err, roompkg.ErrRoomNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if gatewayshared.IsClientMessageError(err) {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.broadcastRoomResync(request.Context(), item.Room.ID, item.Conversation.ID, "room_updated")
	h.api.WriteSuccess(writer, item)
}

// HandleDeleteRoom 删除 room。
func (h *Handlers) HandleDeleteRoom(writer http.ResponseWriter, request *http.Request) {
	roomID := chi.URLParam(request, "room_id")
	if h.roomRealtime != nil {
		_ = h.roomRealtime.InterruptRoom(request.Context(), roomID, "room 已删除")
	}
	err := h.roomService.DeleteRoom(request.Context(), roomID)
	if errors.Is(err, roompkg.ErrRoomNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.broadcastRoomEvent(request.Context(), roomID, protocol.EventTypeRoomDeleted, map[string]any{
		"room_id": roomID,
	})
	if h.removeRoomSubscribers != nil {
		h.removeRoomSubscribers(roomID)
	}
	h.api.WriteSuccess(writer, map[string]any{"success": true})
}

// HandleEnsureDirectRoom 确保 DM room 存在。
func (h *Handlers) HandleEnsureDirectRoom(writer http.ResponseWriter, request *http.Request) {
	item, err := h.roomService.EnsureDirectRoom(request.Context(), chi.URLParam(request, "agent_id"))
	if errors.Is(err, agentpkg.ErrAgentNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if gatewayshared.IsClientMessageError(err) {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleAddRoomMember 添加成员。
func (h *Handlers) HandleAddRoomMember(writer http.ResponseWriter, request *http.Request) {
	var payload protocol.AddRoomMemberRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.roomService.AddRoomMember(request.Context(), chi.URLParam(request, "room_id"), payload)
	if errors.Is(err, roompkg.ErrRoomNotFound) || errors.Is(err, agentpkg.ErrAgentNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if gatewayshared.IsClientMessageError(err) {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.broadcastRoomEvent(request.Context(), item.Room.ID, protocol.EventTypeRoomMemberAdded, map[string]any{
		"room_id":  item.Room.ID,
		"agent_id": payload.AgentID,
	})
	h.api.WriteSuccess(writer, item)
}

// HandleRemoveRoomMember 移除成员。
func (h *Handlers) HandleRemoveRoomMember(writer http.ResponseWriter, request *http.Request) {
	roomID := chi.URLParam(request, "room_id")
	agentID := chi.URLParam(request, "agent_id")
	if h.roomRealtime != nil {
		_ = h.roomRealtime.InterruptAgentTasks(request.Context(), roomID, agentID, "成员已移出 room")
	}
	item, err := h.roomService.RemoveRoomMember(request.Context(), roomID, agentID)
	if errors.Is(err, roompkg.ErrRoomNotFound) || errors.Is(err, roompkg.ErrRoomMemberNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if gatewayshared.IsClientMessageError(err) {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.broadcastRoomEvent(request.Context(), item.Room.ID, protocol.EventTypeRoomMemberRemoved, map[string]any{
		"room_id":  item.Room.ID,
		"agent_id": agentID,
	})
	h.api.WriteSuccess(writer, item)
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
		if gatewayshared.IsClientMessageError(err) {
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
		if gatewayshared.IsClientMessageError(err) {
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
		if gatewayshared.IsClientMessageError(err) {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.broadcastRoomResync(request.Context(), roomID, conversationID, "conversation_deleted")
	h.api.WriteSuccess(writer, item)
}
