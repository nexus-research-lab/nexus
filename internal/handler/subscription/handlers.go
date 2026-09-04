package subscription

import (
	"net/http"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	subscriptionsvc "github.com/nexus-research-lab/nexus/internal/service/subscription"
)

type Handlers struct {
	api          *shared.API
	subscription *subscriptionsvc.Service
}

func New(api *shared.API, subscription *subscriptionsvc.Service) *Handlers {
	return &Handlers{api: api, subscription: subscription}
}

// HandleUsage 返回 Nexus 本地 token 用量；套餐和成员信息由 Web 从 Control 读取。
func (h *Handlers) HandleUsage(w http.ResponseWriter, r *http.Request) {
	if !canManageSubscription(r) {
		h.api.WriteError(w, r, http.StatusForbidden, shared.FailureSpec{
			Code:     "subscription.admin_forbidden",
			Category: protocol.FailureCategoryAuthorization,
			Effect:   protocol.FailureEffectNotApplicable,
			Detail:   "当前账号没有订阅用量查看权限",
		})
		return
	}
	overview, err := h.subscription.UsageOverview(r.Context())
	if err != nil {
		h.api.WriteError(w, r, http.StatusInternalServerError, shared.FailureSpec{
			Code:     "subscription.usage_read_failed",
			Category: protocol.FailureCategoryInternal,
			Effect:   protocol.FailureEffectNotApplicable,
			Detail:   "暂时无法读取本地订阅用量",
			Cause:    err,
		})
		return
	}
	h.api.WriteSuccess(w, overview)
}

func canManageSubscription(r *http.Request) bool {
	principal := authctx.PrincipalFromContext(r.Context())
	if principal == nil {
		return true
	}
	switch strings.TrimSpace(principal.Role) {
	case authctx.RoleOwner, authctx.RoleAdmin:
		return true
	default:
		return false
	}
}
