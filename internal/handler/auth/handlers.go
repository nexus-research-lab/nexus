package auth

import (
	"errors"
	"net/http"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
)

type authLoginPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Handlers 封装认证域 HTTP handlers。
type Handlers struct {
	api          *handlershared.API
	auth         *authsvc.Service
	usage        tokenUsageStore
	subscription subscriptionStore
}

// New 创建认证域 handlers。
func New(
	api *handlershared.API,
	auth *authsvc.Service,
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

// HandleAuthLogin 执行登录。
func (h *Handlers) HandleAuthLogin(writer http.ResponseWriter, request *http.Request) {
	if h.auth == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "auth service is not configured")
		return
	}

	var payload authLoginPayload
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}

	result, err := h.auth.Login(request.Context(), authsvc.LoginInput{
		Username:  payload.Username,
		Password:  payload.Password,
		ClientIP:  authsvc.ResolveClientIP(request),
		UserAgent: request.UserAgent(),
	})
	if err != nil {
		switch {
		case errors.Is(err, authsvc.ErrPasswordLoginDisabled):
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		case errors.Is(err, authsvc.ErrInvalidCredentials):
			h.api.WriteFailure(writer, http.StatusUnauthorized, err.Error())
		default:
			if handlershared.IsClientMessageError(err) {
				h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
				return
			}
			h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		}
		return
	}

	// Best-effort cleanup of the previous session; failures must not block login.
	if logoutErr := h.auth.Logout(request.Context(), h.auth.ExtractSessionToken(request)); logoutErr != nil {
		h.api.BaseLogger().Warn("failed to clear previous session during login", "error", logoutErr.Error())
	}

	http.SetCookie(writer, &http.Cookie{
		Name:     h.auth.CookieName(),
		Value:    result.SessionToken,
		MaxAge:   h.auth.SessionMaxAge(),
		Path:     h.auth.CookiePath(),
		HttpOnly: true,
		SameSite: h.auth.CookieSameSite(),
		Secure:   h.auth.CookieSecure(),
	})
	h.api.WriteSuccess(writer, result.Status)
}

// HandleAuthLogout 执行登出。
func (h *Handlers) HandleAuthLogout(writer http.ResponseWriter, request *http.Request) {
	if h.auth == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "auth service is not configured")
		return
	}

	if err := h.auth.Logout(request.Context(), h.auth.ExtractSessionToken(request)); err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}

	http.SetCookie(writer, &http.Cookie{
		Name:     h.auth.CookieName(),
		Value:    "",
		MaxAge:   -1,
		Path:     h.auth.CookiePath(),
		HttpOnly: true,
		SameSite: h.auth.CookieSameSite(),
		Secure:   h.auth.CookieSecure(),
	})

	status, err := h.auth.BuildStatusPayload(request.Context(), request)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, status)
}
