// INPUT: 当前认证 owner、自定义 MCP JSON 请求、启停请求与路径中的 Connector ID。
// OUTPUT: 脱敏 MCP 配置、固定/自定义工具目录或明确的校验、冲突与不存在响应。
// POS: 自定义 MCP 管理及固定 Connector 只读能力发现的 HTTP API 边界。
package connector

import (
	"errors"
	"net/http"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"

	"github.com/go-chi/chi/v5"
)

type customMCPEnabledPayload struct {
	Enabled bool `json:"enabled"`
}

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

// HandleGetCustomMCPServer 返回当前 owner 的单条脱敏自定义 MCP。
func (h *Handlers) HandleGetCustomMCPServer(writer http.ResponseWriter, request *http.Request) {
	item, err := h.connectors.GetCustomMCPServer(
		request.Context(),
		currentOwnerUserID(request),
		customMCPConnectorID(request),
	)
	if err != nil {
		writeCustomMCPError(h, writer, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleGetCustomMCPCapabilities 探测远程 MCP 暴露的标准能力目录。
func (h *Handlers) HandleGetCustomMCPCapabilities(writer http.ResponseWriter, request *http.Request) {
	item, err := h.connectors.DiscoverCustomMCPCapabilities(
		request.Context(),
		currentOwnerUserID(request),
		customMCPConnectorID(request),
	)
	if err != nil {
		if errors.Is(err, connectorsvc.ErrCustomMCPCapabilityUnavailable) {
			h.api.WriteFailure(writer, http.StatusBadGateway, "无法读取 MCP 能力")
			return
		}
		writeCustomMCPError(h, writer, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleGetConnectorMCPCapabilities 返回固定 Connector 当前 MCP tools/list 快照。
func (h *Handlers) HandleGetConnectorMCPCapabilities(writer http.ResponseWriter, request *http.Request) {
	item, err := h.connectors.DiscoverConnectorMCPCapabilities(
		request.Context(),
		currentOwnerUserID(request),
		chi.URLParam(request, "connector_id"),
	)
	if err != nil {
		if errors.Is(err, connectorsvc.ErrConnectorMCPCapabilityUnavailable) {
			h.api.WriteFailure(writer, http.StatusBadGateway, "无法读取 MCP 能力")
			return
		}
		status := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "not found") ||
			strings.Contains(strings.ToLower(err.Error()), "未知连接器") {
			status = http.StatusNotFound
		}
		h.api.WriteFailure(writer, status, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
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
		customMCPConnectorID(request),
		payload,
	)
	if err != nil {
		writeCustomMCPError(h, writer, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleSetCustomMCPServerEnabled 设置自定义 MCP 的 owner 级启停状态。
func (h *Handlers) HandleSetCustomMCPServerEnabled(writer http.ResponseWriter, request *http.Request) {
	var payload customMCPEnabledPayload
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	item, err := h.connectors.SetCustomMCPServerEnabled(
		request.Context(),
		currentOwnerUserID(request),
		customMCPConnectorID(request),
		payload.Enabled,
	)
	if err != nil {
		writeCustomMCPError(h, writer, err)
		return
	}
	h.api.WriteSuccess(writer, item)
}

// HandleDeleteCustomMCPServer 删除自定义 MCP。
func (h *Handlers) HandleDeleteCustomMCPServer(writer http.ResponseWriter, request *http.Request) {
	connectorID := customMCPConnectorID(request)
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

func customMCPConnectorID(request *http.Request) string {
	return handlershared.PathParam(request, "connector_id")
}

func writeCustomMCPError(h *Handlers, writer http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, connectorsvc.ErrCustomMCPServerNotFound) {
		status = http.StatusNotFound
	} else if errors.Is(err, connectorsvc.ErrCustomMCPServerNameConflict) {
		status = http.StatusConflict
	} else if errors.Is(err, connectorsvc.ErrCustomMCPServerRecoveryRequired) {
		status = http.StatusConflict
	} else if errors.Is(err, connectorsvc.ErrCustomMCPServerInvalid) {
		status = http.StatusBadRequest
	}
	h.api.WriteFailure(writer, status, err.Error())
}
