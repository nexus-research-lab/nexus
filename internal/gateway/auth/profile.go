package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	authsvc "github.com/nexus-research-lab/nexus/internal/auth"
	gatewayshared "github.com/nexus-research-lab/nexus/internal/gateway/shared"
	usagesvc "github.com/nexus-research-lab/nexus/internal/usage"
)

type authChangePasswordPayload struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type personalProfilePayload struct {
	User              personalUserPayload `json:"user"`
	TokenUsage        usagesvc.Summary    `json:"token_usage"`
	CanChangePassword bool                `json:"can_change_password"`
}

type personalUserPayload struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	AuthMethod  string `json:"auth_method"`
}

type tokenUsageStore interface {
	Summary(ctx context.Context, ownerUserID string) (usagesvc.Summary, error)
}

// HandlePersonalProfile 返回当前用户的个人设置资料。
func (h *Handlers) HandlePersonalProfile(writer http.ResponseWriter, request *http.Request) {
	usage, err := h.buildTokenUsageSummary(request.Context())
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}

	principal := authsvc.PrincipalFromContext(request.Context())
	h.api.WriteSuccess(writer, personalProfilePayload{
		User:              buildPersonalUserPayload(principal),
		TokenUsage:        usage,
		CanChangePassword: principal != nil && principal.AuthMethod == authsvc.AuthMethodPassword,
	})
}

// HandleChangePassword 修改当前登录用户密码。
func (h *Handlers) HandleChangePassword(writer http.ResponseWriter, request *http.Request) {
	if h.auth == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "auth service is not configured")
		return
	}
	principal := authsvc.PrincipalFromContext(request.Context())
	if principal == nil {
		h.api.WriteFailure(writer, http.StatusUnauthorized, "未登录或登录状态已过期")
		return
	}

	var payload authChangePasswordPayload
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}

	_, err := h.auth.ChangePassword(request.Context(), authsvc.ChangePasswordInput{
		UserID:          principal.UserID,
		CurrentPassword: payload.CurrentPassword,
		NewPassword:     payload.NewPassword,
	})
	if err != nil {
		switch {
		case errors.Is(err, authsvc.ErrInvalidCredentials):
			h.api.WriteFailure(writer, http.StatusUnprocessableEntity, "当前密码不正确")
		case gatewayshared.IsClientMessageError(err):
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
		default:
			h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		}
		return
	}

	status, err := h.auth.BuildStatusPayload(request.Context(), request)
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, status)
}

func buildPersonalUserPayload(principal *authsvc.Principal) personalUserPayload {
	if principal == nil {
		return personalUserPayload{
			UserID:      authsvc.SystemUserID,
			Username:    "local",
			DisplayName: "Local User",
			Role:        authsvc.RoleOwner,
			AuthMethod:  "",
		}
	}
	return personalUserPayload{
		UserID:      strings.TrimSpace(principal.UserID),
		Username:    strings.TrimSpace(principal.Username),
		DisplayName: strings.TrimSpace(principal.DisplayName),
		Role:        strings.TrimSpace(principal.Role),
		AuthMethod:  strings.TrimSpace(principal.AuthMethod),
	}
}

func (h *Handlers) buildTokenUsageSummary(ctx context.Context) (usagesvc.Summary, error) {
	if h.usage != nil {
		return h.usage.Summary(ctx, currentOwnerUserID(ctx))
	}
	return usagesvc.Summary{
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func currentOwnerUserID(ctx context.Context) string {
	principal := authsvc.PrincipalFromContext(ctx)
	if principal == nil || strings.TrimSpace(principal.UserID) == "" {
		return authsvc.SystemUserID
	}
	return strings.TrimSpace(principal.UserID)
}
