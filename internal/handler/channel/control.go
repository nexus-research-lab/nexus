package channel

import (
	"errors"
	"net/http"
	"strings"

	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	channelspkg "github.com/nexus-research-lab/nexus/internal/service/channels"

	"github.com/go-chi/chi/v5"
)

func (h *Handlers) HandleListChannels(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer) {
		return
	}
	items, err := h.control.ListChannels(request.Context(), currentOwnerUserID(request))
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, items)
}

func (h *Handlers) HandleUpsertChannelConfig(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer) {
		return
	}
	var payload channelspkg.UpsertChannelConfigRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.control.UpsertChannelConfig(
		request.Context(),
		currentOwnerUserID(request),
		chi.URLParam(request, "channel_type"),
		payload,
	)
	if errors.Is(err, channelspkg.ErrChannelNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.writeControlFailure(writer, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleDeleteChannelConfig(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer) {
		return
	}
	err := h.control.DeleteChannelConfig(request.Context(), currentOwnerUserID(request), chi.URLParam(request, "channel_type"))
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, map[string]any{"configured": false})
}

func (h *Handlers) HandleDeleteChannelAccount(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer) {
		return
	}
	item, err := h.control.DeleteChannelAccount(
		request.Context(),
		currentOwnerUserID(request),
		chi.URLParam(request, "channel_type"),
		chi.URLParam(request, "account_id"),
	)
	if errors.Is(err, channelspkg.ErrChannelNotFound) || errors.Is(err, channelspkg.ErrChannelAccountNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.writeControlFailure(writer, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleStartChannelLogin(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer) {
		return
	}
	item, err := h.control.StartChannelLogin(
		request.Context(),
		currentOwnerUserID(request),
		chi.URLParam(request, "channel_type"),
	)
	if errors.Is(err, channelspkg.ErrChannelNotFound) || errors.Is(err, channelspkg.ErrChannelLoginNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if errors.Is(err, channelspkg.ErrChannelLoginUnsupported) {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		h.writeControlFailure(writer, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleGetChannelLogin(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer) {
		return
	}
	item, err := h.control.GetChannelLogin(
		request.Context(),
		currentOwnerUserID(request),
		chi.URLParam(request, "channel_type"),
		chi.URLParam(request, "login_id"),
	)
	if errors.Is(err, channelspkg.ErrChannelNotFound) || errors.Is(err, channelspkg.ErrChannelLoginNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if errors.Is(err, channelspkg.ErrChannelLoginUnsupported) {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		h.writeControlFailure(writer, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleSubmitChannelLoginVerifyCode(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer) {
		return
	}
	var payload channelspkg.SubmitChannelLoginVerifyCodeRequest
	if !h.api.BindJSON(writer, request, &payload) {
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
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if errors.Is(err, channelspkg.ErrChannelLoginUnsupported) {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		h.writeControlFailure(writer, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleListPairings(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer) {
		return
	}
	query := request.URL.Query()
	items, err := h.control.ListPairings(request.Context(), currentOwnerUserID(request), channelspkg.PairingQuery{
		ChannelType: query.Get("channel_type"),
		Status:      query.Get("status"),
		AgentID:     query.Get("agent_id"),
	})
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, items)
}

func (h *Handlers) HandleCreatePairing(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer) {
		return
	}
	var payload channelspkg.CreatePairingRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.control.CreatePairing(request.Context(), currentOwnerUserID(request), payload)
	if errors.Is(err, channelspkg.ErrChannelNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.writeControlFailure(writer, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleUpdatePairing(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer) {
		return
	}
	var payload channelspkg.UpdatePairingRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.control.UpdatePairing(
		request.Context(),
		currentOwnerUserID(request),
		chi.URLParam(request, "pairing_id"),
		payload,
	)
	if errors.Is(err, channelspkg.ErrPairingNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.writeControlFailure(writer, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleDeletePairing(writer http.ResponseWriter, request *http.Request) {
	if !h.ensureControl(writer) {
		return
	}
	err := h.control.DeletePairing(request.Context(), currentOwnerUserID(request), chi.URLParam(request, "pairing_id"))
	if errors.Is(err, channelspkg.ErrPairingNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, map[string]any{"success": true})
}

func (h *Handlers) ensureControl(writer http.ResponseWriter) bool {
	if h.control != nil {
		return true
	}
	h.api.WriteFailure(writer, http.StatusServiceUnavailable, "channel control is not configured")
	return false
}

func (h *Handlers) writeControlFailure(writer http.ResponseWriter, err error) {
	message := err.Error()
	switch {
	case strings.Contains(message, "required"),
		strings.Contains(message, "invalid"),
		strings.Contains(message, "不能为空"),
		strings.Contains(message, "CONNECTOR_CREDENTIALS_KEY"):
		h.api.WriteFailure(writer, http.StatusBadRequest, message)
	default:
		h.api.WriteFailure(writer, http.StatusInternalServerError, message)
	}
}

func currentOwnerUserID(request *http.Request) string {
	return authsvc.OwnerUserID(request.Context())
}
