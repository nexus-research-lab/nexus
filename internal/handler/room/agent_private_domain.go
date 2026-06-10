package room

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	roompkg "github.com/nexus-research-lab/nexus/internal/service/room"

	"github.com/go-chi/chi/v5"
)

// HandleListAgentPrivateThreads 返回指定 Agent 的全局私域线程列表。
func (h *Handlers) HandleListAgentPrivateThreads(writer http.ResponseWriter, request *http.Request) {
	query, ok := h.parseAgentPrivateDomainQuery(writer, request)
	if !ok {
		return
	}
	item, err := h.roomService.ListAgentPrivateThreads(
		request.Context(),
		chi.URLParam(request, "agent_id"),
		query,
	)
	if errors.Is(err, agentpkg.ErrAgentNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if errors.Is(err, roompkg.ErrRoomNotFound) || errors.Is(err, roompkg.ErrConversationNotFound) {
		h.api.WriteSuccess(writer, protocol.AgentPrivateThreadPage{Items: []protocol.AgentPrivateThread{}})
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleListAgentPrivateEvents 返回指定私域线程内的可见事件。
func (h *Handlers) HandleListAgentPrivateEvents(writer http.ResponseWriter, request *http.Request) {
	query, ok := h.parseAgentPrivateDomainQuery(writer, request)
	if !ok {
		return
	}
	item, err := h.roomService.ListAgentPrivateEvents(
		request.Context(),
		chi.URLParam(request, "agent_id"),
		chi.URLParam(request, "thread_id"),
		query,
	)
	if errors.Is(err, agentpkg.ErrAgentNotFound) ||
		errors.Is(err, roompkg.ErrPrivateThreadNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) parseAgentPrivateDomainQuery(
	writer http.ResponseWriter,
	request *http.Request,
) (roompkg.AgentPrivateDomainQuery, bool) {
	query := roompkg.AgentPrivateDomainQuery{
		RoomID:         strings.TrimSpace(request.URL.Query().Get("room_id")),
		ConversationID: strings.TrimSpace(request.URL.Query().Get("conversation_id")),
	}
	if raw := strings.TrimSpace(request.URL.Query().Get("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit <= 0 {
			h.api.WriteFailure(writer, http.StatusBadRequest, "limit 参数错误")
			return roompkg.AgentPrivateDomainQuery{}, false
		}
		query.Limit = limit
	}
	if raw := strings.TrimSpace(request.URL.Query().Get("room_limit")); raw != "" {
		roomLimit, err := strconv.Atoi(raw)
		if err != nil || roomLimit <= 0 {
			h.api.WriteFailure(writer, http.StatusBadRequest, "room_limit 参数错误")
			return roompkg.AgentPrivateDomainQuery{}, false
		}
		query.RoomLimit = roomLimit
	}
	return query, true
}
