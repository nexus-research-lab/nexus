package room

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	roompkg "github.com/nexus-research-lab/nexus/internal/service/room"
	memorysvc "github.com/nexus-research-lab/nexus/internal/workspace/memory"

	"github.com/go-chi/chi/v5"
)

// HandleListConversationMemory 返回 Room conversation 的共享记忆。
func (h *Handlers) HandleListConversationMemory(writer http.ResponseWriter, request *http.Request) {
	items, err := h.roomService.ListRoomSharedMemory(
		request.Context(),
		chi.URLParam(request, "room_id"),
		chi.URLParam(request, "conversation_id"),
		intQuery(request, "limit", 200),
		splitCSV(request.URL.Query().Get("status")),
	)
	if errors.Is(err, roompkg.ErrRoomNotFound) || errors.Is(err, roompkg.ErrConversationNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, map[string]any{"items": items})
}

// HandleAddConversationMemory 手动新增 Room conversation 共享记忆。
func (h *Handlers) HandleAddConversationMemory(writer http.ResponseWriter, request *http.Request) {
	var payload memorysvc.MemoryWriteInput
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.roomService.AddRoomSharedMemory(
		request.Context(),
		chi.URLParam(request, "room_id"),
		chi.URLParam(request, "conversation_id"),
		payload,
	)
	if errors.Is(err, roompkg.ErrRoomNotFound) || errors.Is(err, roompkg.ErrConversationNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if memorysvc.IsClientError(err) {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleUpdateConversationMemory 更新 Room conversation 共享记忆。
func (h *Handlers) HandleUpdateConversationMemory(writer http.ResponseWriter, request *http.Request) {
	var payload memorysvc.MemoryWriteInput
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.roomService.UpdateRoomSharedMemory(
		request.Context(),
		chi.URLParam(request, "room_id"),
		chi.URLParam(request, "conversation_id"),
		chi.URLParam(request, "entry_id"),
		payload,
	)
	if errors.Is(err, roompkg.ErrRoomNotFound) || errors.Is(err, roompkg.ErrConversationNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if memorysvc.IsClientError(err) {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleDeleteConversationMemory 删除 Room conversation 共享记忆。
func (h *Handlers) HandleDeleteConversationMemory(writer http.ResponseWriter, request *http.Request) {
	err := h.roomService.DeleteRoomSharedMemory(
		request.Context(),
		chi.URLParam(request, "room_id"),
		chi.URLParam(request, "conversation_id"),
		chi.URLParam(request, "entry_id"),
	)
	if errors.Is(err, roompkg.ErrRoomNotFound) || errors.Is(err, roompkg.ErrConversationNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if memorysvc.IsClientError(err) {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, map[string]bool{"deleted": true})
}

// HandleConversationMemoryStats 返回 Room conversation 的共享记忆统计。
func (h *Handlers) HandleConversationMemoryStats(writer http.ResponseWriter, request *http.Request) {
	stats, err := h.roomService.RoomSharedMemoryStats(
		request.Context(),
		chi.URLParam(request, "room_id"),
		chi.URLParam(request, "conversation_id"),
	)
	if errors.Is(err, roompkg.ErrRoomNotFound) || errors.Is(err, roompkg.ErrConversationNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, stats)
}

func intQuery(request *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(request.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			items = append(items, value)
		}
	}
	return items
}
