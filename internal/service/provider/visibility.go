package provider

import (
	"context"
	"fmt"
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

func requirePublicProviderManagement(ctx context.Context) error {
	if canManagePublicProviders(ctx) {
		return nil
	}
	return fmt.Errorf("%w: 只有管理员可以维护公共 Provider", ErrProviderManagementForbidden)
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
		return providerstore.VisibilityPrivate, nil
	case providerstore.VisibilityPublic:
		if !canManagePublic {
			return "", fmt.Errorf("%w: 只有管理员可以创建公共 Provider", ErrProviderManagementForbidden)
		}
		return providerstore.VisibilityPublic, nil
	case providerstore.VisibilityPrivate:
		return providerstore.VisibilityPrivate, nil
	default:
		return "", fmt.Errorf("%w: provider visibility 只支持 public 或 private", ErrInvalidInput)
	}
}

func (s *Service) requireProviderManagement(ctx context.Context, item providerstore.Entity) error {
	if item.Visibility != providerstore.VisibilityPublic {
		return nil
	}
	return requirePublicProviderManagement(ctx)
}

func (s *Service) runtimeBindingCountForMutation(ctx context.Context, item providerstore.Entity) (int, error) {
	if item.Visibility == providerstore.VisibilityPublic {
		return s.repository.RuntimeBindingCountForPublic(ctx, item.Provider)
	}
	return s.repository.RuntimeBindingCountForOwner(ctx, item.OwnerUserID, item.Provider)
}

// ValidateDefaultAgentSelection 确认用户选择的默认模型能够被当前 runtime 使用。
func (s *Service) ValidateDefaultAgentSelection(ctx context.Context, selection DefaultAgentSelection) error {
	return s.validateDefaultAgentSelection(ctx, selection, "")
}

// ReconcileDefaultAgentBindings 让 Nexus 主智能体始终跟随用户默认模型。
// 普通 Agent 的显式绑定是用户意图，即使暂时不可用也必须保留，以便 Provider 恢复后自动生效。
func (s *Service) ReconcileDefaultAgentBindings(ctx context.Context, selection DefaultAgentSelection) (int, error) {
	if err := s.ValidateDefaultAgentSelection(ctx, selection); err != nil {
		return 0, err
	}
	ownerUserID := ownerUserIDFromContext(ctx)
	bindings, err := s.repository.ListRuntimeBindingsByOwner(ctx, ownerUserID)
	if err != nil {
		return 0, err
	}
	toClear := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		if binding.IsMain && (binding.Provider != "" || binding.Model != "") {
			toClear = append(toClear, binding.AgentID)
		}
	}
	return s.repository.ClearRuntimeSelectionsByOwner(ctx, ownerUserID, toClear)
}

// validateProviderInvalidationFallback 防止停用当前全局默认模型后，令主智能体和临时回退的 Agent 无可用模型。
func (s *Service) validateProviderInvalidationFallback(ctx context.Context, item providerstore.Entity) error {
	ownerUserIDs := []string{item.OwnerUserID}
	if item.Visibility == providerstore.VisibilityPublic {
		owners, err := s.repository.ListActiveOwnerUserIDs(ctx)
		if err != nil {
			return err
		}
		ownerUserIDs = owners
	}
	for _, ownerUserID := range ownerUserIDs {
		affected, err := s.providerAffectsOwner(ctx, item, ownerUserID)
		if err != nil {
			return err
		}
		if !affected {
			continue
		}
		ownerContext := contextForOwner(ctx, ownerUserID)
		selection, err := s.defaultAgentSelectionForOwner(ownerContext, ownerUserID)
		if err != nil {
			return err
		}
		if err = s.validateDefaultAgentSelection(ownerContext, selection, item.Provider); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) providerAffectsOwner(ctx context.Context, item providerstore.Entity, ownerUserID string) (bool, error) {
	if item.Visibility != providerstore.VisibilityPublic {
		return strings.TrimSpace(item.OwnerUserID) == strings.TrimSpace(ownerUserID), nil
	}
	visible, err := s.repository.GetVisibleByProvider(ctx, ownerUserID, item.Provider)
	if err != nil {
		return false, err
	}
	return visible != nil && visible.ID == item.ID, nil
}

func (s *Service) defaultAgentSelectionForOwner(
	ctx context.Context,
	ownerUserID string,
) (DefaultAgentSelection, error) {
	if s.defaultAgentSelectionResolver == nil {
		return DefaultAgentSelection{RuntimeKind: "claude"}, nil
	}
	selection, err := s.defaultAgentSelectionResolver(ctx, ownerUserID)
	if err != nil {
		return DefaultAgentSelection{}, err
	}
	selection.Provider = strings.TrimSpace(selection.Provider)
	selection.Model = strings.TrimSpace(selection.Model)
	selection.RuntimeKind = strings.TrimSpace(selection.RuntimeKind)
	return selection, nil
}

func (s *Service) validateDefaultAgentSelection(
	ctx context.Context,
	selection DefaultAgentSelection,
	invalidProvider string,
) error {
	invalidProvider = strings.TrimSpace(invalidProvider)
	config, err := s.ResolveRuntimeConfigForRuntime(
		ctx,
		strings.TrimSpace(selection.Provider),
		strings.TrimSpace(selection.Model),
		strings.TrimSpace(selection.RuntimeKind),
	)
	if err != nil {
		return fmt.Errorf("默认模型不可用: %w", err)
	}
	if invalidProvider != "" && strings.TrimSpace(config.Provider) == invalidProvider {
		return fmt.Errorf(
			"默认模型仍使用 Provider %s；请先在设置中切换默认模型",
			invalidProvider,
		)
	}
	return nil
}

func contextForOwner(ctx context.Context, ownerUserID string) context.Context {
	principal := authctx.PrincipalFromContext(ctx)
	if principal == nil {
		return authctx.WithPrincipal(ctx, &authctx.Principal{
			UserID: strings.TrimSpace(ownerUserID),
			Role:   authctx.RoleOwner,
		})
	}
	copy := *principal
	copy.UserID = strings.TrimSpace(ownerUserID)
	return authctx.WithPrincipal(ctx, &copy)
}
