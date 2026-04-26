package core

import (
	"net/http"
	"strings"

	agentpkg "github.com/nexus-research-lab/nexus/internal/agent"
	authsvc "github.com/nexus-research-lab/nexus/internal/auth"
	gatewayshared "github.com/nexus-research-lab/nexus/internal/gateway/shared"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/provider"

	"github.com/go-chi/chi/v5"
)

// Handlers 封装网关核心 HTTP handlers。
type Handlers struct {
	api       *gatewayshared.API
	agents    *agentpkg.Service
	providers *providercfg.Service
	prefs     *preferencessvc.Service
}

// New 创建核心 handlers。
func New(
	api *gatewayshared.API,
	agents *agentpkg.Service,
	providers *providercfg.Service,
	prefs ...*preferencessvc.Service,
) *Handlers {
	var prefService *preferencessvc.Service
	if len(prefs) > 0 {
		prefService = prefs[0]
	}
	return &Handlers{
		api:       api,
		agents:    agents,
		providers: providers,
		prefs:     prefService,
	}
}

// HandleHealth 返回健康检查。
func (h *Handlers) HandleHealth(writer http.ResponseWriter, request *http.Request) {
	h.api.WriteJSON(writer, http.StatusOK, map[string]any{
		"code": 0,
		"msg":  "ok",
		"data": map[string]any{
			"status": "ok",
		},
	})
}

// HandleRuntimeOptions 返回前端启动所需运行时选项。
func (h *Handlers) HandleRuntimeOptions(writer http.ResponseWriter, request *http.Request) {
	defaultAgent, err := h.agents.GetDefaultAgent(request.Context())
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	defaultProvider, err := h.providers.DefaultProvider(request.Context())
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	prefs, err := h.currentPreferences(request)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteJSON(writer, http.StatusOK, map[string]any{
		"code":    "0000",
		"message": "success",
		"success": true,
		"data": map[string]any{
			"default_agent_id":       defaultAgent.AgentID,
			"default_agent_avatar":   defaultAgent.Avatar,
			"default_agent_provider": defaultProvider,
			"preferences":            prefs,
		},
	})
}

// HandleGetPreferences 返回当前用户偏好。
func (h *Handlers) HandleGetPreferences(writer http.ResponseWriter, request *http.Request) {
	prefs, err := h.currentPreferences(request)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, prefs)
}

// HandleUpdatePreferences 更新当前用户偏好。
func (h *Handlers) HandleUpdatePreferences(writer http.ResponseWriter, request *http.Request) {
	if h.prefs == nil {
		h.api.WriteSuccess(writer, preferencessvc.DefaultPreferences())
		return
	}
	var payload preferencessvc.UpdateRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.prefs.Update(request.Context(), currentOwnerUserID(request), payload)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) currentPreferences(request *http.Request) (preferencessvc.Preferences, error) {
	if h.prefs == nil {
		return preferencessvc.DefaultPreferences(), nil
	}
	return h.prefs.Get(request.Context(), currentOwnerUserID(request))
}

func currentOwnerUserID(request *http.Request) string {
	if userID, ok := authsvc.CurrentUserID(request.Context()); ok {
		return userID
	}
	return authsvc.SystemUserID
}

// HandleListProviderConfigs 返回 provider 配置列表。
func (h *Handlers) HandleListProviderConfigs(writer http.ResponseWriter, request *http.Request) {
	items, err := h.providers.List(request.Context())
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, items)
}

// HandleListProviderOptions 返回 provider 下拉选项。
func (h *Handlers) HandleListProviderOptions(writer http.ResponseWriter, request *http.Request) {
	item, err := h.providers.ListOptions(request.Context())
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleCreateProviderConfig 创建 provider 配置。
func (h *Handlers) HandleCreateProviderConfig(writer http.ResponseWriter, request *http.Request) {
	var payload providercfg.CreateInput
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.providers.Create(request.Context(), payload)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleUpdateProviderConfig 更新 provider 配置。
func (h *Handlers) HandleUpdateProviderConfig(writer http.ResponseWriter, request *http.Request) {
	var payload providercfg.UpdateInput
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.providers.Update(request.Context(), chi.URLParam(request, "provider"), payload)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "不存在") {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleDeleteProviderConfig 删除 provider 配置。
func (h *Handlers) HandleDeleteProviderConfig(writer http.ResponseWriter, request *http.Request) {
	provider := chi.URLParam(request, "provider")
	if err := h.providers.Delete(request.Context(), provider); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "不存在") {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	h.api.WriteSuccess(writer, map[string]any{"provider": provider})
}
