package room

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	roompkg "github.com/nexus-research-lab/nexus/internal/service/room"
	sessionpkg "github.com/nexus-research-lab/nexus/internal/service/session"

	"github.com/go-chi/chi/v5"
)

type roomEventBroadcaster func(context.Context, string, protocol.EventType, map[string]any)
type roomResyncBroadcaster func(context.Context, string, string, string)
type roomRegistryRemover func(string)
type directoryBroadcaster func(context.Context, string, map[string]any)

// Handlers 封装 room 域 HTTP handlers。
type Handlers struct {
	api                   *handlershared.API
	roomService           *roompkg.Service
	roomRealtime          *roompkg.RealtimeService
	sessions              *sessionpkg.Service
	broadcastRoomEvent    roomEventBroadcaster
	broadcastRoomResync   roomResyncBroadcaster
	broadcastDirectory    directoryBroadcaster
	removeRoomSubscribers roomRegistryRemover
}

// New 创建 room 域 handlers。
func New(
	api *handlershared.API,
	roomService *roompkg.Service,
	roomRealtime *roompkg.RealtimeService,
	sessions *sessionpkg.Service,
	broadcastRoomEvent roomEventBroadcaster,
	broadcastRoomResync roomResyncBroadcaster,
	removeRoomSubscribers roomRegistryRemover,
	broadcastDirectory ...directoryBroadcaster,
) *Handlers {
	var directory directoryBroadcaster
	if len(broadcastDirectory) > 0 {
		directory = broadcastDirectory[0]
	}
	return &Handlers{
		api:                   api,
		roomService:           roomService,
		roomRealtime:          roomRealtime,
		sessions:              sessions,
		broadcastRoomEvent:    broadcastRoomEvent,
		broadcastRoomResync:   broadcastRoomResync,
		broadcastDirectory:    directory,
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

func (h *Handlers) broadcastDirectoryChanged(ctx context.Context, reason string, data map[string]any) {
	if h.broadcastDirectory == nil {
		return
	}
	h.broadcastDirectory(ctx, reason, data)
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
		if handlershared.IsClientMessageError(err) {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.broadcastDirectoryChanged(request.Context(), "room_created", map[string]any{
		"room_id":         item.Room.ID,
		"conversation_id": item.Conversation.ID,
	})
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
		if handlershared.IsClientMessageError(err) {
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
		if handlershared.IsClientMessageError(err) {
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
		if handlershared.IsClientMessageError(err) {
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
		if handlershared.IsClientMessageError(err) {
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
