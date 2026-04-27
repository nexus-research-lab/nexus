package agent

import (
	"errors"
	"net/http"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	roompkg "github.com/nexus-research-lab/nexus/internal/service/room"
	sessionpkg "github.com/nexus-research-lab/nexus/internal/service/session"

	"github.com/go-chi/chi/v5"
)

// Handlers 封装 Agent / Session 域 HTTP handlers。
type Handlers struct {
	api          *handlershared.API
	agents       *agentpkg.Service
	sessions     *sessionpkg.Service
	runtime      *runtimectx.Manager
	roomRealtime *roompkg.RealtimeService
	prefs        *preferencessvc.Service
}

// New 创建 Agent / Session 域 handlers。
func New(
	api *handlershared.API,
	agents *agentpkg.Service,
	sessions *sessionpkg.Service,
	runtime *runtimectx.Manager,
	roomRealtime *roompkg.RealtimeService,
	prefs ...*preferencessvc.Service,
) *Handlers {
	var prefService *preferencessvc.Service
	if len(prefs) > 0 {
		prefService = prefs[0]
	}
	return &Handlers{
		api:          api,
		agents:       agents,
		sessions:     sessions,
		runtime:      runtime,
		roomRealtime: roomRealtime,
		prefs:        prefService,
	}
}

// HandleListAgents 返回 agent 列表。
func (h *Handlers) HandleListAgents(writer http.ResponseWriter, request *http.Request) {
	agents, err := h.agents.ListAgents(request.Context())
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, agents)
}

// HandleAgentRuntimeStatuses 返回 agent 运行态列表。
func (h *Handlers) HandleAgentRuntimeStatuses(writer http.ResponseWriter, request *http.Request) {
	agents, err := h.agents.ListAgentRecords(request.Context())
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	statuses := make([]map[string]any, 0, len(agents))
	for _, item := range agents {
		runningCount := 0
		if h.runtime != nil {
			runningCount += h.runtime.CountRunningRounds(item.AgentID)
		}
		if h.roomRealtime != nil {
			runningCount += h.roomRealtime.CountRunningTasks(item.AgentID)
		}
		status := "idle"
		if runningCount > 0 {
			status = "running"
		}
		statuses = append(statuses, map[string]any{
			"agent_id":           item.AgentID,
			"running_task_count": runningCount,
			"status":             status,
		})
	}
	h.api.WriteSuccess(writer, statuses)
}

// HandleGetAgent 返回单个 agent。
func (h *Handlers) HandleGetAgent(writer http.ResponseWriter, request *http.Request) {
	agentID := chi.URLParam(request, "agent_id")
	agentValue, err := h.agents.GetAgent(request.Context(), agentID)
	if errors.Is(err, agentpkg.ErrAgentNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, agentValue)
}

// HandleValidateAgentName 校验 agent 名称。
func (h *Handlers) HandleValidateAgentName(writer http.ResponseWriter, request *http.Request) {
	name := request.URL.Query().Get("name")
	excludeAgentID := request.URL.Query().Get("exclude_agent_id")
	result, err := h.agents.ValidateName(request.Context(), name, excludeAgentID)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, result)
}

// HandleCreateAgent 创建 agent。
func (h *Handlers) HandleCreateAgent(writer http.ResponseWriter, request *http.Request) {
	var payload protocol.CreateRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	if payload.Options == nil && h.prefs != nil {
		prefs, prefErr := h.prefs.Get(request.Context(), currentOwnerUserID(request))
		if prefErr != nil {
			h.api.WriteFailure(writer, http.StatusInternalServerError, prefErr.Error())
			return
		}
		payload.Options = &prefs.DefaultAgentOptions
	}

	created, err := h.agents.CreateAgent(request.Context(), payload)
	if err != nil {
		if strings.Contains(err.Error(), "名称") {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, created)
}

func currentOwnerUserID(request *http.Request) string {
	return authsvc.OwnerUserID(request.Context())
}

// HandleUpdateAgent 更新 agent。
func (h *Handlers) HandleUpdateAgent(writer http.ResponseWriter, request *http.Request) {
	var payload protocol.UpdateRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.agents.UpdateAgent(request.Context(), chi.URLParam(request, "agent_id"), payload)
	if errors.Is(err, agentpkg.ErrAgentNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "名称") || strings.Contains(err.Error(), "不可") || strings.Contains(err.Error(), "目录") {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleDeleteAgent 删除 agent。
func (h *Handlers) HandleDeleteAgent(writer http.ResponseWriter, request *http.Request) {
	err := h.agents.DeleteAgent(request.Context(), chi.URLParam(request, "agent_id"))
	if errors.Is(err, agentpkg.ErrAgentNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "不可删除") {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, map[string]any{"success": true})
}

// HandleListAgentSessions 返回指定 agent 的 session 列表。
func (h *Handlers) HandleListAgentSessions(writer http.ResponseWriter, request *http.Request) {
	items, err := h.sessions.ListAgentSessions(request.Context(), chi.URLParam(request, "agent_id"))
	if errors.Is(err, agentpkg.ErrAgentNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, items)
}

// HandleListSessions 返回全部 session 列表。
func (h *Handlers) HandleListSessions(writer http.ResponseWriter, request *http.Request) {
	items, err := h.sessions.ListSessions(request.Context())
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, items)
}

// HandleCreateSession 创建 session。
func (h *Handlers) HandleCreateSession(writer http.ResponseWriter, request *http.Request) {
	var payload sessionpkg.CreateRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.sessions.CreateSession(request.Context(), payload)
	if handlershared.IsStructuredSessionKeyError(err) {
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if errors.Is(err, sessionpkg.ErrSessionMutationUnsupported) || handlershared.IsClientMessageError(err) {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, agentpkg.ErrAgentNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleUpdateSession 更新 session。
func (h *Handlers) HandleUpdateSession(writer http.ResponseWriter, request *http.Request) {
	var payload sessionpkg.UpdateRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.sessions.UpdateSession(request.Context(), chi.URLParam(request, "session_key"), payload)
	if handlershared.IsStructuredSessionKeyError(err) {
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if errors.Is(err, sessionpkg.ErrSessionNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if errors.Is(err, sessionpkg.ErrSessionMutationUnsupported) || handlershared.IsClientMessageError(err) {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleDeleteSession 删除 session。
func (h *Handlers) HandleDeleteSession(writer http.ResponseWriter, request *http.Request) {
	err := h.sessions.DeleteSession(request.Context(), chi.URLParam(request, "session_key"))
	if handlershared.IsStructuredSessionKeyError(err) {
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if errors.Is(err, sessionpkg.ErrSessionNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, map[string]any{"success": true})
}
