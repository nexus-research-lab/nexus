// INPUT: 当前认证 owner、自定义 MCP JSON 请求与路径中的 Connector ID。
// OUTPUT: 脱敏 MCP 配置或明确的校验、冲突与不存在响应。
// POS: 自定义 MCP HTTP API 边界。
package connector

import (
	"errors"
	"net/http"

	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"

	"github.com/go-chi/chi/v5"
)

// HandleListCustomMCPServers 列出当前 owner 的自定义 MCP。
func (h *Handlers) HandleListCustomMCPServers(writer http.ResponseWriter, request *http.Request) {
	items, err := h.connectors.ListCustomMCPServers(
		request.Context(),
		currentOwnerUserID(request),
	)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, items)
}

// HandleCreateCustomMCPServer 创建自定义 MCP。
func (h *Handlers) HandleCreateCustomMCPServer(writer http.ResponseWriter, request *http.Request) {
	var payload connectorsvc.CustomMCPServerInput
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.connectors.CreateCustomMCPServer(
		request.Context(),
		currentOwnerUserID(request),
		payload,
	)
	if err != nil {
		writeCustomMCPError(h, writer, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleUpdateCustomMCPServer 更新自定义 MCP。
func (h *Handlers) HandleUpdateCustomMCPServer(writer http.ResponseWriter, request *http.Request) {
	var payload connectorsvc.CustomMCPServerInput
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.connectors.UpdateCustomMCPServer(
		request.Context(),
		currentOwnerUserID(request),
		chi.URLParam(request, "connector_id"),
		payload,
	)
	if err != nil {
		writeCustomMCPError(h, writer, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleDeleteCustomMCPServer 删除自定义 MCP。
func (h *Handlers) HandleDeleteCustomMCPServer(writer http.ResponseWriter, request *http.Request) {
	connectorID := chi.URLParam(request, "connector_id")
	if err := h.connectors.DeleteCustomMCPServer(
		request.Context(),
		currentOwnerUserID(request),
		connectorID,
	); err != nil {
		writeCustomMCPError(h, writer, err)
		return
	}
	h.api.WriteSuccess(writer, map[string]string{"connector_id": connectorID})
}

func writeCustomMCPError(h *Handlers, writer http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, connectorsvc.ErrCustomMCPServerNotFound) {
		status = http.StatusNotFound
	} else if errors.Is(err, connectorsvc.ErrCustomMCPServerNameConflict) {
		status = http.StatusConflict
	} else if errors.Is(err, connectorsvc.ErrCustomMCPServerInvalid) {
		status = http.StatusBadRequest
	}
	h.api.WriteFailure(writer, status, err.Error())
}
