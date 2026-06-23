package provider

import (
	"database/sql"
	"strings"
)

func scanEntity(scanner interface {
	Scan(dest ...any) error
}) (Entity, error) {
	var item Entity
	var ownerUserID sql.NullString
	var lastTestStatus sql.NullString
	var lastTestError sql.NullString
	var lastTestAt sql.NullTime
	err := scanner.Scan(
		&item.ID,
		&ownerUserID,
		&item.Visibility,
		&item.ProviderKind,
		&item.Provider,
		&item.PresetKey,
		&item.APIFormat,
		&item.DisplayName,
		&item.AuthToken,
		&item.BaseURL,
		&item.ModelsPath,
		&item.Enabled,
		&lastTestStatus,
		&lastTestError,
		&lastTestAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return Entity{}, err
	}
	if ownerUserID.Valid {
		item.OwnerUserID = strings.TrimSpace(ownerUserID.String)
	}
	item.Visibility = strings.TrimSpace(item.Visibility)
	if item.Visibility == "" {
		item.Visibility = VisibilityPublic
	}
	item.Provider = strings.TrimSpace(item.Provider)
	item.ProviderKind = strings.TrimSpace(item.ProviderKind)
	if item.ProviderKind == "" {
		item.ProviderKind = "llm"
	}
	item.PresetKey = strings.TrimSpace(item.PresetKey)
	if item.PresetKey == "" {
		item.PresetKey = "custom"
	}
	item.APIFormat = strings.TrimSpace(item.APIFormat)
	if item.APIFormat == "" {
		item.APIFormat = "anthropic_messages"
	}
	item.DisplayName = strings.TrimSpace(item.DisplayName)
	item.AuthToken = strings.TrimSpace(item.AuthToken)
	item.BaseURL = strings.TrimSpace(item.BaseURL)
	item.ModelsPath = strings.TrimSpace(item.ModelsPath)
	if item.ModelsPath == "" &&
		item.APIFormat != apiFormatDashScopeImageGeneration &&
		item.APIFormat != apiFormatModelScopeImageGeneration {
		item.ModelsPath = "/v1/models"
	}
	item.LastTestStatus = strings.TrimSpace(lastTestStatus.String)
	item.LastTestError = strings.TrimSpace(lastTestError.String)
	if lastTestAt.Valid {
		value := lastTestAt.Time.UTC()
		item.LastTestAt = &value
	}
	item.CreatedAt = item.CreatedAt.UTC()
	item.UpdatedAt = item.UpdatedAt.UTC()
	return item, nil
}

func nullableOwnerUserID(item Entity) any {
	ownerUserID := strings.TrimSpace(item.OwnerUserID)
	if strings.TrimSpace(item.Visibility) != VisibilityPrivate || ownerUserID == "" {
		return nil
	}
	return ownerUserID
}

func scanModelEntity(scanner interface {
	Scan(dest ...any) error
}) (ModelEntity, error) {
	var item ModelEntity
	var contextWindow sql.NullInt64
	var maxOutputTokens sql.NullInt64
	err := scanner.Scan(
		&item.ID,
		&item.ProviderID,
		&item.ModelID,
		&item.DisplayName,
		&item.Category,
		&item.Enabled,
		&item.IsDefault,
		&item.CapabilitiesAutoJSON,
		&item.CapabilitiesOverrideJSON,
		&contextWindow,
		&maxOutputTokens,
		&item.ProviderOptionsJSON,
		&item.LastSeenAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return ModelEntity{}, err
	}
	item.ProviderID = strings.TrimSpace(item.ProviderID)
	item.ModelID = strings.TrimSpace(item.ModelID)
	item.DisplayName = strings.TrimSpace(item.DisplayName)
	if item.DisplayName == "" {
		item.DisplayName = item.ModelID
	}
	item.Category = strings.TrimSpace(item.Category)
	item.CapabilitiesAutoJSON = normalizeJSONText(item.CapabilitiesAutoJSON)
	item.CapabilitiesOverrideJSON = normalizeJSONText(item.CapabilitiesOverrideJSON)
	item.ProviderOptionsJSON = normalizeJSONText(item.ProviderOptionsJSON)
	if contextWindow.Valid {
		value := int(contextWindow.Int64)
		item.ContextWindow = &value
	}
	if maxOutputTokens.Valid {
		value := int(maxOutputTokens.Int64)
		item.MaxOutputTokens = &value
	}
	item.LastSeenAt = item.LastSeenAt.UTC()
	item.CreatedAt = item.CreatedAt.UTC()
	item.UpdatedAt = item.UpdatedAt.UTC()
	return item, nil
}

func normalizeJSONText(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "{}"
	}
	return trimmed
}
