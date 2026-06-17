package loop

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	loopsvc "github.com/nexus-research-lab/nexus/internal/service/loops"
)

// Handlers 封装 loop catalog HTTP handlers。
type Handlers struct {
	api   *handlershared.API
	loops *loopsvc.Service
}

// New 创建 loop catalog handlers。
func New(api *handlershared.API, loops *loopsvc.Service) *Handlers {
	return &Handlers{api: api, loops: loops}
}

// HandleListLoops 返回内置 loops 列表。
func (h *Handlers) HandleListLoops(writer http.ResponseWriter, request *http.Request) {
	h.api.WriteSuccess(writer, h.loops.ListLoops(request.Context(), localeFromRequest(request)))
}

// HandleGetLoopDetail 返回单个 loop 详情。
func (h *Handlers) HandleGetLoopDetail(writer http.ResponseWriter, request *http.Request) {
	item, err := h.loops.GetLoop(request.Context(), chi.URLParam(request, "slug"), localeFromRequest(request))
	if errors.Is(err, loopsvc.ErrLoopNotFound) {
		h.api.WriteFailure(writer, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, item)
}

func localeFromRequest(request *http.Request) string {
	if locale := strings.TrimSpace(request.URL.Query().Get("locale")); locale != "" {
		return locale
	}
	return request.Header.Get("Accept-Language")
}
