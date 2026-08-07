// INPUT: owner 外部来源配置、来源健康结果与可选 expected catalog version。
// OUTPUT: 默认来源初始化、版本化开关更新与不递增功能版本的健康元数据。
// POS: Skill marketplace 来源配置的 owner mutation 边界。
package skills

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	skillstore "github.com/nexus-research-lab/nexus/internal/storage/skills"
)

func (s *Service) recordExternalSourceCheck(ctx context.Context, source externalSkillSource, lastError string) {
	if s.skillStore == nil || strings.TrimSpace(source.Key) == "" {
		return
	}
	_, err := s.withCatalogMutation(
		ctx,
		nil,
		false,
		func(mutation *skillstore.CatalogMutation) error {
			return mutation.RecordSourceCheck(
				ctx,
				source.Key,
				time.Now().UTC(),
				lastError,
			)
		},
	)
	if err != nil {
		slog.WarnContext(ctx, "记录 skill 来源检查状态失败", "source", source.Name, "err", err)
	}
}

// ListExternalSkillSources 返回当前用户可见的社区与私有 skill 来源配置。
func (s *Service) ListExternalSkillSources(ctx context.Context) ([]ExternalSkillSourceInfo, error) {
	configuredSources := s.configuredExternalSkillSources()
	if s.skillStore == nil {
		items := make([]ExternalSkillSourceInfo, 0, len(configuredSources))
		for _, source := range configuredSources {
			items = append(items, externalSkillSourceInfoFromSource(source))
		}
		return items, nil
	}
	if err := s.ensureConfiguredSkillSources(ctx, configuredSources); err != nil {
		return nil, err
	}
	rows, err := s.skillStore.ListSources(ctx, authctx.OwnerUserID(ctx))
	if err != nil {
		return nil, err
	}
	configuredIDs := configuredExternalSourceIDs(configuredSources)
	items := make([]ExternalSkillSourceInfo, 0, len(rows))
	for _, row := range rows {
		if !externalSourceVisible(row, configuredIDs) {
			continue
		}
		items = append(items, externalSkillSourceInfoFromEntity(row))
	}
	return items, nil
}

// UpdateExternalSkillSource 更新当前用户的 skill 来源配置。
func (s *Service) UpdateExternalSkillSource(ctx context.Context, sourceID string, request ExternalSkillSourceRequest) (*ExternalSkillSourceInfo, error) {
	return s.updateExternalSkillSource(ctx, sourceID, request, nil)
}

// UpdateExternalSkillSourceAtVersion 仅在 owner catalog version 匹配时更新来源开关。
func (s *Service) UpdateExternalSkillSourceAtVersion(
	ctx context.Context,
	sourceID string,
	request ExternalSkillSourceRequest,
	expectedVersion int64,
) (*ExternalSkillSourceInfo, error) {
	return s.updateExternalSkillSource(ctx, sourceID, request, &expectedVersion)
}

func (s *Service) updateExternalSkillSource(
	ctx context.Context,
	sourceID string,
	request ExternalSkillSourceRequest,
	expectedVersion *int64,
) (*ExternalSkillSourceInfo, error) {
	sourceID = strings.TrimSpace(sourceID)
	if s.skillStore == nil {
		return nil, errors.New("skill source store not configured")
	}
	configuredSources := s.configuredExternalSkillSources()
	if err := s.ensureConfiguredSkillSources(ctx, configuredSources); err != nil {
		return nil, err
	}
	ownerUserID := authctx.OwnerUserID(ctx)
	existing, err := s.skillStore.GetSource(ctx, ownerUserID, sourceID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("skill source not found")
	}
	if sourceManagedBy(*existing) == externalSourceManagedByUser {
		if existing.Kind != externalSourceKindPrivateRegistry {
			return nil, errors.New("不支持修改该用户来源类型")
		}
		return s.updatePrivateSkillSource(ctx, *existing, request)
	}
	if _, ok := configuredExternalSourceIDs(configuredSources)[sourceID]; !ok {
		return nil, errors.New("skill source not found")
	}
	if request.Name != nil || request.AuthType != nil || request.Token != nil {
		return nil, errors.New("系统来源只允许修改启用状态")
	}
	var row *skillstore.SourceEntity
	_, err = s.withCatalogMutation(
		ctx,
		expectedVersion,
		true,
		func(mutation *skillstore.CatalogMutation) error {
			current, loadErr := mutation.GetSource(ctx, sourceID)
			if loadErr != nil {
				return loadErr
			}
			if current == nil || sourceManagedBy(*current) != externalSourceManagedBySystem {
				return errors.New("skill source not found")
			}
			if request.Enabled != nil {
				current.Enabled = *request.Enabled
			}
			if updateErr := mutation.UpsertSource(ctx, *current); updateErr != nil {
				return updateErr
			}
			row = current
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errors.New("skill source not found")
	}
	item := externalSkillSourceInfoFromEntity(*row)
	return &item, nil
}

func configuredExternalSourceIDs(sources []externalSkillSource) map[string]struct{} {
	result := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		key := strings.TrimSpace(source.Key)
		if key == "" {
			continue
		}
		result[key] = struct{}{}
	}
	return result
}

func externalSourceVisible(row skillstore.SourceEntity, configuredIDs map[string]struct{}) bool {
	if sourceManagedBy(row) == externalSourceManagedByUser {
		return true
	}
	_, ok := configuredIDs[row.SourceID]
	return ok
}

func (s *Service) externalSkillSources(ctx context.Context) []externalSkillSource {
	configuredSources := s.configuredExternalSkillSources()
	if s.skillStore == nil {
		return configuredSources
	}
	if err := s.ensureConfiguredSkillSources(ctx, configuredSources); err != nil {
		slog.WarnContext(ctx, "初始化 skill 来源配置失败", "err", err)
		return configuredSources
	}
	rows, err := s.skillStore.ListEnabledSources(ctx, authctx.OwnerUserID(ctx))
	if err != nil {
		slog.WarnContext(ctx, "读取 skill 来源配置失败", "err", err)
		return configuredSources
	}
	configuredIDs := configuredExternalSourceIDs(configuredSources)
	sources := make([]externalSkillSource, 0, len(rows))
	for _, row := range rows {
		if !externalSourceVisible(row, configuredIDs) {
			continue
		}
		sources = append(sources, s.externalSkillSourceFromEntity(row))
	}
	return sources
}

func (s *Service) ensureConfiguredSkillSources(ctx context.Context, sources []externalSkillSource) error {
	if s.skillStore == nil {
		return nil
	}
	ownerUserID := authctx.OwnerUserID(ctx)
	_, err := s.withCatalogMutation(
		ctx,
		nil,
		false,
		func(mutation *skillstore.CatalogMutation) error {
			for _, source := range sources {
				if ensureErr := mutation.EnsureSource(ctx, skillstore.SourceEntity{
					OwnerUserID: ownerUserID,
					SourceID:    source.Key,
					Name:        source.Name,
					Kind:        source.Kind,
					URL:         source.URL,
					Trust:       firstNonEmpty(source.Trust, externalSourceTrustCommunity),
					ManagedBy:   externalSourceManagedBySystem,
					AuthType:    externalSourceAuthNone,
					Enabled:     source.Enabled,
					SortOrder:   source.SortOrder,
				}); ensureErr != nil {
					return ensureErr
				}
			}
			return nil
		},
	)
	return err
}

func externalSkillSourceInfoFromSource(source externalSkillSource) ExternalSkillSourceInfo {
	return ExternalSkillSourceInfo{
		SourceID:  source.Key,
		Name:      source.Name,
		Kind:      source.Kind,
		URL:       source.URL,
		Trust:     source.Trust,
		Enabled:   source.Enabled,
		SortOrder: source.SortOrder,
		ManagedBy: externalSourceManagedBySystem,
		AuthType:  externalSourceAuthNone,
		Deletable: false,
	}
}

func externalSkillSourceInfoFromEntity(entity skillstore.SourceEntity) ExternalSkillSourceInfo {
	managedBy := sourceManagedBy(entity)
	return ExternalSkillSourceInfo{
		SourceID:             entity.SourceID,
		Name:                 entity.Name,
		Kind:                 entity.Kind,
		URL:                  entity.URL,
		Trust:                entity.Trust,
		Enabled:              entity.Enabled,
		SortOrder:            entity.SortOrder,
		LastCheckedAt:        entity.LastCheckedAt,
		LastError:            entity.LastError,
		ManagedBy:            managedBy,
		AuthType:             firstNonEmpty(entity.AuthType, externalSourceAuthNone),
		CredentialConfigured: strings.TrimSpace(entity.CredentialsEncrypted) != "",
		Deletable:            managedBy == externalSourceManagedByUser && entity.Kind == externalSourceKindPrivateRegistry,
	}
}

func sourceManagedBy(entity skillstore.SourceEntity) string {
	if strings.TrimSpace(entity.ManagedBy) == externalSourceManagedByUser {
		return externalSourceManagedByUser
	}
	return externalSourceManagedBySystem
}

func (s *Service) externalSkillSourceFromEntity(entity skillstore.SourceEntity) externalSkillSource {
	source := externalSkillSource{
		Key:       entity.SourceID,
		Name:      entity.Name,
		Kind:      entity.Kind,
		URL:       entity.URL,
		Trust:     entity.Trust,
		Enabled:   entity.Enabled,
		SortOrder: entity.SortOrder,
		ManagedBy: sourceManagedBy(entity),
		AuthType:  firstNonEmpty(entity.AuthType, externalSourceAuthNone),
	}
	if strings.TrimSpace(entity.CredentialsEncrypted) == "" {
		return source
	}
	credential, err := s.decryptPrivateSourceCredential(entity.CredentialsEncrypted)
	if err != nil {
		source.CredentialError = err.Error()
		return source
	}
	source.Credential = credential
	return source
}
