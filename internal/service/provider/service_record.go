package provider

import (
	"context"
	"strings"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

func toRecord(
	ctx context.Context,
	item providerstore.Entity,
	usageCount int,
	usageAgents []providerstore.UsageAgentEntity,
	models []ModelRecord,
) Record {
	createdAt := item.CreatedAt
	updatedAt := item.UpdatedAt
	canManage := item.Visibility != providerstore.VisibilityPublic || canManagePublicProviders(ctx)
	authTokenMasked := maskToken(item.AuthToken)
	if !canManage {
		authTokenMasked = ""
	}
	return Record{
		ID:                    item.ID,
		OwnerUserID:           item.OwnerUserID,
		Visibility:            item.Visibility,
		ProviderKind:          item.ProviderKind,
		Provider:              item.Provider,
		PresetKey:             item.PresetKey,
		APIFormat:             item.APIFormat,
		DisplayName:           item.DisplayName,
		AuthTokenMasked:       authTokenMasked,
		BaseURL:               item.BaseURL,
		ModelsPath:            item.ModelsPath,
		Enabled:               item.Enabled,
		UsageCount:            usageCount,
		UsedByAgents:          toUsageAgents(usageAgents),
		LastTestStatus:        item.LastTestStatus,
		LastTestError:         item.LastTestError,
		LastTestAt:            item.LastTestAt,
		CanManage:             canManage,
		AgentRuntimeSupported: isAgentRuntimeProvider(item),
		Models:                models,
		CreatedAt:             &createdAt,
		UpdatedAt:             &updatedAt,
	}
}

func toUsageAgents(items []providerstore.UsageAgentEntity) []UsageAgent {
	result := make([]UsageAgent, 0, len(items))
	for _, item := range items {
		displayName := strings.TrimSpace(item.DisplayName)
		if displayName == "" {
			displayName = strings.TrimSpace(item.Name)
		}
		result = append(result, UsageAgent{
			AgentID:     strings.TrimSpace(item.AgentID),
			Name:        strings.TrimSpace(item.Name),
			DisplayName: displayName,
			Avatar:      strings.TrimSpace(item.Avatar),
			IsMain:      item.IsMain,
		})
	}
	return result
}

func maskToken(token string) string {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 10 {
		return strings.Repeat("*", len(trimmed))
	}
	return trimmed[:5] + strings.Repeat("*", 24) + trimmed[len(trimmed)-5:]
}
