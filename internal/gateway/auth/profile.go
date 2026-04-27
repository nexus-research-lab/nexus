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

type authUpdateProfilePayload struct {
	Avatar *string `json:"avatar,omitempty"`
}

type personalProfilePayload struct {
	User              personalUserPayload `json:"user"`
	TokenUsage        usagesvc.Summary    `json:"token_usage"`
	CanChangePassword bool                `json:"can_change_password"`
	CanUpdateProfile  bool                `json:"can_update_profile"`
}

type personalUserPayload struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Avatar      string `json:"avatar"`
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
		CanUpdateProfile:  principal != nil && principal.AuthMethod == authsvc.AuthMethodPassword,
	})
}

// HandleUpdatePersonalProfile 更新当前用户的个人资料。
func (h *Handlers) HandleUpdatePersonalProfile(writer http.ResponseWriter, request *http.Request) {
	if h.auth == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "auth service is not configured")
		return
	}
	principal := authsvc.PrincipalFromContext(request.Context())
	if principal == nil || principal.AuthMethod != authsvc.AuthMethodPassword {
		h.api.WriteFailure(writer, http.StatusUnauthorized, "当前登录方式不支持修改个人资料")
		return
	}

	var payload authUpdateProfilePayload
	if !h.api.BindJSON(writer, request, &payload) {
		return
	}

	if payload.Avatar == nil {
		h.api.WriteFailure(writer, http.StatusBadRequest, "缺少要更新的个人资料字段")
		return
	}

	updatedUser, err := h.auth.UpdateProfile(request.Context(), authsvc.UpdateProfileInput{
		UserID: principal.UserID,
		Avatar: payload.Avatar,
	})
	if err != nil {
		if gatewayshared.IsClientMessageError(err) {
			h.api.WriteFailure(writer, http.StatusBadRequest, err.Error())
			return
		}
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}

	usage, err := h.buildTokenUsageSummary(request.Context())
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(writer, personalProfilePayload{
		User:              buildPersonalUserPayload(buildPrincipalFromUser(updatedUser, authsvc.AuthMethodPassword)),
		TokenUsage:        usage,
		CanChangePassword: true,
		CanUpdateProfile:  true,
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
			Role:        authsvc.RoleMember,
			Avatar:      "",
			AuthMethod:  "",
		}
	}
	return personalUserPayload{
		UserID:      strings.TrimSpace(principal.UserID),
		Username:    strings.TrimSpace(principal.Username),
		DisplayName: strings.TrimSpace(principal.DisplayName),
		Role:        strings.TrimSpace(principal.Role),
		Avatar:      strings.TrimSpace(principal.Avatar),
		AuthMethod:  strings.TrimSpace(principal.AuthMethod),
	}
}

func buildPrincipalFromUser(user *authsvc.User, authMethod string) *authsvc.Principal {
	if user == nil {
		return nil
	}
	return &authsvc.Principal{
		UserID:      strings.TrimSpace(user.UserID),
		Username:    strings.TrimSpace(user.Username),
		DisplayName: strings.TrimSpace(user.DisplayName),
		Role:        strings.TrimSpace(user.Role),
		Avatar:      strings.TrimSpace(user.Avatar),
		AuthMethod:  strings.TrimSpace(authMethod),
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
