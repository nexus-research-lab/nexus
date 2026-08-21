// INPUT: Echo 用户级设置 HTTP 请求。
// OUTPUT: owner-scoped 全局开关 JSON。
// POS: Echo 的 HTTP 适配层。
package echo

import (
	"net/http"

	echodomain "github.com/nexus-research-lab/nexus/internal/echo"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	echosvc "github.com/nexus-research-lab/nexus/internal/service/echo"
)

// Handlers 封装 Echo HTTP handlers。
type Handlers struct {
	api  *handlershared.API
	echo *echosvc.Service
}

// New 创建 Echo handlers。
func New(api *handlershared.API, service *echosvc.Service) *Handlers {
	return &Handlers{api: api, echo: service}
}

// HandleGetEcho 返回当前用户的 Echo 全局开关。
func (h *Handlers) HandleGetEcho(writer http.ResponseWriter, request *http.Request) {
	settings, err := h.echo.GetSettings(request.Context())
	if h.writeError(writer, err) {
		return
	}
	h.api.WriteSuccess(writer, settings)
}

// HandleUpdateEcho 更新当前用户的 Echo 全局开关。
func (h *Handlers) HandleUpdateEcho(writer http.ResponseWriter, request *http.Request) {
	var payload struct {
		Enabled *bool `json:"enabled"`
	}
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	if payload.Enabled == nil {
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, "缺少 Echo 开关")
		return
	}
	settings, err := h.echo.UpdateSettings(
		request.Context(),
		echodomain.Settings{Enabled: *payload.Enabled},
	)
	if h.writeError(writer, err) {
		return
	}
	h.api.WriteSuccess(writer, settings)
}

func (h *Handlers) writeError(writer http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
	return true
}
