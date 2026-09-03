package auth

import (
	"net/http"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
)

// Handlers 封装认证域 HTTP handlers。
type Handlers struct {
	api          *handlershared.API
	auth         authsvc.Authority
	usage        tokenUsageStore
	subscription subscriptionStore
}

// New 创建认证域 handlers。
func New(
	api *handlershared.API,
	auth authsvc.Authority,
	usage tokenUsageStore,
	subscription subscriptionStore,
) *Handlers {
	return &Handlers{
		api:          api,
		auth:         auth,
		usage:        usage,
		subscription: subscription,
	}
}

// HandleAuthStatus 返回当前认证状态。
func (h *Handlers) HandleAuthStatus(writer http.ResponseWriter, request *http.Request) {
	if h.auth == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "auth service is not configured")
		return
	}
	status, err := h.auth.BuildStatusPayload(request.Context(), request)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, status)
}
