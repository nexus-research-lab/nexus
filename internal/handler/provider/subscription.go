package provider

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

// HandleListSubscriptionProviderConfigs 返回订阅运营使用的公共 provider 配置列表。
func (h *Handlers) HandleListSubscriptionProviderConfigs(writer http.ResponseWriter, request *http.Request) {
	if !h.requireSubscriptionProviderAdmin(writer, request) {
		return
	}
	items, err := h.providers.ListPublic(request.Context())
	if err != nil {
		h.api.WriteError(writer, request, http.StatusBadRequest, providerReadFailure(err))
		return
	}
	h.api.WriteSuccess(writer, items)
}

// HandleCreateSubscriptionProviderConfig 创建订阅运营使用的公共 provider 配置。
func (h *Handlers) HandleCreateSubscriptionProviderConfig(writer http.ResponseWriter, request *http.Request) {
	if !h.requireSubscriptionProviderAdmin(writer, request) {
		return
	}
	var payload providercfg.CreateInput
	if !h.api.BindJSONError(writer, request, &payload, providerRequestInvalidFailure("create")) {
		return
	}
	item, err := h.providers.CreatePublic(request.Context(), payload)
	if err != nil {
		h.writeProviderMutationFailure(writer, request, "create", err)
		return
	}
	writeProviderETag(writer, item.ConfigurationVersion)
	h.api.WriteSuccess(writer, item)
}

// HandleFetchSubscriptionProviderModels 拉取订阅 provider 的模型列表。
func (h *Handlers) HandleFetchSubscriptionProviderModels(writer http.ResponseWriter, request *http.Request) {
	if !h.requireSubscriptionProviderAdmin(writer, request) {
		return
	}
	item, err := h.providers.FetchPublicModels(request.Context(), chi.URLParam(request, "provider"))
	if err != nil {
		h.writeProviderMutationFailure(writer, request, "fetch_models", err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleUpdateSubscriptionProviderModel 更新订阅 provider 的模型卡。
func (h *Handlers) HandleUpdateSubscriptionProviderModel(writer http.ResponseWriter, request *http.Request) {
	if !h.requireSubscriptionProviderAdmin(writer, request) {
		return
	}
	var payload providercfg.UpdateModelInput
	if !h.api.BindJSONError(writer, request, &payload, providerRequestInvalidFailure("update_model")) {
		return
	}
	item, err := h.providers.UpdatePublicModel(
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

// HandleDeleteSubscriptionProviderModel 删除订阅 Provider 模型卡。
func (h *Handlers) HandleDeleteSubscriptionProviderModel(writer http.ResponseWriter, request *http.Request) {
	if !h.requireSubscriptionProviderAdmin(writer, request) {
		return
	}
	item, err := h.providers.DeletePublicModel(
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

// HandleTestSubscriptionProviderConfig 执行订阅 provider 连通性测试。
func (h *Handlers) HandleTestSubscriptionProviderConfig(writer http.ResponseWriter, request *http.Request) {
	if !h.requireSubscriptionProviderAdmin(writer, request) {
		return
	}
	item, err := h.providers.TestPublicProvider(request.Context(), chi.URLParam(request, "provider"))
	if err != nil {
		h.writeProviderMutationFailure(writer, request, "test", err)
		return
	}
	writeProviderETag(writer, item.ConfigurationVersion)
	h.api.WriteSuccess(writer, item)
}

// HandleTestSubscriptionProviderModel 执行订阅 provider 指定模型的连通性测试。
func (h *Handlers) HandleTestSubscriptionProviderModel(writer http.ResponseWriter, request *http.Request) {
	if !h.requireSubscriptionProviderAdmin(writer, request) {
		return
	}
	item, err := h.providers.TestPublicModel(
		request.Context(),
		chi.URLParam(request, "provider"),
		chi.URLParam(request, "model_id"),
	)
	if err != nil {
		h.writeProviderMutationFailure(writer, request, "test_model", err)
		return
	}
	writeProviderETag(writer, item.ConfigurationVersion)
	h.api.WriteSuccess(writer, item)
}

// HandleUpdateSubscriptionProviderConfig 更新订阅运营使用的公共 provider 配置。
func (h *Handlers) HandleUpdateSubscriptionProviderConfig(writer http.ResponseWriter, request *http.Request) {
	if !h.requireSubscriptionProviderAdmin(writer, request) {
		return
	}
	var payload providercfg.UpdateInput
	if !h.api.BindJSONError(writer, request, &payload, providerRequestInvalidFailure("update")) {
		return
	}
	item, err := h.providers.UpdatePublic(request.Context(), chi.URLParam(request, "provider"), payload)
	if err != nil {
		h.writeProviderMutationFailure(writer, request, "update", err)
		return
	}
	writeProviderETag(writer, item.ConfigurationVersion)
	h.api.WriteSuccess(writer, item)
}

// HandleDeleteSubscriptionProviderConfig 删除订阅运营使用的公共 provider 配置。
func (h *Handlers) HandleDeleteSubscriptionProviderConfig(writer http.ResponseWriter, request *http.Request) {
	if !h.requireSubscriptionProviderAdmin(writer, request) {
		return
	}
	result, err := h.providers.DeletePublic(request.Context(), chi.URLParam(request, "provider"), providercfg.DeleteInput{
		Force: parseBoolQuery(request.URL.Query().Get("force")),
	})
	if err != nil {
		h.writeProviderMutationFailure(writer, request, "delete", err)
		return
	}
	h.api.WriteSuccess(writer, result)
}

func (h *Handlers) requireSubscriptionProviderAdmin(
	writer http.ResponseWriter,
	request *http.Request,
) bool {
	principal := authctx.PrincipalFromContext(request.Context())
	if principal == nil {
		// 认证关闭的本地单用户模式仍由本机 owner 管理。
		return true
	}
	switch strings.TrimSpace(principal.Role) {
	case authctx.RoleOwner, authctx.RoleAdmin:
		return true
	default:
		h.writeProviderMutationFailure(
			writer,
			request,
			"admin_access",
			providercfg.ErrProviderManagementForbidden,
		)
		return false
	}
}
