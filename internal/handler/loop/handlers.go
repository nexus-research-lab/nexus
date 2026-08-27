// INPUT: Loop catalog HTTP 查询、slug 与 locale。
// OUTPUT: 旧成功 envelope，以及显式 FailureCore 的只读失败响应。
// POS: Loop 只读 HTTP 试点；不参与 Loop 启动、Goal、Session 或任何业务 ID。
package loop

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
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
		h.api.WriteError(writer, request, http.StatusNotFound, handlershared.FailureSpec{
			Code:     "loop.not_found",
			Category: protocol.FailureCategoryNotFound,
			Effect:   protocol.FailureEffectNotApplicable,
			Detail:   "资源不存在",
			Resolution: &protocol.FailureResolution{
				Actor:  protocol.FailureRecoveryActorUser,
				Action: "loop.return_to_directory",
			},
		})
		return
	}
	if err != nil {
		h.api.WriteError(writer, request, http.StatusInternalServerError, handlershared.FailureSpec{
			Code:     "loop.catalog_unavailable",
			Category: protocol.FailureCategoryInternal,
			Effect:   protocol.FailureEffectNotApplicable,
			Detail:   "服务内部错误",
			Cause:    err,
		})
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
