// INPUT: 当前认证主体与个人资料请求。
// OUTPUT: 个人设置读模型，以及 Desktop 本地头像更新。
// POS: auth handler 的个人设置边界；Web 账号写操作直达 Control。
package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	subscriptionsvc "github.com/nexus-research-lab/nexus/internal/service/subscription"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
)

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
	local, ok := h.auth.(*authsvc.LocalAuthority)
	if !ok {
		h.api.WriteFailure(writer, http.StatusNotFound, "个人资料写入由 nexus-control 提供")
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

	updatedPrincipal, err := local.UpdateLocalAvatar(request.Context(), *payload.Avatar)
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
		User:              buildPersonalUserPayload(updatedPrincipal),
		TokenUsage:        usage,
		Subscription:      subscription,
		CanChangePassword: false,
		CanUpdateProfile:  true,
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
	userID := strings.TrimSpace(principal.ControlUserID)
	if userID == "" {
		userID = strings.TrimSpace(principal.UserID)
	}
	return personalUserPayload{
		UserID:      userID,
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
