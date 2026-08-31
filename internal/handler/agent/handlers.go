// INPUT: 已认证 owner 的 Agent/Session HTTP 请求、领域服务结果与提交证据。
// OUTPUT: Agent CRUD、exact 创建回执、Session 操作与兼容成功 envelope/FailureCore。
// POS: Agent HTTP 消费边界；创建校验只报告 not_applied，诊断 ID 不参与业务对账。
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
	communicationsvc "github.com/nexus-research-lab/nexus/internal/service/communication"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	sessionpkg "github.com/nexus-research-lab/nexus/internal/service/session"

	"github.com/go-chi/chi/v5"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

type directoryBroadcaster func(context.Context, string, map[string]any)

// roomPermissionModeSetter 只暴露 Agent 更新所需的 Room runtime 操作。
type roomPermissionModeSetter interface {
	SetPermissionModeForAgent(context.Context, string, sdkpermission.Mode) error
}

// Handlers 封装 Agent / Session 域 HTTP handlers。
type Handlers struct {
	api           *handlershared.API
	agents        *agentpkg.Service
	sessions      *sessionpkg.Service
	runtime       *runtimectx.Manager
	roomRealtime  roomPermissionModeSetter
	communication *communicationsvc.Service
	prefs         *preferencessvc.Service
	directory     directoryBroadcaster
}

// New 创建 Agent / Session 域 handlers。
func New(
	api *handlershared.API,
	agents *agentpkg.Service,
	sessions *sessionpkg.Service,
	runtime *runtimectx.Manager,
	roomRealtime roomPermissionModeSetter,
	communication *communicationsvc.Service,
	directory directoryBroadcaster,
	prefs ...*preferencessvc.Service,
) *Handlers {
	var prefService *preferencessvc.Service
	if len(prefs) > 0 {
		prefService = prefs[0]
	}
	return &Handlers{
		api:           api,
		agents:        agents,
		sessions:      sessions,
		runtime:       runtime,
		roomRealtime:  roomRealtime,
		communication: communication,
		prefs:         prefService,
		directory:     directory,
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

// HandleGetAgentProfileTemplate 返回新建 Agent 的默认行为模板。
func (h *Handlers) HandleGetAgentProfileTemplate(writer http.ResponseWriter, _ *http.Request) {
	h.api.WriteSuccess(writer, protocol.ProfileTemplateResponse{
		Content: agentpkg.DefaultProfileTemplate(),
	})
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
	if !h.api.BindJSONError(writer, request, &payload, handlershared.FailureSpec{
		Code:     "agent.creation_request_invalid",
		Category: protocol.FailureCategoryValidation,
		Effect:   protocol.FailureEffectNotApplied,
		Detail:   "创建 Agent 的内容格式不正确",
	}) {
		return
	}
	if payload.Options == nil && h.prefs != nil {
		prefs, prefErr := h.prefs.Get(request.Context(), authsvc.OwnerUserID(request.Context()))
		if prefErr != nil {
			h.api.WriteError(writer, request, http.StatusInternalServerError, handlershared.FailureSpec{
				Code:     "agent.creation_defaults_unavailable",
				Category: protocol.FailureCategoryInternal,
				Effect:   protocol.FailureEffectNotApplied,
				Detail:   "无法读取创建 Agent 所需的默认设置",
				Cause:    prefErr,
			})
			return
		}
		payload.Options = &prefs.DefaultAgentOptions
	}

	created, err := h.agents.CreateAgent(request.Context(), payload)
	if err != nil {
		if created != nil && agentpkg.AgentCreationCommitted(err) {
			h.broadcastDirectoryChanged(request.Context(), "agent_created", map[string]any{
				"agent_id": created.AgentID,
			})
		}
		status, failure := agentCreateFailure(err)
		h.api.WriteError(writer, request, status, failure)
		return
	}
	h.broadcastDirectoryChanged(request.Context(), "agent_created", map[string]any{
		"agent_id": created.AgentID,
	})
	h.api.WriteSuccess(writer, created)
}

// HandleGetAgentCreationRequest 按当前 owner 与 exact 业务请求身份返回创建回执。
func (h *Handlers) HandleGetAgentCreationRequest(writer http.ResponseWriter, request *http.Request) {
	result, err := h.agents.GetAgentCreationRequestResult(
		request.Context(),
		chi.URLParam(request, "creation_request_id"),
	)
	if err != nil {
		status, failure := agentCreationLookupFailure(err)
		h.api.WriteError(writer, request, status, failure)
		return
	}
	h.api.WriteSuccess(writer, result)
}

// HandleUpdateAgent 更新 agent。
func (h *Handlers) HandleUpdateAgent(writer http.ResponseWriter, request *http.Request) {
	var payload protocol.UpdateRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.agents.UpdateAgent(request.Context(), chi.URLParam(request, "agent_id"), payload)
	if err != nil {
		if item != nil && agentpkg.AgentUpdateCommitted(err) {
			h.broadcastDirectoryChanged(request.Context(), "agent_updated", map[string]any{
				"agent_id": item.AgentID,
			})
		}
		status, failure := agentUpdateFailure(err)
		h.api.WriteError(writer, request, status, failure)
		return
	}
	if err := h.applyUpdatedPermissionMode(request.Context(), item, payload); err != nil {
		h.broadcastDirectoryChanged(request.Context(), "agent_updated", map[string]any{
			"agent_id": item.AgentID,
		})
		h.api.WriteError(
			writer,
			request,
			http.StatusInternalServerError,
			agentPermissionModeSyncFailure(err),
		)
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
	if err != nil {
		status, failure := agentDeleteFailure(err)
		h.api.WriteError(writer, request, status, failure)
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
