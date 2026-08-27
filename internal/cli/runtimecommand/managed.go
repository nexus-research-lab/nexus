// INPUT: Goal/Execution command domains and their bundled Skill bindings.
// OUTPUT: immutable managed semantic Skill catalog and exact membership checks.
// POS: runtime discovery, prompt projection and permission policy share one binding truth.
package runtimecommand

import "strings"

const (
	// GoalSkillName is the canonical Skill binding for Goal command semantics.
	GoalSkillName = "goal-manager"
	// ExecutionSkillName is the canonical Skill binding for Execution/WorkGraph command semantics.
	ExecutionSkillName = "execution-orchestrator"
)

var managedSemanticSkillNames = []string{
	GoalSkillName,
	ExecutionSkillName,
}

// ManagedSemanticSkillNames returns a caller-owned copy of the canonical Skill bindings.
func ManagedSemanticSkillNames() []string {
	return append([]string(nil), managedSemanticSkillNames...)
}

// IsManagedSemanticSkillName reports whether name is an exact managed Goal/Execution binding.
func IsManagedSemanticSkillName(name string) bool {
	name = strings.TrimSpace(name)
	for _, candidate := range managedSemanticSkillNames {
		if strings.EqualFold(name, candidate) {
			return true
		}
	}
	return false
}

// BindManagedSemanticSkills makes Goal/Execution bindings non-optional while
// preserving caller order for every unrelated Skill. Managed names are
// canonicalized, deduplicated and removed from the disabled set.
func BindManagedSemanticSkills(skillIDs []string, disabledSkillIDs []string) ([]string, []string) {
	bound := make([]string, 0, len(skillIDs)+len(managedSemanticSkillNames))
	seenManaged := make(map[string]struct{}, len(managedSemanticSkillNames))
	for _, skillID := range skillIDs {
		skillID = strings.TrimSpace(skillID)
		if skillID == "" {
			continue
		}
		canonical, managed := canonicalManagedSemanticSkillName(skillID)
		if !managed {
			bound = append(bound, skillID)
			continue
		}
		if _, duplicate := seenManaged[canonical]; duplicate {
			continue
		}
		seenManaged[canonical] = struct{}{}
		bound = append(bound, canonical)
	}
	for _, skillName := range managedSemanticSkillNames {
		if _, exists := seenManaged[skillName]; !exists {
			bound = append(bound, skillName)
		}
	}

	disabled := make([]string, 0, len(disabledSkillIDs))
	for _, skillID := range disabledSkillIDs {
		skillID = strings.TrimSpace(skillID)
		if skillID == "" || IsManagedSemanticSkillName(skillID) {
			continue
		}
		disabled = append(disabled, skillID)
	}
	return bound, disabled
}

func canonicalManagedSemanticSkillName(name string) (string, bool) {
	for _, candidate := range managedSemanticSkillNames {
		if strings.EqualFold(strings.TrimSpace(name), candidate) {
			return candidate, true
		}
	}
	return "", false
}
