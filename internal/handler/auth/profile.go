// INPUT: 当前认证主体、个人资料请求与密码 exact request/终态回执。
// OUTPUT: 个人资料读写、密码原子修改、回执核对与 unknown 请求放弃 HTTP 投影。
// POS: auth handler 的个人设置边界；不按错误文本猜测数据影响。
package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	subscriptionsvc "github.com/nexus-research-lab/nexus/internal/service/subscription"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
)

type authChangePasswordPayload struct {
	RequestID       string `json:"request_id"`
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type passwordChangeReceiptPayload struct {
	RequestID string                        `json:"request_id"`
	Effect    authsvc.PasswordChangeOutcome `json:"effect"`
}

type authUpdateProfilePayload struct {
	Avatar *string `json:"avatar,omitempty"`
}

type personalProfilePayload struct {
	User              personalUserPayload  `json:"user"`
	TokenUsage        usagesvc.Summary     `json:"token_usage"`
	Subscription      *subscriptionPayload `json:"subscription,omitempty"`
	CanChangePassword bool                 `json:"can_change_password"`
	CanUpdateProfile  bool                 `json:"can_update_profile"`
}

type personalUserPayload struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Avatar      string `json:"avatar"`
	AuthMethod  string `json:"auth_method"`
}

type subscriptionPayload struct {
	PlanKey           string   `json:"plan_key"`
	PlanName          string   `json:"plan_name"`
	MonthlyTokenLimit *int64   `json:"monthly_token_limit"`
	UsedTokens        int64    `json:"used_tokens"`
	UsedPercent       *float64 `json:"used_percent"`
	PeriodStart       string   `json:"period_start"`
	PeriodEnd         string   `json:"period_end"`
}

type tokenUsageStore interface {
	Summary(ctx context.Context, ownerUserID string) (usagesvc.Summary, error)
}

type subscriptionStore interface {
	CurrentAccount(ctx context.Context, ownerUserID string) (*subscriptionsvc.Account, error)
}

// HandlePersonalProfile 返回当前用户的个人设置资料。
func (h *Handlers) HandlePersonalProfile(writer http.ResponseWriter, request *http.Request) {
	usage, err := h.buildTokenUsageSummary(request.Context())
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	subscription, err := h.buildSubscriptionSummary(request.Context())
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	applySubscriptionQuota(&usage, subscription)

	principal := authsvc.PrincipalFromContext(request.Context())
	h.api.WriteSuccess(writer, personalProfilePayload{
		User:              buildPersonalUserPayload(principal),
		TokenUsage:        usage,
		Subscription:      subscription,
		CanChangePassword: principal != nil && principal.AuthMethod == authsvc.AuthMethodPassword,
		CanUpdateProfile:  canUpdatePersonalProfile(principal),
	})
}

// HandleUpdatePersonalProfile 更新当前用户的个人资料。
func (h *Handlers) HandleUpdatePersonalProfile(writer http.ResponseWriter, request *http.Request) {
	if h.auth == nil {
		h.api.WriteFailure(writer, http.StatusServiceUnavailable, "auth service is not configured")
		return
	}
	principal := authsvc.PrincipalFromContext(request.Context())
	if !canUpdatePersonalProfile(principal) {
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
		if handlershared.IsClientMessageError(err) {
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
	subscription, err := h.buildSubscriptionSummary(request.Context())
	if err != nil {
		h.api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
		return
	}
	applySubscriptionQuota(&usage, subscription)
	h.api.WriteSuccess(writer, personalProfilePayload{
		User:              buildPersonalUserPayload(buildPrincipalFromUser(updatedUser, principal.AuthMethod)),
		TokenUsage:        usage,
		Subscription:      subscription,
		CanChangePassword: principal.AuthMethod == authsvc.AuthMethodPassword,
		CanUpdateProfile:  canUpdatePersonalProfile(principal),
	})
}

// HandleChangePassword 修改当前登录用户密码。
func (h *Handlers) HandleChangePassword(writer http.ResponseWriter, request *http.Request) {
	if h.auth == nil {
		h.api.WriteError(writer, request, http.StatusServiceUnavailable, passwordServiceUnavailableFailure())
		return
	}
	principal := authsvc.PrincipalFromContext(request.Context())
	if principal == nil || principal.AuthMethod != authsvc.AuthMethodPassword {
		h.api.WriteError(writer, request, http.StatusUnauthorized, passwordAccessFailure(
			"auth.password_change_unsupported",
			protocol.FailureCategoryAuthentication,
		))
		return
	}

	var payload authChangePasswordPayload
	if !h.api.BindJSONError(writer, request, &payload, passwordBodyFailure()) {
		return
	}

	_, err := h.auth.ChangePassword(request.Context(), authsvc.ChangePasswordInput{
		UserID:          principal.UserID,
		RequestID:       payload.RequestID,
		CurrentPassword: payload.CurrentPassword,
		NewPassword:     payload.NewPassword,
	})
	if err != nil {
		status, failure := passwordMutationFailure(err)
		h.api.WriteError(writer, request, status, failure)
		return
	}

	status, err := h.auth.BuildStatusPayload(request.Context(), request)
	if err != nil {
		h.api.WriteError(writer, request, http.StatusInternalServerError, handlershared.FailureSpec{
			Code:     "auth.password_change_committed",
			Category: protocol.FailureCategoryInternal,
			Effect:   protocol.FailureEffectCommitted,
			Detail:   "密码已修改，但暂时无法刷新登录状态",
			Cause:    err,
		})
		return
	}
	h.api.WriteSuccess(writer, status)
}

// HandlePasswordChangeReceipt 核对 exact request 是否已与密码凭据同事务提交。
func (h *Handlers) HandlePasswordChangeReceipt(writer http.ResponseWriter, request *http.Request) {
	if h.auth == nil {
		h.api.WriteError(writer, request, http.StatusServiceUnavailable, passwordReceiptReadFailure(
			"auth.password_receipt_unavailable",
			protocol.FailureCategoryUnavailable,
			nil,
		))
		return
	}
	principal := authsvc.PrincipalFromContext(request.Context())
	if principal == nil || principal.AuthMethod != authsvc.AuthMethodPassword {
		h.api.WriteError(writer, request, http.StatusUnauthorized, passwordReceiptReadFailure(
			"auth.password_receipt_unsupported",
			protocol.FailureCategoryAuthentication,
			nil,
		))
		return
	}
	requestID := strings.TrimSpace(request.URL.Query().Get("request_id"))
	outcome, err := h.auth.PasswordChangeOutcome(request.Context(), principal.UserID, requestID)
	if err != nil {
		status := http.StatusInternalServerError
		category := protocol.FailureCategoryUnavailable
		if errors.Is(err, authsvc.ErrPasswordChangeInvalidInput) {
			status = http.StatusBadRequest
			category = protocol.FailureCategoryValidation
		}
		h.api.WriteError(writer, request, status, passwordReceiptReadFailure(
			"auth.password_receipt_read_failed",
			category,
			err,
		))
		return
	}
	h.api.WriteSuccess(writer, passwordChangeReceiptPayload{
		RequestID: requestID,
		Effect:    outcome,
	})
}

// HandleSettlePasswordChangeNotApplied 放弃 unknown exact request，并原子阻止其迟到写入。
func (h *Handlers) HandleSettlePasswordChangeNotApplied(writer http.ResponseWriter, request *http.Request) {
	if h.auth == nil {
		h.api.WriteError(writer, request, http.StatusServiceUnavailable, passwordSettlementFailure(nil))
		return
	}
	principal := authsvc.PrincipalFromContext(request.Context())
	if principal == nil || principal.AuthMethod != authsvc.AuthMethodPassword {
		h.api.WriteError(writer, request, http.StatusUnauthorized, passwordAccessFailure(
			"auth.password_receipt_unsupported",
			protocol.FailureCategoryAuthentication,
		))
		return
	}
	var payload struct {
		RequestID string `json:"request_id"`
	}
	if !h.api.BindJSONError(writer, request, &payload, passwordBodyFailure()) {
		return
	}
	requestID := strings.TrimSpace(payload.RequestID)
	outcome, err := h.auth.SettlePasswordChangeNotApplied(
		request.Context(),
		principal.UserID,
		requestID,
	)
	if err != nil {
		status := http.StatusInternalServerError
		failure := passwordSettlementFailure(err)
		if errors.Is(err, authsvc.ErrPasswordChangeInvalidInput) {
			status = http.StatusBadRequest
			failure = passwordBodyFailure()
		}
		h.api.WriteError(writer, request, status, failure)
		return
	}
	h.api.WriteSuccess(writer, passwordChangeReceiptPayload{
		RequestID: requestID,
		Effect:    outcome,
	})
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

func canUpdatePersonalProfile(principal *authsvc.Principal) bool {
	if principal == nil {
		return false
	}
	return principal.AuthMethod == authsvc.AuthMethodPassword || principal.AuthMethod == authsvc.AuthMethodLocal
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
		return h.usage.Summary(ctx, authsvc.OwnerUserID(ctx))
	}
	return usagesvc.Summary{
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (h *Handlers) buildSubscriptionSummary(ctx context.Context) (*subscriptionPayload, error) {
	if h.subscription == nil {
		return nil, nil
	}
	account, err := h.subscription.CurrentAccount(ctx, authsvc.OwnerUserID(ctx))
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, nil
	}
	return &subscriptionPayload{
		PlanKey:           account.PlanKey,
		PlanName:          account.PlanName,
		MonthlyTokenLimit: account.MonthlyTokenLimit,
		UsedTokens:        account.UsedTokens,
		UsedPercent:       account.UsedPercent,
		PeriodStart:       account.PeriodStart,
		PeriodEnd:         account.PeriodEnd,
	}, nil
}

func applySubscriptionQuota(usage *usagesvc.Summary, subscription *subscriptionPayload) {
	if usage == nil || subscription == nil {
		return
	}
	usage.QuotaLimitTokens = subscription.MonthlyTokenLimit
}
