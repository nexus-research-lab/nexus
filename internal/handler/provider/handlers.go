package provider

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
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
	writer.Header().Set("Cache-Control", "no-store")
	items, err := h.providers.List(request.Context())
	if err != nil {
		h.writeProviderReadFailure(writer, request, err)
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
	if !h.api.BindJSONError(writer, request, &payload, providerPreviewRequestInvalidFailure()) {
		return
	}
	prefs, err := h.currentPreferences(request)
	if err != nil {
		h.writeProviderReadFailure(writer, request, err)
		return
	}
	payload.RuntimeKind = prefs.AgentRuntimeKind
	result, err := h.providers.PreviewCCSwitch(request.Context(), payload)
	if err != nil {
		h.api.WriteError(writer, request, http.StatusBadRequest, providerImportPreviewFailure(err))
		return
	}
	h.api.WriteSuccess(writer, result)
}

// HandleSyncCCSwitch 把所选 CC Switch Provider 同步到当前用户私有配置。
func (h *Handlers) HandleSyncCCSwitch(writer http.ResponseWriter, request *http.Request) {
	var payload providercfg.CCSwitchSyncInput
	if !h.api.BindJSONError(writer, request, &payload, providerRequestInvalidFailure("sync_import")) {
		return
	}
	prefs, err := h.currentPreferences(request)
	if err != nil {
		h.writeProviderReadFailure(writer, request, err)
		return
	}
	payload.RuntimeKind = prefs.AgentRuntimeKind
	result, err := h.providers.SyncCCSwitch(request.Context(), payload)
	if err != nil {
		h.writeProviderMutationFailure(writer, request, "sync_import", err)
		return
	}
	if result.DefaultSelection != nil && h.prefs != nil {
		if _, err = h.prefs.Update(
			request.Context(),
			authsvc.OwnerUserID(request.Context()),
			ccSwitchDefaultPreferencesUpdate(prefs, *result.DefaultSelection),
		); err != nil {
			_, failure := providerMutationFailure(
				"sync_import",
				errors.Join(providercfg.ErrMutationCommitted, err),
			)
			h.api.WriteError(writer, request, http.StatusInternalServerError, failure)
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
		h.writeProviderReadFailure(writer, request, err)
		return
	}
	if runtimeKind == "" {
		runtimeKind = prefs.AgentRuntimeKind
	}
	item, err := h.providers.ListOptionsForRuntime(request.Context(), runtimeKind)
	if err != nil {
		h.writeProviderReadFailure(writer, request, err)
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
		h.writeProviderMutationFailure(writer, request, "fetch_models", err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleUpdateProviderModel 更新 Provider 模型卡。
func (h *Handlers) HandleUpdateProviderModel(writer http.ResponseWriter, request *http.Request) {
	var payload providercfg.UpdateModelInput
	if !h.api.BindJSONError(writer, request, &payload, providerRequestInvalidFailure("update_model")) {
		return
	}
	item, err := h.providers.UpdateModel(
		request.Context(),
		chi.URLParam(request, "provider"),
		chi.URLParam(request, "model_id"),
		payload,
	)
	if err != nil {
		h.writeProviderMutationFailure(writer, request, "update_model", err)
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
		h.writeProviderMutationFailure(writer, request, "set_default_model", err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleTestProviderConfig 执行 Provider 连通性测试。
func (h *Handlers) HandleTestProviderConfig(writer http.ResponseWriter, request *http.Request) {
	expectedVersion, err := parseProviderIfMatch(request.Header.Get("If-Match"))
	if err != nil {
		h.writeProviderPreconditionFailure(writer, request, err)
		return
	}
	var item *providercfg.TestResult
	if expectedVersion == nil {
		item, err = h.providers.TestProvider(request.Context(), chi.URLParam(request, "provider"))
	} else {
		item, err = h.providers.TestProviderAtVersion(
			request.Context(), chi.URLParam(request, "provider"), *expectedVersion,
		)
	}
	if err != nil {
		h.writeProviderMutationFailure(writer, request, "test", err, expectedVersion != nil)
		return
	}
	writeProviderETag(writer, item.ConfigurationVersion)
	h.api.WriteSuccess(writer, item)
}

// HandleTestProviderModel 执行指定模型的连通性测试。
func (h *Handlers) HandleTestProviderModel(writer http.ResponseWriter, request *http.Request) {
	expectedVersion, err := parseProviderIfMatch(request.Header.Get("If-Match"))
	if err != nil {
		h.writeProviderPreconditionFailure(writer, request, err)
		return
	}
	var item *providercfg.TestResult
	if expectedVersion == nil {
		item, err = h.providers.TestModel(
			request.Context(),
			chi.URLParam(request, "provider"),
			chi.URLParam(request, "model_id"),
		)
	} else {
		item, err = h.providers.TestModelAtVersion(
			request.Context(),
			chi.URLParam(request, "provider"),
			chi.URLParam(request, "model_id"),
			*expectedVersion,
		)
	}
	if err != nil {
		h.writeProviderMutationFailure(writer, request, "test_model", err, expectedVersion != nil)
		return
	}
	writeProviderETag(writer, item.ConfigurationVersion)
	h.api.WriteSuccess(writer, item)
}

// HandleCreateProviderConfig 创建 provider 配置。
func (h *Handlers) HandleCreateProviderConfig(writer http.ResponseWriter, request *http.Request) {
	var payload providercfg.CreateInput
	if !h.api.BindJSONError(writer, request, &payload, providerRequestInvalidFailure("create")) {
		return
	}
	item, err := h.providers.Create(request.Context(), payload)
	if err != nil {
		h.writeProviderMutationFailure(writer, request, "create", err)
		return
	}
	writeProviderETag(writer, item.ConfigurationVersion)
	h.api.WriteSuccess(writer, item)
}

// HandleUpdateProviderConfig 更新 provider 配置。
func (h *Handlers) HandleUpdateProviderConfig(writer http.ResponseWriter, request *http.Request) {
	var payload providercfg.UpdateInput
	if !h.api.BindJSONError(writer, request, &payload, providerRequestInvalidFailure("update")) {
		return
	}
	expectedVersion, err := parseProviderIfMatch(request.Header.Get("If-Match"))
	if err != nil {
		h.writeProviderPreconditionFailure(writer, request, err)
		return
	}
	var item *providercfg.Record
	if expectedVersion == nil {
		item, err = h.providers.Update(request.Context(), chi.URLParam(request, "provider"), payload)
	} else {
		item, err = h.providers.UpdateAtVersion(
			request.Context(), chi.URLParam(request, "provider"), payload, *expectedVersion,
		)
	}
	if err != nil {
		h.writeProviderMutationFailure(writer, request, "update", err, expectedVersion != nil)
		return
	}
	writeProviderETag(writer, item.ConfigurationVersion)
	h.api.WriteSuccess(writer, item)
}

// HandleDeleteProviderConfig 删除 provider 配置。
func (h *Handlers) HandleDeleteProviderConfig(writer http.ResponseWriter, request *http.Request) {
	provider := chi.URLParam(request, "provider")
	expectedVersion, err := parseProviderIfMatch(request.Header.Get("If-Match"))
	if err != nil {
		h.writeProviderPreconditionFailure(writer, request, err)
		return
	}
	input := providercfg.DeleteInput{Force: parseBoolQuery(request.URL.Query().Get("force"))}
	var result *providercfg.DeleteResult
	if expectedVersion == nil {
		result, err = h.providers.Delete(request.Context(), provider, input)
	} else {
		result, err = h.providers.DeleteAtVersion(request.Context(), provider, input, *expectedVersion)
	}
	if err != nil {
		h.writeProviderMutationFailure(writer, request, "delete", err, expectedVersion != nil)
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

func providerRequestInvalidFailure(operation string) handlershared.FailureSpec {
	return handlershared.FailureSpec{
		Code:     "provider." + operation + "_request_invalid",
		Category: protocol.FailureCategoryValidation,
		Effect:   protocol.FailureEffectNotApplied,
		Detail:   "模型服务设置格式不正确",
	}
}

func providerPreviewRequestInvalidFailure() handlershared.FailureSpec {
	return handlershared.FailureSpec{
		Code:     "provider.preview_import_request_invalid",
		Category: protocol.FailureCategoryValidation,
		Effect:   protocol.FailureEffectNotApplicable,
		Detail:   "本机模型服务配置的读取条件不完整",
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
