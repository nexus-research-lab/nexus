package skills

import (
	"net/url"
	"sort"
	"strings"
)

func (s *Service) defaultExternalSkillSources() []externalSkillSource {
	skillsShURL := strings.TrimRight(firstNonEmpty(s.config.SkillsAPIURL, defaultSkillsShURL), "/")
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
