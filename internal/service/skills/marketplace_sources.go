package skills

import (
	"context"
	"errors"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	skillstore "github.com/nexus-research-lab/nexus/internal/storage/skills"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"time"
)

func (s *Service) recordExternalSourceCheck(ctx context.Context, source externalSkillSource, lastError string) {
	if s.skillStore == nil || strings.TrimSpace(source.Key) == "" {
		return
	}
	if err := s.skillStore.RecordSourceCheck(ctx, authctx.OwnerUserID(ctx), source.Key, time.Now().UTC(), lastError); err != nil {
		slog.WarnContext(ctx, "记录 skill 来源检查状态失败", "source", source.Name, "err", err)
	}
}

// ListExternalSkillSources 返回当前用户的社区 skill 来源配置。
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
		if _, ok := configuredIDs[row.SourceID]; !ok {
			continue
		}
		items = append(items, externalSkillSourceInfoFromEntity(row))
	}
	return items, nil
}

// UpdateExternalSkillSource 更新当前用户的社区 skill 来源开关。
func (s *Service) UpdateExternalSkillSource(ctx context.Context, sourceID string, request ExternalSkillSourceRequest) (*ExternalSkillSourceInfo, error) {
	sourceID = strings.TrimSpace(sourceID)
	if s.skillStore == nil {
		return nil, errors.New("skill source store not configured")
	}
	configuredSources := s.configuredExternalSkillSources()
	if err := s.ensureConfiguredSkillSources(ctx, configuredSources); err != nil {
		return nil, err
	}
	if _, ok := configuredExternalSourceIDs(configuredSources)[sourceID]; !ok {
		return nil, errors.New("skill source not found")
	}
	ownerUserID := authctx.OwnerUserID(ctx)
	existing, err := s.skillStore.GetSource(ctx, ownerUserID, sourceID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("skill source not found")
	}
	enabled := existing.Enabled
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	entity := *existing
	entity.Enabled = enabled
	if err = s.skillStore.UpsertSource(ctx, entity); err != nil {
		return nil, err
	}
	row, err := s.skillStore.GetSource(ctx, ownerUserID, existing.SourceID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errors.New("skill source not found")
	}
	item := externalSkillSourceInfoFromEntity(*row)
	return &item, nil
}

func (s *Service) defaultExternalSkillSources() []externalSkillSource {
	skillsShURL := strings.TrimRight(firstNonEmpty(strings.TrimSpace(s.config.SkillsAPIURL), defaultSkillsShURL), "/")
	return []externalSkillSource{
		{
			Key:       buildSkillSourceID(externalSourceKindClaudePlugins, defaultClaudePluginsSearchURL),
			Name:      "claude-plugins.dev",
			Kind:      externalSourceKindClaudePlugins,
			URL:       defaultClaudePluginsSearchURL,
			Trust:     externalSourceTrustCommunity,
			Enabled:   true,
			SortOrder: 0,
		},
		{
			Key:       buildSkillSourceID(externalSourceKindSkillsSh, skillsShURL),
			Name:      "skills.sh",
			Kind:      externalSourceKindSkillsSh,
			URL:       skillsShURL,
			Trust:     externalSourceTrustCommunity,
			Enabled:   true,
			SortOrder: 10,
		},
		{
			Key:       buildSkillSourceID(externalSourceKindClawhub, defaultClawhubSearchURL),
			Name:      "clawhub.ai",
			Kind:      externalSourceKindClawhub,
			URL:       defaultClawhubSearchURL,
			Trust:     externalSourceTrustCommunity,
			Enabled:   true,
			SortOrder: 20,
		},
		{
			Key:       buildSkillSourceID(externalSourceKindBrowseSh, defaultBrowseShURL),
			Name:      "browse.sh",
			Kind:      externalSourceKindBrowseSh,
			URL:       defaultBrowseShURL,
			Trust:     externalSourceTrustCommunity,
			Enabled:   true,
			SortOrder: 30,
		},
		{
			Key:       buildSkillSourceID(externalSourceKindHermesIndex, defaultHermesIndexURL),
			Name:      "Hermes Skills Index",
			Kind:      externalSourceKindHermesIndex,
			URL:       defaultHermesIndexURL,
			Trust:     externalSourceTrustCommunity,
			Enabled:   false,
			SortOrder: 40,
		},
	}
}

func (s *Service) configuredExternalSkillSources() []externalSkillSource {
	sources := make([]externalSkillSource, 0)
	if s.config.SkillsDefaultSourcesEnabled {
		sources = append(sources, s.defaultExternalSkillSources()...)
	} else if apiURL := strings.TrimRight(strings.TrimSpace(s.config.SkillsAPIURL), "/"); apiURL != "" {
		sources = append(sources, externalSkillSource{
			Key:       buildSkillSourceID(externalSourceKindSkillsSh, apiURL),
			Name:      "skills.sh",
			Kind:      externalSourceKindSkillsSh,
			URL:       apiURL,
			Trust:     externalSourceTrustCommunity,
			Enabled:   true,
			SortOrder: 0,
		})
	}
	for index, raw := range splitExternalSourceList(s.config.SkillsSourceURLs) {
		source, ok := parseConfiguredExternalSource(raw)
		if !ok {
			continue
		}
		source.SortOrder = 100 + index*10
		sources = append(sources, source)
	}
	seen := map[string]struct{}{}
	result := make([]externalSkillSource, 0, len(sources))
	for _, source := range sources {
		if source.Key == "" {
			continue
		}
		if _, ok := seen[source.Key]; ok {
			continue
		}
		seen[source.Key] = struct{}{}
		result = append(result, source)
	}
	sort.SliceStable(result, func(i int, j int) bool {
		if result[i].SortOrder != result[j].SortOrder {
			return result[i].SortOrder < result[j].SortOrder
		}
		return result[i].Name < result[j].Name
	})
	return result
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
		if _, ok := configuredIDs[row.SourceID]; !ok {
			continue
		}
		sources = append(sources, externalSkillSource{
			Key:       row.SourceID,
			Name:      row.Name,
			Kind:      row.Kind,
			URL:       row.URL,
			Trust:     row.Trust,
			Enabled:   row.Enabled,
			SortOrder: row.SortOrder,
		})
	}
	return sources
}

func (s *Service) ensureConfiguredSkillSources(ctx context.Context, sources []externalSkillSource) error {
	if s.skillStore == nil {
		return nil
	}
	ownerUserID := authctx.OwnerUserID(ctx)
	for _, source := range sources {
		if err := s.skillStore.EnsureSource(ctx, skillstore.SourceEntity{
			OwnerUserID: ownerUserID,
			SourceID:    source.Key,
			Name:        source.Name,
			Kind:        source.Kind,
			URL:         source.URL,
			Trust:       firstNonEmpty(source.Trust, externalSourceTrustCommunity),
			Enabled:     source.Enabled,
			SortOrder:   source.SortOrder,
		}); err != nil {
			return err
		}
	}
	return nil
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
	}
}

func externalSkillSourceInfoFromEntity(entity skillstore.SourceEntity) ExternalSkillSourceInfo {
	return ExternalSkillSourceInfo{
		SourceID:      entity.SourceID,
		Name:          entity.Name,
		Kind:          entity.Kind,
		URL:           entity.URL,
		Trust:         entity.Trust,
		Enabled:       entity.Enabled,
		SortOrder:     entity.SortOrder,
		LastCheckedAt: entity.LastCheckedAt,
		LastError:     entity.LastError,
	}
}

func splitExternalSourceList(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == ';'
	})
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseConfiguredExternalSource(raw string) (externalSkillSource, bool) {
	label := ""
	sourceURL := strings.TrimSpace(raw)
	if before, after, ok := strings.Cut(sourceURL, "|"); ok {
		label = strings.TrimSpace(before)
		sourceURL = strings.TrimSpace(after)
	}
	parsed, err := url.Parse(sourceURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return externalSkillSource{}, false
	}
	kind := classifyExternalSourceKind(parsed)
	name := firstNonEmpty(label, externalSourceDefaultName(kind, parsed))
	sourceURL = strings.TrimRight(parsed.String(), "/")
	return externalSkillSource{
		Key:     buildSkillSourceID(kind, sourceURL),
		Name:    name,
		Kind:    kind,
		URL:     sourceURL,
		Trust:   externalSourceTrustCommunity,
		Enabled: true,
	}, true
}

func classifyExternalSourceKind(parsed *url.URL) string {
	path := strings.ToLower(parsed.Path)
	host := strings.ToLower(parsed.Host)
	switch {
	case strings.Contains(host, "claude-plugins.dev"):
		return externalSourceKindClaudePlugins
	case strings.Contains(host, "skills.sh"):
		return externalSourceKindSkillsSh
	case strings.Contains(host, "clawhub.ai"):
		return externalSourceKindClawhub
	case strings.Contains(host, "hermes-agent.nousresearch.com"):
		return externalSourceKindHermesIndex
	case strings.Contains(host, "browse.sh"):
		return externalSourceKindBrowseSh
	}
	if strings.HasSuffix(path, ".json") || strings.Contains(path, ".well-known") {
		return externalSourceKindWellKnown
	}
	if strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".zip") {
		return externalSourceKindURL
	}
	if strings.Contains(strings.ToLower(parsed.Host), "github.com") || strings.HasSuffix(path, ".git") {
		return externalSourceKindGit
	}
	return externalSourceKindWellKnown
}

func externalSourceDefaultName(kind string, parsed *url.URL) string {
	if strings.Contains(strings.ToLower(parsed.Host), "github.com") {
		return "GitHub"
	}
	switch kind {
	case externalSourceKindClaudePlugins:
		return "claude-plugins.dev"
	case externalSourceKindSkillsSh:
		return "skills.sh"
	case externalSourceKindClawhub:
		return "clawhub.ai"
	case externalSourceKindHermesIndex:
		return "Hermes Skills Index"
	case externalSourceKindBrowseSh:
		return "browse.sh"
	case externalSourceKindWellKnown:
		return "Skill Index"
	case externalSourceKindURL:
		return "URL"
	case externalSourceKindGit:
		return "Git"
	default:
		return parsed.Host
	}
}
