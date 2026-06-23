package provider

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

func ownerUserIDFromContext(ctx context.Context) string {
	return authctx.OwnerUserID(ctx)
}

func canManagePublicProviders(ctx context.Context) bool {
	principal := authctx.PrincipalFromContext(ctx)
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

func (s *Service) createVisibility(ctx context.Context, requested string) (string, string, error) {
	visibility, err := normalizeProviderVisibility(requested, canManagePublicProviders(ctx))
	if err != nil {
		return "", "", err
	}
	if visibility == providerstore.VisibilityPublic {
		return visibility, "", nil
	}
	return visibility, ownerUserIDFromContext(ctx), nil
}

func normalizeProviderVisibility(requested string, canManagePublic bool) (string, error) {
	switch strings.TrimSpace(requested) {
	case "":
		if canManagePublic {
			return providerstore.VisibilityPublic, nil
		}
		return providerstore.VisibilityPrivate, nil
	case providerstore.VisibilityPublic:
		if !canManagePublic {
			return "", errors.New("只有管理员可以创建公共 Provider")
		}
		return providerstore.VisibilityPublic, nil
	case providerstore.VisibilityPrivate:
		return providerstore.VisibilityPrivate, nil
	default:
		return "", errors.New("provider visibility 只支持 public 或 private")
	}
}

func (s *Service) requireProviderManagement(ctx context.Context, item providerstore.Entity) error {
	if item.Visibility != providerstore.VisibilityPublic {
		return nil
	}
	if canManagePublicProviders(ctx) {
		return nil
	}
	return errors.New("只有管理员可以维护公共 Provider")
}

func (s *Service) usageCountForMutation(ctx context.Context, item providerstore.Entity) (int, error) {
	if item.Visibility == providerstore.VisibilityPublic {
		return s.repository.UsageCountForPublic(ctx, item.Provider)
	}
	return s.repository.UsageCountForOwner(ctx, item.OwnerUserID, item.Provider)
}

func (s *Service) replaceRuntimeProviderForDelete(
	ctx context.Context,
	deleting providerstore.Entity,
	newProvider string,
	newModel string,
) (int, error) {
	if deleting.Visibility == providerstore.VisibilityPublic {
		return s.repository.ReplaceRuntimeProviderForPublic(ctx, deleting.Provider, newProvider, newModel)
	}
	return s.repository.ReplaceRuntimeProviderForOwner(ctx, deleting.OwnerUserID, deleting.Provider, newProvider, newModel)
}
