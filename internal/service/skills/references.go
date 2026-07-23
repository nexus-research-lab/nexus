// INPUT: Skill catalog 记录与 Agent 保存的 Skill 引用。
// OUTPUT: 稳定的 Agent 引用、显示名称和迁移结果。
// POS: 外部全局 Skill 与 Agent runtime 之间的引用适配边界。
package skills

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func skillReference(record catalogRecord) string {
	if isPlatformSkill(record) || record.Detail.SourceType == sourceTypeSystem {
		return record.Detail.Name
	}
	return protocol.BuildExternalSkillReference(record.Detail.Name)
}

func normalizeAgentSkillReferences(
	ctx context.Context,
	agentValue *protocol.Agent,
	records map[string]catalogRecord,
	agents interface {
		UpdateAgent(context.Context, string, protocol.UpdateRequest) (*protocol.Agent, error)
	},
) (*protocol.Agent, error) {
	if agentValue == nil || agents == nil {
		return agentValue, nil
	}
	selected := make([]string, 0, len(agentValue.Options.SkillIDs))
	seen := map[string]struct{}{}
	changed := false
	for _, current := range agentValue.Options.SkillIDs {
		value := strings.TrimSpace(current)
		if value == "" {
			changed = true
			continue
		}
		if record, ok := findCatalogRecord(records, value); ok && record.Detail.SourceType == sourceTypeExternal {
			value = skillReference(record)
			if value != strings.TrimSpace(current) {
				changed = true
			}
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			changed = true
			continue
		}
		seen[key] = struct{}{}
		selected = append(selected, value)
	}
	if !changed {
		return agentValue, nil
	}
	options := agentValue.Options
	options.SkillIDs = selected
	return agents.UpdateAgent(ctx, agentValue.AgentID, protocol.UpdateRequest{Options: &options})
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

func ensureExternalSkillReference(skillIDs []string, skillName string) ([]string, bool) {
	reference := protocol.BuildExternalSkillReference(skillName)
	selected := make([]string, 0, len(skillIDs)+1)
	seen := map[string]struct{}{}
	found := false
	changed := false
	for _, current := range skillIDs {
		value := strings.TrimSpace(current)
		if value == "" {
			changed = true
			continue
		}
		if skillReferenceMatches(value, skillName) {
			if found {
				changed = true
				continue
			}
			found = true
			if value != reference {
				changed = true
			}
			value = reference
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			changed = true
			continue
		}
		seen[key] = struct{}{}
		selected = append(selected, value)
	}
	if !found {
		selected = append(selected, reference)
		changed = true
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
