package agent

import (
	"context"
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
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

type directoryBroadcaster func(context.Context, string, map[string]any)

// Handlers 封装 Agent / Session 域 HTTP handlers。
type Handlers struct {
	api          *handlershared.API
	agents       *agentpkg.Service
	sessions     *sessionpkg.Service
	runtime      *runtimectx.Manager
	roomRealtime *roompkg.RealtimeService
	prefs        *preferencessvc.Service
	directory    directoryBroadcaster
}

// New 创建 Agent / Session 域 handlers。
func New(
	api *handlershared.API,
	agents *agentpkg.Service,
	sessions *sessionpkg.Service,
	runtime *runtimectx.Manager,
	roomRealtime *roompkg.RealtimeService,
	directory directoryBroadcaster,
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
		directory:    directory,
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
		prefs, prefErr := h.prefs.Get(request.Context(), authsvc.OwnerUserID(request.Context()))
		if prefErr != nil {
			h.api.WriteFailure(writer, http.StatusInternalServerError, prefErr.Error())
			return
		}
		payload.Options = &prefs.DefaultAgentOptions
	}

	created, err := h.agents.CreateAgent(request.Context(), payload)
	if err != nil {
		if errors.Is(err, agentpkg.ErrAgentNameInvalid) || strings.Contains(err.Error(), "名称") {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.broadcastDirectoryChanged(request.Context(), "agent_created", map[string]any{
		"agent_id": created.AgentID,
	})
	h.api.WriteSuccess(writer, created)
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
		if errors.Is(err, agentpkg.ErrAgentNameInvalid) ||
			strings.Contains(err.Error(), "名称") ||
			strings.Contains(err.Error(), "不可") ||
			strings.Contains(err.Error(), "目录") {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.applyUpdatedPermissionMode(request.Context(), item, payload); err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.broadcastDirectoryChanged(request.Context(), "agent_updated", map[string]any{
		"agent_id": item.AgentID,
	})
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) applyUpdatedPermissionMode(ctx context.Context, item *protocol.Agent, payload protocol.UpdateRequest) error {
	if h == nil || item == nil || payload.Options == nil || strings.TrimSpace(payload.Options.PermissionMode) == "" {
		return nil
	}
	mode := sdkpermission.Mode(strings.TrimSpace(item.Options.PermissionMode))
	if h.runtime != nil {
		if err := h.runtime.SetPermissionModeForAgent(ctx, item.AgentID, mode); err != nil {
			return err
		}
	}
	if h.roomRealtime != nil {
		if err := h.roomRealtime.SetPermissionModeForAgent(ctx, item.AgentID, mode); err != nil {
			return err
		}
	}
	return nil
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
	h.broadcastDirectoryChanged(request.Context(), "agent_deleted", map[string]any{
		"agent_id": chi.URLParam(request, "agent_id"),
	})
	h.api.WriteSuccess(writer, map[string]any{"success": true})
}

func (h *Handlers) broadcastDirectoryChanged(ctx context.Context, reason string, data map[string]any) {
	if h.directory == nil {
		return
	}
	h.directory(ctx, reason, data)
}
