package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (a *ControlAuthority) controlEntitlement(
	ctx context.Context,
	deploymentID string,
	controlUserID string,
) (controlEntitlement, error) {
	var entitlement controlEntitlement
	path := fmt.Sprintf(
		"/internal/deployments/%s/users/%s/entitlement",
		url.PathEscape(deploymentID),
		url.PathEscape(controlUserID),
	)
	if err := a.call(ctx, http.MethodGet, path, nil, &entitlement); err != nil {
		return controlEntitlement{}, mapControlError(err)
	}
	entitlement.PlanKey = strings.TrimSpace(entitlement.PlanKey)
	entitlement.PlanName = strings.TrimSpace(entitlement.PlanName)
	entitlement.UpdatedAt = entitlement.UpdatedAt.UTC()
	if entitlement.PlanKey == "" || entitlement.PlanName == "" || entitlement.UpdatedAt.IsZero() ||
		(entitlement.MonthlyTokenLimit != nil && *entitlement.MonthlyTokenLimit < 0) {
		return controlEntitlement{}, fmt.Errorf("Control entitlement 无效")
	}
	return entitlement, nil
}
