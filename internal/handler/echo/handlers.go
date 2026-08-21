// INPUT: Echo HTTP 请求与结构化 DM session key。
// OUTPUT: owner-scoped 开关和会话覆盖 JSON。
// POS: Echo 的 HTTP 适配层。
package echo

import (
	"database/sql"
	"errors"
	"net/http"

	echodomain "github.com/nexus-research-lab/nexus/internal/echo"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	echosvc "github.com/nexus-research-lab/nexus/internal/service/echo"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"

	"github.com/go-chi/chi/v5"
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

// HandleGetSessionEcho 返回当前 DM 的 Echo 覆盖。
func (h *Handlers) HandleGetSessionEcho(writer http.ResponseWriter, request *http.Request) {
	override, err := h.echo.GetSessionOverride(request.Context(), chi.URLParam(request, "session_key"))
	if h.writeError(writer, err) {
		return
	}
	h.api.WriteSuccess(writer, override)
}

// HandleUpdateSessionEcho 更新当前 DM 的 Echo 覆盖。
func (h *Handlers) HandleUpdateSessionEcho(writer http.ResponseWriter, request *http.Request) {
	var payload echodomain.SessionOverride
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}
	override, err := h.echo.UpdateSessionOverride(
		request.Context(),
		chi.URLParam(request, "session_key"),
		payload,
	)
	if h.writeError(writer, err) {
		return
	}
	h.api.WriteSuccess(writer, override)
}

func (h *Handlers) writeError(writer http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, echodomain.ErrInvalidSessionMode),
		errors.Is(err, echodomain.ErrUnsupportedSession),
		handlershared.IsStructuredSessionKeyError(err):
		h.api.WriteFailure(writer, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, sql.ErrNoRows),
		errors.Is(err, sessionsvc.ErrSessionNotFound):
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
	default:
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
	}
	return true
}
