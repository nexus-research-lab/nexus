package skills

import (
	"slices"
	"strings"
)

func externalItemMatchesQuery(item ExternalSkillSearchItem, needle string) bool {
	query := strings.ToLower(strings.TrimSpace(needle))
	if query == "" {
		return true
	}
	values := []string{item.Name, item.Title, item.Description, item.Source, item.PackageSpec, item.SourceName}
	values = append(values, item.Tags...)
	return slices.ContainsFunc(values, func(value string) bool {
		return strings.Contains(strings.ToLower(value), query)
	})
}

func dedupeExternalItems(items []ExternalSkillSearchItem) []ExternalSkillSearchItem {
	seen := map[string]struct{}{}
	result := make([]ExternalSkillSearchItem, 0, len(items))
	for _, item := range items {
		key := firstNonEmpty(item.SourceKey, item.PackageSpec, item.GitURL, item.RawURL, item.DetailURL) + "::" + firstNonEmpty(item.SkillSlug, item.Name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}
