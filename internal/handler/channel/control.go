// INPUT: 已认证 owner 的 Channel 配置、账号、登录与 Pairing HTTP 请求。
// OUTPUT: 成功读模型，或携带领域 effect 与恢复动作的 FailureCore 响应。
// POS: Channel 人工控制 HTTP 入口；不从错误文本推断状态，也不重放未知写入。
package channel

import (
	"errors"
	"net/http"

	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	channelspkg "github.com/nexus-research-lab/nexus/internal/service/channels"

	"github.com/go-chi/chi/v5"
)

func (h *Handlers) HandleListChannels(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer, request, channelOperationListConfigs, true) {
		return
	}
	items, err := h.control.ListChannels(request.Context(), currentOwnerUserID(request))
	if err != nil {
		h.writeChannelReadFailure(writer, request, channelOperationListConfigs, err)
		return
	}
	h.api.WriteSuccess(writer, items)
}

func (h *Handlers) HandleUpsertChannelConfig(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer, request, channelOperationSaveConfig, false) {
		return
	}
	var payload channelspkg.UpsertChannelConfigRequest
	if !h.api.BindJSONError(writer, request, &payload, channelControlRequestFailure(channelOperationSaveConfig)) {
		return
	}
	item, err := h.control.UpsertChannelConfig(
		request.Context(),
		currentOwnerUserID(request),
		chi.URLParam(request, "channel_type"),
		payload,
	)
	if errors.Is(err, channelspkg.ErrChannelNotFound) {
		h.writeChannelMutationFailure(writer, request, channelOperationSaveConfig, err)
		return
	}
	if err != nil {
		h.writeChannelMutationFailure(writer, request, channelOperationSaveConfig, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleDeleteChannelConfig(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer, request, channelOperationDeleteConfig, false) {
		return
	}
	err := h.control.DeleteChannelConfig(request.Context(), currentOwnerUserID(request), chi.URLParam(request, "channel_type"))
	if err != nil {
		h.writeChannelMutationFailure(writer, request, channelOperationDeleteConfig, err)
		return
	}
	h.api.WriteSuccess(writer, map[string]any{"configured": false})
}

func (h *Handlers) HandleDeleteChannelAccount(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer, request, channelOperationDeleteAccount, false) {
		return
	}
	item, err := h.control.DeleteChannelAccount(
		request.Context(),
		currentOwnerUserID(request),
		chi.URLParam(request, "channel_type"),
		chi.URLParam(request, "account_id"),
	)
	if errors.Is(err, channelspkg.ErrChannelNotFound) || errors.Is(err, channelspkg.ErrChannelAccountNotFound) {
		h.writeChannelMutationFailure(writer, request, channelOperationDeleteAccount, err)
		return
	}
	if err != nil {
		h.writeChannelMutationFailure(writer, request, channelOperationDeleteAccount, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleStartChannelLogin(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer, request, channelOperationStartLogin, false) {
		return
	}
	item, err := h.control.StartChannelLogin(
		request.Context(),
		currentOwnerUserID(request),
		chi.URLParam(request, "channel_type"),
	)
	if errors.Is(err, channelspkg.ErrChannelNotFound) || errors.Is(err, channelspkg.ErrChannelLoginNotFound) {
		h.writeChannelMutationFailure(writer, request, channelOperationStartLogin, err)
		return
	}
	if errors.Is(err, channelspkg.ErrChannelLoginUnsupported) {
		h.writeChannelMutationFailure(writer, request, channelOperationStartLogin, err)
		return
	}
	if err != nil {
		h.writeChannelMutationFailure(writer, request, channelOperationStartLogin, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleGetCurrentChannelLogin(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer, request, channelOperationReadLogin, true) {
		return
	}
	item, err := h.control.GetCurrentChannelLogin(
		request.Context(),
		currentOwnerUserID(request),
		chi.URLParam(request, "channel_type"),
	)
	if err != nil {
		h.writeChannelReadFailure(writer, request, channelOperationReadLogin, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleGetChannelLogin(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer, request, channelOperationReadLogin, true) {
		return
	}
	item, err := h.control.GetChannelLogin(
		request.Context(),
		currentOwnerUserID(request),
		chi.URLParam(request, "channel_type"),
		chi.URLParam(request, "login_id"),
	)
	if errors.Is(err, channelspkg.ErrChannelNotFound) || errors.Is(err, channelspkg.ErrChannelLoginNotFound) {
		h.writeChannelReadFailure(writer, request, channelOperationReadLogin, err)
		return
	}
	if errors.Is(err, channelspkg.ErrChannelLoginUnsupported) {
		h.writeChannelReadFailure(writer, request, channelOperationReadLogin, err)
		return
	}
	if err != nil {
		h.writeChannelReadFailure(writer, request, channelOperationReadLogin, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleSubmitChannelLoginVerifyCode(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer, request, channelOperationSubmitVerifyCode, false) {
		return
	}
	var payload channelspkg.SubmitChannelLoginVerifyCodeRequest
	if !h.api.BindJSONError(writer, request, &payload, channelControlRequestFailure(channelOperationSubmitVerifyCode)) {
		return
	}
	item, err := h.control.SubmitChannelLoginVerifyCode(
		request.Context(),
		currentOwnerUserID(request),
		chi.URLParam(request, "channel_type"),
		chi.URLParam(request, "login_id"),
		payload,
	)
	if errors.Is(err, channelspkg.ErrChannelNotFound) || errors.Is(err, channelspkg.ErrChannelLoginNotFound) {
		h.writeChannelMutationFailure(writer, request, channelOperationSubmitVerifyCode, err)
		return
	}
	if errors.Is(err, channelspkg.ErrChannelLoginUnsupported) {
		h.writeChannelMutationFailure(writer, request, channelOperationSubmitVerifyCode, err)
		return
	}
	if err != nil {
		h.writeChannelMutationFailure(writer, request, channelOperationSubmitVerifyCode, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleListPairings(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer, request, channelOperationListPairings, true) {
		return
	}
	query := request.URL.Query()
	items, err := h.control.ListPairings(request.Context(), currentOwnerUserID(request), channelspkg.PairingQuery{
		ChannelType: query.Get("channel_type"),
		Status:      query.Get("status"),
		AgentID:     query.Get("agent_id"),
	})
	if err != nil {
		h.writeChannelReadFailure(writer, request, channelOperationListPairings, err)
		return
	}
	h.api.WriteSuccess(writer, items)
}

func (h *Handlers) HandleCreatePairing(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer, request, channelOperationCreatePairing, false) {
		return
	}
	var payload channelspkg.CreatePairingRequest
	if !h.api.BindJSONError(writer, request, &payload, channelControlRequestFailure(channelOperationCreatePairing)) {
		return
	}
	item, err := h.control.CreatePairing(request.Context(), currentOwnerUserID(request), payload)
	if errors.Is(err, channelspkg.ErrChannelNotFound) {
		h.writeChannelMutationFailure(writer, request, channelOperationCreatePairing, err)
		return
	}
	if err != nil {
		h.writeChannelMutationFailure(writer, request, channelOperationCreatePairing, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleUpdatePairing(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer, request, channelOperationUpdatePairing, false) {
		return
	}
	var payload channelspkg.UpdatePairingRequest
	if !h.api.BindJSONError(writer, request, &payload, channelControlRequestFailure(channelOperationUpdatePairing)) {
		return
	}
	item, err := h.control.UpdatePairing(
		request.Context(),
		currentOwnerUserID(request),
		chi.URLParam(request, "pairing_id"),
		payload,
	)
	if errors.Is(err, channelspkg.ErrPairingNotFound) {
		h.writeChannelMutationFailure(writer, request, channelOperationUpdatePairing, err)
		return
	}
	if err != nil {
		h.writeChannelMutationFailure(writer, request, channelOperationUpdatePairing, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleDeletePairing(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer, request, channelOperationDeletePairing, false) {
		return
	}
	err := h.control.DeletePairing(request.Context(), currentOwnerUserID(request), chi.URLParam(request, "pairing_id"))
	if errors.Is(err, channelspkg.ErrPairingNotFound) {
		h.writeChannelMutationFailure(writer, request, channelOperationDeletePairing, err)
		return
	}
	if err != nil {
		h.writeChannelMutationFailure(writer, request, channelOperationDeletePairing, err)
		return
	}
	h.api.WriteSuccess(writer, map[string]any{"success": true})
}

func (h *Handlers) ensureControl(
	writer http.ResponseWriter,
	request *http.Request,
	operation channelControlOperation,
	readOnly bool,
) bool {
	if h.control != nil {
		return true
	}
	h.writeChannelControlUnavailable(writer, request, operation, readOnly)
	return false
}

func currentOwnerUserID(request *http.Request) string {
	return authsvc.OwnerUserID(request.Context())
}
