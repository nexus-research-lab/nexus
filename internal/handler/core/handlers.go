package core

import (
	"net/http"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	nxsruntimesvc "github.com/nexus-research-lab/nexus/internal/service/nxsruntime"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	versionpkg "github.com/nexus-research-lab/nexus/internal/version"

	"github.com/go-chi/chi/v5"
)

// Handlers 封装核心 HTTP handlers。
type Handlers struct {
	api       *handlershared.API
	agents    *agentpkg.Service
	providers *providercfg.Service
	prefs     *preferencessvc.Service
	nxs       *nxsruntimesvc.Service
}

// New 创建核心 handlers。
func New(
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
	if strings.TrimSpace(prefs.DefaultAgentOptions.Provider) != "" && strings.TrimSpace(prefs.DefaultAgentOptions.Model) != "" {
		providerValue := strings.TrimSpace(prefs.DefaultAgentOptions.Provider)
		modelValue := strings.TrimSpace(prefs.DefaultAgentOptions.Model)
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
	item, err := h.prefs.Update(request.Context(), currentOwnerUserID(request), payload)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	item, err = h.persistProviderPreferenceDefaults(request, item)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
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

// HandleListProviderConfigs 返回 provider 配置列表。
func (h *Handlers) HandleListProviderConfigs(writer http.ResponseWriter, request *http.Request) {
	items, err := h.providers.List(request.Context())
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, items)
}

// HandleListProviderPresets 返回内置 Provider 模板列表。
func (h *Handlers) HandleListProviderPresets(writer http.ResponseWriter, request *http.Request) {
	h.api.WriteSuccess(writer, h.providers.ListPresets())
}

// HandleListProviderOptions 返回 provider 下拉选项。
func (h *Handlers) HandleListProviderOptions(writer http.ResponseWriter, request *http.Request) {
	runtimeKind := strings.TrimSpace(request.URL.Query().Get("agent_runtime_kind"))
	if runtimeKind == "" {
		runtimeKind = strings.TrimSpace(request.URL.Query().Get("runtime_kind"))
	}
	if runtimeKind == "" {
		prefs, err := h.currentPreferences(request)
		if err != nil {
			h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
			return
		}
		runtimeKind = prefs.AgentRuntimeKind
	}
	item, err := h.providers.ListOptionsForRuntime(request.Context(), runtimeKind)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleFetchProviderModels 拉取并保存 Provider 模型列表。
func (h *Handlers) HandleFetchProviderModels(writer http.ResponseWriter, request *http.Request) {
	item, err := h.providers.FetchModels(request.Context(), chi.URLParam(request, "provider"))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "不存在") {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		h.api.WriteFailure(writer, providerMutationErrorStatus(err), err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleUpdateProviderModel 更新 Provider 模型卡。
func (h *Handlers) HandleUpdateProviderModel(writer http.ResponseWriter, request *http.Request) {
	var payload providercfg.UpdateModelInput
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.providers.UpdateModel(
		request.Context(),
		chi.URLParam(request, "provider"),
		chi.URLParam(request, "model_id"),
		payload,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "不存在") {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		h.api.WriteFailure(writer, providerMutationErrorStatus(err), err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleSetDefaultProviderModel 设置默认运行模型。
func (h *Handlers) HandleSetDefaultProviderModel(writer http.ResponseWriter, request *http.Request) {
	item, err := h.providers.SetDefaultModel(
		request.Context(),
		chi.URLParam(request, "provider"),
		chi.URLParam(request, "model_id"),
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "不存在") {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		h.api.WriteFailure(writer, providerMutationErrorStatus(err), err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleTestProviderConfig 执行 Provider 连通性测试。
func (h *Handlers) HandleTestProviderConfig(writer http.ResponseWriter, request *http.Request) {
	item, err := h.providers.TestProvider(request.Context(), chi.URLParam(request, "provider"))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "不存在") {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		h.api.WriteFailure(writer, providerMutationErrorStatus(err), err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleTestProviderModel 执行指定模型的连通性测试。
func (h *Handlers) HandleTestProviderModel(writer http.ResponseWriter, request *http.Request) {
	item, err := h.providers.TestModel(
		request.Context(),
		chi.URLParam(request, "provider"),
		chi.URLParam(request, "model_id"),
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "不存在") {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		h.api.WriteFailure(writer, providerMutationErrorStatus(err), err.Error())
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
		h.api.WriteFailure(writer, providerMutationErrorStatus(err), err.Error())
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
		h.api.WriteFailure(writer, providerMutationErrorStatus(err), err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleDeleteProviderConfig 删除 provider 配置。
func (h *Handlers) HandleDeleteProviderConfig(writer http.ResponseWriter, request *http.Request) {
	provider := chi.URLParam(request, "provider")
	result, err := h.providers.Delete(request.Context(), provider, providercfg.DeleteInput{
		Force: parseBoolQuery(request.URL.Query().Get("force")),
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "不存在") {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		h.api.WriteFailure(writer, providerMutationErrorStatus(err), err.Error())
		return
	}
	h.api.WriteSuccess(writer, result)
}

func parseBoolQuery(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func providerMutationErrorStatus(err error) int {
	if err != nil && strings.Contains(err.Error(), "只有管理员") {
		return http.StatusForbidden
	}
	return http.StatusBadRequest
}
