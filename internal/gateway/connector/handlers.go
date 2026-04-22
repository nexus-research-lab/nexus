package connector

import (
	"net/http"
	"strings"

	connectorsvc "github.com/nexus-research-lab/nexus/internal/connectors"
	gatewayshared "github.com/nexus-research-lab/nexus/internal/gateway/shared"

	"github.com/go-chi/chi/v5"
)

// Handlers 封装连接器域 HTTP handlers。
type Handlers struct {
	api        *gatewayshared.API
	connectors *connectorsvc.Service
}

// New 创建连接器 handlers。
func New(api *gatewayshared.API, connectors *connectorsvc.Service) *Handlers {
	return &Handlers{
		api:        api,
		connectors: connectors,
	}
}

func (h *Handlers) HandleListConnectors(writer http.ResponseWriter, request *http.Request) {
	items, err := h.connectors.ListConnectors(
		request.Context(),
		request.URL.Query().Get("q"),
		request.URL.Query().Get("category"),
		request.URL.Query().Get("status"),
	)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, items)
}

func (h *Handlers) HandleConnectorCategories(writer http.ResponseWriter, request *http.Request) {
	h.api.WriteSuccess(writer, h.connectors.GetCategories())
}

func (h *Handlers) HandleConnectorCount(writer http.ResponseWriter, request *http.Request) {
	count, err := h.connectors.GetConnectedCount(request.Context())
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, map[string]any{"count": count})
}

func (h *Handlers) HandleConnectorDetail(writer http.ResponseWriter, request *http.Request) {
	item, err := h.connectors.GetConnectorDetail(request.Context(), chi.URLParam(request, "connector_id"))
	if strings.Contains(strings.ToLower(gatewayshared.ErrString(err)), "not found") {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleConnectorAuthURL(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	connectorID := chi.URLParam(request, "connector_id")
	allowedExtraKeys := h.connectors.RequiredExtraKeys(connectorID)
	extras := make(map[string]string, len(allowedExtraKeys))
	for _, key := range allowedExtraKeys {
		values := query[key]
		if len(values) == 0 {
			continue
		}
		extras[key] = values[0]
	}
	item, err := h.connectors.GetAuthURL(request.Context(), connectorID, query.Get("redirect_uri"), extras)
	if strings.Contains(strings.ToLower(gatewayshared.ErrString(err)), "未知连接器") {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleConnectorOAuthCallback(writer http.ResponseWriter, request *http.Request) {
	var payload connectorsvc.OAuthCallbackRequest
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.connectors.CompleteOAuthCallback(request.Context(), payload)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleConnectConnector(writer http.ResponseWriter, request *http.Request) {
	var payload map[string]string
	if !h.api.BindJSONAllowEmpty(writer, request, &payload) {
		return
	}
	item, err := h.connectors.Connect(request.Context(), chi.URLParam(request, "connector_id"), payload)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "未知连接器") {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

func (h *Handlers) HandleDisconnectConnector(writer http.ResponseWriter, request *http.Request) {
	item, err := h.connectors.Disconnect(request.Context(), chi.URLParam(request, "connector_id"))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "未知连接器") {
			h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
			return
		}
		h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}
