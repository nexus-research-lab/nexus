package subscription

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

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

func (h *Handlers) HandleOverview(w http.ResponseWriter, r *http.Request) {
	if !canManageSubscription(r) {
		h.api.WriteError(w, r, http.StatusForbidden, subscriptionAccessFailure(
			protocol.FailureEffectNotApplicable,
		))
		return
	}

	overview, err := h.subscription.Overview(r.Context())
	if err != nil {
		h.api.WriteError(w, r, http.StatusInternalServerError, subscriptionReadFailure(err))
		return
	}
	h.api.WriteSuccess(w, overview)
}

func (h *Handlers) HandleUpdateUserSubscription(w http.ResponseWriter, r *http.Request) {
	if !canManageSubscription(r) {
		h.api.WriteError(w, r, http.StatusForbidden, subscriptionAccessFailure(
			protocol.FailureEffectNotApplied,
		))
		return
	}

	ownerUserID := chi.URLParam(r, "user_id")
	var payload struct {
		PlanKey string `json:"plan_key"`
	}
	if !h.api.BindJSONError(w, r, &payload, subscriptionBodyFailure()) {
		return
	}

	overview, err := h.subscription.UpdateUserSubscription(r.Context(), subscriptionsvc.UpdateUserSubscriptionInput{
		OwnerUserID: ownerUserID,
		PlanKey:     payload.PlanKey,
	})
	if err != nil {
		status, failure := subscriptionMutationFailure("account_update", err)
		h.api.WriteError(w, r, status, failure)
		return
	}
	h.api.WriteSuccess(w, overview)
}

func (h *Handlers) HandleUpsertPlan(w http.ResponseWriter, r *http.Request) {
	if !canManageSubscription(r) {
		h.api.WriteError(w, r, http.StatusForbidden, subscriptionAccessFailure(
			protocol.FailureEffectNotApplied,
		))
		return
	}

	planKey := chi.URLParam(r, "plan_key")
	var payload struct {
		PlanKey           string `json:"plan_key"`
		DisplayName       string `json:"display_name"`
		Status            string `json:"status"`
		MonthlyTokenLimit *int64 `json:"monthly_token_limit"`
		Notes             string `json:"notes"`
		SortOrder         int    `json:"sort_order"`
	}
	if !h.api.BindJSONError(w, r, &payload, subscriptionBodyFailure()) {
		return
	}
	if planKey != "" {
		payload.PlanKey = planKey
	}

	overview, err := h.subscription.UpsertPlan(r.Context(), subscriptionsvc.UpsertPlanInput{
		PlanKey:           payload.PlanKey,
		DisplayName:       payload.DisplayName,
		Status:            payload.Status,
		MonthlyTokenLimit: payload.MonthlyTokenLimit,
		Notes:             payload.Notes,
		SortOrder:         payload.SortOrder,
	})
	if err != nil {
		operation := "plan_create"
		if planKey != "" {
			operation = "plan_update"
		}
		status, failure := subscriptionMutationFailure(operation, err)
		h.api.WriteError(w, r, status, failure)
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
