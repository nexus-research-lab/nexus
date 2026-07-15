package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	clientopts "github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	nxsruntimesvc "github.com/nexus-research-lab/nexus/internal/service/nxsruntime"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	versionpkg "github.com/nexus-research-lab/nexus/internal/version"
)

// Handlers 封装核心 HTTP handlers。
type Handlers struct {
	api       *handlershared.API
	config    config.Config
	agents    *agentpkg.Service
	providers *providercfg.Service
	prefs     *preferencessvc.Service
	nxs       *nxsruntimesvc.Service
	runtime   *runtimectx.Manager
}

// SetRuntimeManager 绑定活跃 Agent runtime 管理器。
func (h *Handlers) SetRuntimeManager(manager *runtimectx.Manager) {
	h.runtime = manager
}

// New 创建核心 handlers。
func New(
	cfg config.Config,
	api *handlershared.API,
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
		config:    cfg,
		agents:    agents,
		providers: providers,
		prefs:     prefService,
		nxs:       nxsruntimesvc.NewService(),
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

// HandleSystemVersion 返回当前二进制版本信息。
func (h *Handlers) HandleSystemVersion(writer http.ResponseWriter, request *http.Request) {
	h.api.WriteSuccess(writer, versionpkg.Current())
}

// HandleRuntimeOptions 返回前端启动所需运行时选项。
func (h *Handlers) HandleRuntimeOptions(writer http.ResponseWriter, request *http.Request) {
	defaultAgent, err := h.agents.GetDefaultAgent(request.Context())
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	prefs, err := h.currentPreferences(request)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	providerOptions, err := h.providers.ListOptionsForRuntime(request.Context(), prefs.AgentRuntimeKind)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	prefs, _ = applyImagegenDefaultTool(prefs, providerOptions)
	defaultProvider := providerOptions.DefaultProvider
	defaultModel := providerOptions.DefaultModel
	providerValue := strings.TrimSpace(prefs.DefaultAgentOptions.Provider)
	modelValue := strings.TrimSpace(prefs.DefaultAgentOptions.Model)
	if providerValue != "" && modelValue != "" {
		defaultProvider = &providerValue
		defaultModel = &modelValue
	}
	h.api.WriteJSON(writer, http.StatusOK, map[string]any{
		"code":    "0000",
		"message": "success",
		"success": true,
		"data": map[string]any{
			"default_agent_id":       defaultAgent.AgentID,
			"default_agent_avatar":   defaultAgent.Avatar,
			"default_agent_provider": defaultProvider,
			"default_agent_model":    defaultModel,
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
	prefs, err = h.withProviderPreferenceDefaults(request, prefs)
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
	ownerUserID := currentOwnerUserID(request)
	webSearchChanged := payload.WebSearch != nil || payload.WebSearchAPIKey != nil
	var previous preferencessvc.Preferences
	var err error
	if webSearchChanged {
		previous, err = h.prefs.Get(request.Context(), ownerUserID)
		if err != nil {
			h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
			return
		}
	}
	item, err := h.prefs.Update(request.Context(), ownerUserID, payload)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	item, err = h.persistProviderPreferenceDefaults(request, item)
	if err != nil {
		if webSearchChanged {
			err = errors.Join(err, h.restoreWebSearchPreferences(request.Context(), ownerUserID, previous))
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	if err = h.syncWebSearchRuntime(request.Context(), item); err != nil {
		if webSearchChanged {
			err = errors.Join(
				err,
				h.syncWebSearchRuntime(request.Context(), previous),
				h.restoreWebSearchPreferences(request.Context(), ownerUserID, previous),
			)
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// restoreWebSearchPreferences 恢复运行时同步失败前的用户配置与凭据。
func (h *Handlers) restoreWebSearchPreferences(ctx context.Context, ownerUserID string, previous preferencessvc.Preferences) error {
	apiKey := previous.WebSearchAPIKey()
	_, err := h.prefs.Update(ctx, ownerUserID, preferencessvc.UpdateRequest{
		WebSearch:       &previous.WebSearch,
		WebSearchAPIKey: &apiKey,
	})
	return err
}

func (h *Handlers) syncWebSearchRuntime(ctx context.Context, preferences preferencessvc.Preferences) error {
	if h.runtime == nil || h.agents == nil {
		return nil
	}
	agents, err := h.agents.ListAgentRecords(ctx)
	if err != nil {
		return err
	}
	environment := clientopts.BuildWebSearchRuntimeEnv("nxs", preferences.WebSearch)
	for _, item := range agents {
		if err := h.runtime.UpdateEnvironmentForAgent(ctx, item.AgentID, environment); err != nil {
			return err
		}
	}
	return nil
}

// HandleGetRuntimeSettings 返回当前主机级运行配置。
func (h *Handlers) HandleGetRuntimeSettings(writer http.ResponseWriter, request *http.Request) {
	settings, err := config.LoadRuntimeSettings()
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, h.runtimeSettingsResponse(settings))
}

// HandleUpdateRuntimeSettings 更新当前主机级运行配置。
func (h *Handlers) HandleUpdateRuntimeSettings(writer http.ResponseWriter, request *http.Request) {
	var payload config.RuntimeSettings
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	settings, err := config.SaveRuntimeSettings(payload)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, h.runtimeSettingsResponse(settings))
}

// HandleNXSRuntimeStatus 返回当前主机上 nxs runtime 的本地可用状态。
func (h *Handlers) HandleNXSRuntimeStatus(writer http.ResponseWriter, request *http.Request) {
	h.api.WriteSuccess(writer, h.nxs.Status())
}

func (h *Handlers) currentPreferences(request *http.Request) (preferencessvc.Preferences, error) {
	if h.prefs == nil {
		return preferencessvc.DefaultPreferences(), nil
	}
	return h.prefs.Get(request.Context(), currentOwnerUserID(request))
}

func currentOwnerUserID(request *http.Request) string {
	return authsvc.OwnerUserID(request.Context())
}

func (h *Handlers) runtimeSettingsResponse(settings config.RuntimeSettings) map[string]any {
	return map[string]any{
		"workspace_path":         strings.TrimSpace(settings.WorkspacePath),
		"current_workspace_path": agentpkg.WorkspaceBasePath(h.config),
		"restart_required":       true,
		"updated_at":             strings.TrimSpace(settings.UpdatedAt),
	}
}
