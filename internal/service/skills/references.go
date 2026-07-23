// INPUT: Skill catalog 记录与 Agent 保存的 Skill 引用。
// OUTPUT: 稳定的 Agent 引用与显示名称。
// POS: 外部 Skill 与 Agent runtime 之间的引用适配边界。
package skills

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func skillReference(record catalogRecord) string {
	if isPlatformSkill(record) || record.Detail.SourceType == sourceTypeSystem {
		return record.Detail.Name
	}
	return protocol.BuildExternalSkillReference(record.Detail.Name)
}

func findCatalogRecord(records map[string]catalogRecord, name string) (catalogRecord, bool) {
	trimmed := strings.TrimSpace(name)
	if externalName, ok := protocol.ParseExternalSkillReference(trimmed); ok {
		trimmed = externalName
	}
	if record, ok := records[trimmed]; ok {
		return record, true
	}
	for key, record := range records {
		if strings.EqualFold(strings.TrimSpace(key), trimmed) {
			return record, true
		}
	}
	return catalogRecord{}, false
}

func skillReferenceMatches(reference string, skillName string) bool {
	value := strings.TrimSpace(reference)
	name := strings.TrimSpace(skillName)
	if externalName, ok := protocol.ParseExternalSkillReference(value); ok {
		value = externalName
	}
	return value != "" && name != "" && strings.EqualFold(value, name)
}

func removeSkillReferences(skillIDs []string, skillName string) ([]string, bool) {
	selected := make([]string, 0, len(skillIDs))
	changed := false
	seen := map[string]struct{}{}
	for _, reference := range skillIDs {
		value := strings.TrimSpace(reference)
		if value == "" || skillReferenceMatches(value, skillName) {
			changed = true
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			changed = true
			continue
		}
		seen[key] = struct{}{}
		selected = append(selected, value)
	}
	return selected, changed
}

func installedSkillNames(agentValue *protocol.Agent, records map[string]catalogRecord) map[string]bool {
	result := map[string]bool{}
	if agentValue == nil {
		return result
	}
	for _, reference := range agentValue.Options.SkillIDs {
		value := strings.TrimSpace(reference)
		if value == "" {
			continue
		}
		result[value] = true
		name := value
		if externalName, ok := protocol.ParseExternalSkillReference(value); ok {
			name = externalName
		}
		if record, ok := findCatalogRecord(records, name); ok {
			result[record.Detail.Name] = true
		}
	}
	return result
}
