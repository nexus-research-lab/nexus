package subscription

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
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
		h.api.WriteFailure(w, http.StatusForbidden, "subscription admin access required")
		return
	}

	overview, err := h.subscription.Overview(r.Context())
	if err != nil {
		h.api.WriteFailure(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(w, overview)
}

func (h *Handlers) HandleUpdateUserSubscription(w http.ResponseWriter, r *http.Request) {
	if !canManageSubscription(r) {
		h.api.WriteFailure(w, http.StatusForbidden, "subscription admin access required")
		return
	}

	ownerUserID := chi.URLParam(r, "user_id")
	var payload struct {
		PlanKey string `json:"plan_key"`
	}
	if !h.api.BindJSON(w, r, &payload) {
		return
	}

	overview, err := h.subscription.UpdateUserSubscription(r.Context(), subscriptionsvc.UpdateUserSubscriptionInput{
		OwnerUserID: ownerUserID,
		PlanKey:     payload.PlanKey,
	})
	if errors.Is(err, subscriptionsvc.ErrInvalidInput) {
		h.api.WriteFailure(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		h.api.WriteFailure(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.api.WriteSuccess(w, overview)
}

func (h *Handlers) HandleUpsertPlan(w http.ResponseWriter, r *http.Request) {
	if !canManageSubscription(r) {
		h.api.WriteFailure(w, http.StatusForbidden, "subscription admin access required")
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
	if !h.api.BindJSON(w, r, &payload) {
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
	if errors.Is(err, subscriptionsvc.ErrInvalidInput) {
		h.api.WriteFailure(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		h.api.WriteFailure(w, http.StatusInternalServerError, err.Error())
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
