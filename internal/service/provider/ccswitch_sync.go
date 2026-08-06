// INPUT: 已认证用户、CC Switch 只读候选与同步选择。
// OUTPUT: Nexus 私有 Provider/模型的幂等写入与可选默认模型。
// POS: CC Switch 同步用例编排层；外部凭据只在后端内存和 Nexus 凭据表间流动。
package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

// PreviewCCSwitch 只读检查本机 CC Switch 配置并返回脱敏预览。
func (s *Service) PreviewCCSwitch(ctx context.Context, input CCSwitchPreviewInput) (*CCSwitchPreview, error) {
	if !s.desktopMode {
		return nil, errors.New("CC Switch 同步仅支持 Nexus 桌面版")
	}
	source, err := s.readCCSwitchSource(ctx, input.ConfigDir)
	if isCCSwitchSourceMissing(err) {
		return &CCSwitchPreview{
			Detected:     false,
			ConfigDir:    source.configDir,
			DatabasePath: source.databasePath,
			Providers:    []CCSwitchProviderPreview{},
		}, nil
	}
	if err != nil {
		return nil, err
	}
	runtimeKind := normalizeRuntimeKind(input.RuntimeKind)
	preview := &CCSwitchPreview{
		Detected:      true,
		ConfigDir:     source.configDir,
		DatabasePath:  source.databasePath,
		SchemaVersion: source.schemaVersion,
		Providers:     make([]CCSwitchProviderPreview, 0, len(source.candidates)),
	}
	defaultTarget, err := s.defaultRuntimeSelectionForRuntime(ctx, runtimeKind)
	if err != nil {
		return nil, err
	}
	preview.NeedsDefault = defaultTarget == nil
	ownerUserID := ownerUserIDFromContext(ctx)
	for _, candidate := range source.candidates {
		item := candidate.preview
		item.CurrentRuntimeSupported = ccSwitchRuntimeSupports(item.APIFormat, runtimeKind)
		existing, lookupErr := s.repository.GetScopedByProvider(
			ctx,
			providerstore.VisibilityPrivate,
			ownerUserID,
			item.Provider,
		)
		if lookupErr != nil {
			return nil, lookupErr
		}
		item.Existing = existing != nil
		preview.ProviderCount++
		preview.ModelCount += len(item.Models)
		if item.CanSync {
			preview.ReadyCount++
		}
		if preview.RecommendedSource == "" && item.CanSync && item.Current &&
			item.CurrentRuntimeSupported && item.AppType == ccSwitchPreferredApp(runtimeKind) {
			preview.RecommendedSource = item.SourceKey
		}
		preview.Providers = append(preview.Providers, item)
	}
	if preview.RecommendedSource == "" {
		for _, item := range preview.Providers {
			if item.CanSync && item.Current && item.CurrentRuntimeSupported {
				preview.RecommendedSource = item.SourceKey
				break
			}
		}
	}
	if preview.RecommendedSource == "" {
		for _, item := range preview.Providers {
			if item.CanSync && item.CurrentRuntimeSupported {
				preview.RecommendedSource = item.SourceKey
				break
			}
		}
	}
	return preview, nil
}

// SyncCCSwitch 把选中的 CC Switch Provider 与模型同步到当前用户私有配置。
func (s *Service) SyncCCSwitch(ctx context.Context, input CCSwitchSyncInput) (*CCSwitchSyncResult, error) {
	if !s.desktopMode {
		return nil, errors.New("CC Switch 同步仅支持 Nexus 桌面版")
	}
	selected := normalizeCCSwitchSourceKeys(input.SourceKeys)
	if len(selected) == 0 {
		return nil, errors.New("请至少选择一个可同步服务")
	}
	source, err := s.readCCSwitchSource(ctx, input.ConfigDir)
	if err != nil {
		if isCCSwitchSourceMissing(err) {
			return nil, errors.New("未找到 CC Switch 数据库")
		}
		return nil, err
	}
	candidates := make([]ccSwitchCandidate, 0, len(selected))
	for _, candidate := range source.candidates {
		if !selected[candidate.preview.SourceKey] {
			continue
		}
		if !candidate.preview.CanSync {
			return nil, fmt.Errorf("%s 暂不能同步: %s", candidate.preview.Name, candidate.preview.Reason)
		}
		candidates = append(candidates, candidate)
	}
	if len(candidates) != len(selected) {
		return nil, errors.New("CC Switch 服务已变化，请重新检测后再同步")
	}

	result := &CCSwitchSyncResult{}
	providerIDs := make(map[string]string, len(candidates))
	for _, candidate := range candidates {
		providerID, created, modelCount, syncErr := s.syncCCSwitchCandidate(ctx, candidate)
		if syncErr != nil {
			return nil, syncErr
		}
		providerIDs[candidate.preview.SourceKey] = providerID
		if created {
			result.Created++
		} else {
			result.Updated++
		}
		result.ProviderCount++
		result.ModelCount += modelCount
	}
	if input.SetDefault {
		selection, selectErr := s.setCCSwitchDefault(ctx, candidates, providerIDs, input.RuntimeKind)
		if selectErr != nil {
			return nil, selectErr
		}
		result.DefaultSelection = selection
	}
	return result, nil
}

func (s *Service) syncCCSwitchCandidate(
	ctx context.Context,
	candidate ccSwitchCandidate,
) (string, bool, int, error) {
	now := s.now()
	ownerUserID := ownerUserIDFromContext(ctx)
	existing, err := s.repository.GetScopedByProvider(
		ctx,
		providerstore.VisibilityPrivate,
		ownerUserID,
		candidate.preview.Provider,
	)
	if err != nil {
		return "", false, 0, err
	}
	created := existing == nil
	item := providerstore.Entity{
		ID:           s.idFactory("provider"),
		OwnerUserID:  ownerUserID,
		Visibility:   providerstore.VisibilityPrivate,
		ProviderKind: ProviderKindLLM,
		Provider:     candidate.preview.Provider,
		PresetKey:    presetCustom,
		APIFormat:    candidate.preview.APIFormat,
		DisplayName:  candidate.preview.Name,
		AuthToken:    strings.TrimSpace(candidate.authToken),
		BaseURL:      strings.TrimRight(strings.TrimSpace(candidate.preview.BaseURL), "/"),
		ModelsPath:   ccSwitchModelsPath(candidate.preview.APIFormat),
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if existing != nil {
		item.ID = existing.ID
		item.CreatedAt = existing.CreatedAt
		item.LastTestStatus = existing.LastTestStatus
		item.LastTestError = existing.LastTestError
		item.LastTestAt = existing.LastTestAt
		if err = s.repository.Update(ctx, item); err != nil {
			return "", false, 0, err
		}
	} else if err = s.repository.Create(ctx, item); err != nil {
		return "", false, 0, err
	}
	models := make([]providerstore.ModelEntity, 0, len(candidate.preview.Models))
	for _, model := range candidate.preview.Models {
		models = append(models, providerstore.ModelEntity{
			ID:                       s.idFactory("provider_model"),
			ProviderID:               item.ID,
			ModelID:                  normalizeModelID(model.ModelID),
			DisplayName:              modelDisplayName(model.ModelID, model.DisplayName),
			Category:                 "chat",
			Enabled:                  true,
			IsDefault:                false,
			CapabilitiesAutoJSON:     encodeModelCapabilities(ccSwitchModelCapabilities(model.Capabilities)),
			CapabilitiesOverrideJSON: "{}",
			ContextWindow:            contextWindowOrKnown(model.ModelID, model.ContextWindow),
			ProviderOptionsJSON:      "{}",
			LastSeenAt:               now,
			CreatedAt:                now,
			UpdatedAt:                now,
		})
	}
	if err = s.repository.UpsertModels(ctx, models); err != nil {
		return "", false, 0, err
	}
	return item.ID, created, len(models), nil
}

func (s *Service) setCCSwitchDefault(
	ctx context.Context,
	candidates []ccSwitchCandidate,
	providerIDs map[string]string,
	runtimeKind string,
) (*ModelSelection, error) {
	runtimeKind = normalizeRuntimeKind(runtimeKind)
	var selected *ccSwitchCandidate
	preferredApp := ccSwitchPreferredApp(runtimeKind)
	for index := range candidates {
		candidate := &candidates[index]
		if candidate.preview.Current && candidate.preview.AppType == preferredApp &&
			ccSwitchRuntimeSupports(candidate.preview.APIFormat, runtimeKind) {
			selected = candidate
			break
		}
	}
	for index := range candidates {
		if selected != nil {
			break
		}
		candidate := &candidates[index]
		if !ccSwitchRuntimeSupports(candidate.preview.APIFormat, runtimeKind) {
			continue
		}
		if selected == nil || candidate.preview.Current {
			selected = candidate
		}
		if candidate.preview.Current {
			break
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("所选服务不支持当前 %s runtime，无法设为默认", runtimeKind)
	}
	modelID := normalizeModelID(selected.preview.DefaultModel)
	if modelID == "" {
		return nil, fmt.Errorf("%s 没有可设为默认的模型", selected.preview.Name)
	}
	providerID := providerIDs[selected.preview.SourceKey]
	if err := s.repository.UpdateDefaultModel(ctx, providerID, modelID, s.now()); err != nil {
		return nil, err
	}
	return &ModelSelection{
		Provider:            selected.preview.Provider,
		ProviderDisplayName: selected.preview.Name,
		Model:               modelID,
		ModelDisplayName:    ccSwitchModelDisplayName(selected.preview.Models, modelID),
	}, nil
}

func ccSwitchPreferredApp(runtimeKind string) string {
	if normalizeRuntimeKind(runtimeKind) == "claude" {
		return ccSwitchAppClaude
	}
	return ccSwitchAppCodex
}

func normalizeCCSwitchSourceKeys(items []string) map[string]bool {
	result := make(map[string]bool, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value != "" {
			result[value] = true
		}
	}
	return result
}

func ccSwitchModelsPath(apiFormat string) string {
	return resolvePreset(presetCustom).Format(apiFormat).ModelsPath
}

func ccSwitchModelCapabilities(items []string) ModelCapabilities {
	result := ModelCapabilities{}
	for _, item := range items {
		switch strings.ToLower(strings.TrimSpace(item)) {
		case "vision":
			result.Vision = boolPointer(true)
		case "tools":
			result.ToolCalling = boolPointer(true)
		case "reasoning":
			result.Reasoning = boolPointer(true)
		}
	}
	return result
}

func ccSwitchModelDisplayName(items []CCSwitchModelPreview, modelID string) string {
	for _, item := range items {
		if normalizeModelID(item.ModelID) == modelID {
			return modelDisplayName(item.ModelID, item.DisplayName)
		}
	}
	return modelID
}
