package provider

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

// Handlers 封装 Provider HTTP handlers。
type Handlers struct {
	api       *handlershared.API
	providers *providercfg.Service
	prefs     *preferencessvc.Service
}

// New 创建 Provider handlers。
func New(api *handlershared.API, providers *providercfg.Service, prefs ...*preferencessvc.Service) *Handlers {
	var prefService *preferencessvc.Service
	if len(prefs) > 0 {
		prefService = prefs[0]
	}
	return &Handlers{api: api, providers: providers, prefs: prefService}
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

// HandlePreviewCCSwitch 只读返回本机 CC Switch Provider 同步预览。
func (h *Handlers) HandlePreviewCCSwitch(writer http.ResponseWriter, request *http.Request) {
	var payload providercfg.CCSwitchPreviewInput
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	prefs, err := h.currentPreferences(request)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	payload.RuntimeKind = prefs.AgentRuntimeKind
	result, err := h.providers.PreviewCCSwitch(request.Context(), payload)
	if err != nil {
		h.api.WriteFailure(writer, providerMutationErrorStatus(err), err.Error())
		return
	}
	h.api.WriteSuccess(writer, result)
}

// HandleSyncCCSwitch 把所选 CC Switch Provider 同步到当前用户私有配置。
func (h *Handlers) HandleSyncCCSwitch(writer http.ResponseWriter, request *http.Request) {
	var payload providercfg.CCSwitchSyncInput
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	prefs, err := h.currentPreferences(request)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	payload.RuntimeKind = prefs.AgentRuntimeKind
	result, err := h.providers.SyncCCSwitch(request.Context(), payload)
	if err != nil {
		h.api.WriteFailure(writer, providerMutationErrorStatus(err), err.Error())
		return
	}
	if result.DefaultSelection != nil && h.prefs != nil {
		if _, err = h.prefs.Update(
			request.Context(),
			authsvc.OwnerUserID(request.Context()),
			ccSwitchDefaultPreferencesUpdate(prefs, *result.DefaultSelection),
		); err != nil {
			h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
			return
		}
	}
	h.api.WriteSuccess(writer, result)
}

// ccSwitchDefaultPreferencesUpdate 让显式默认选择同时覆盖对话与后台任务。
func ccSwitchDefaultPreferencesUpdate(
	prefs preferencessvc.Preferences,
	selection providercfg.ModelSelection,
) preferencessvc.UpdateRequest {
	options := prefs.DefaultAgentOptions
	options.Provider = selection.Provider
	options.Model = selection.Model
	background := preferencessvc.ModelSelection{
		Provider: selection.Provider,
		Model:    selection.Model,
	}
	return preferencessvc.UpdateRequest{
		DefaultAgentOptions:             &options,
		DefaultBackgroundModelSelection: &background,
	}
}

// HandleListProviderOptions 返回 provider 下拉选项。
func (h *Handlers) HandleListProviderOptions(writer http.ResponseWriter, request *http.Request) {
	runtimeKind := strings.TrimSpace(request.URL.Query().Get("agent_runtime_kind"))
	if runtimeKind == "" {
		runtimeKind = strings.TrimSpace(request.URL.Query().Get("runtime_kind"))
	}
	prefs, err := h.currentPreferences(request)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	if runtimeKind == "" {
		runtimeKind = prefs.AgentRuntimeKind
	}
	item, err := h.providers.ListOptionsForRuntime(request.Context(), runtimeKind)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	// 覆盖默认值为用户偏好的 Provider/Model
	providerValue := strings.TrimSpace(prefs.DefaultAgentOptions.Provider)
	modelValue := strings.TrimSpace(prefs.DefaultAgentOptions.Model)
	if providerValue != "" && modelValue != "" {
		item.DefaultProvider = &providerValue
		item.DefaultModel = &modelValue
		item.DefaultSelection = &providercfg.ModelSelection{
			Provider:            providerValue,
			ProviderDisplayName: providerValue,
			Model:               modelValue,
			ModelDisplayName:    modelValue,
		}
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

// HandleDeleteProviderModel 删除 Provider 模型卡。
func (h *Handlers) HandleDeleteProviderModel(writer http.ResponseWriter, request *http.Request) {
	item, err := h.providers.DeleteModel(
		request.Context(),
		chi.URLParam(request, "provider"),
		chi.URLParam(request, "model_id"),
	)
	if err != nil {
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

func (h *Handlers) currentPreferences(request *http.Request) (preferencessvc.Preferences, error) {
	if h.prefs == nil {
		return preferencessvc.DefaultPreferences(), nil
	}
	return h.prefs.Get(request.Context(), authsvc.OwnerUserID(request.Context()))
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
	switch {
	case errors.Is(err, providercfg.ErrProviderNotFound), errors.Is(err, providercfg.ErrModelNotFound):
		return http.StatusNotFound
	case errors.Is(err, providercfg.ErrConfigurationVersionConflict):
		return http.StatusConflict
	}
	if err != nil && strings.Contains(err.Error(), "只有管理员") {
		return http.StatusForbidden
	}
	return http.StatusBadRequest
}
