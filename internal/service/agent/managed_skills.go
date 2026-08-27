// INPUT: Goal/Execution command 领域及其内置 Skill 绑定。
// OUTPUT: 不可变的托管语义 Skill 目录与精确成员判断。
// POS: runtime 发现、提示投影和权限策略共用的唯一绑定真相源。
package agent

import "strings"

const (
	// GoalSkillName 是 Goal command 语义的规范 Skill 绑定。
	GoalSkillName = "goal-manager"
	// ExecutionSkillName 是 Execution/WorkGraph command 语义的规范 Skill 绑定。
	ExecutionSkillName = "execution-orchestrator"
)

var managedSemanticSkillNames = []string{
	GoalSkillName,
	ExecutionSkillName,
}

// ManagedSemanticSkillNames 返回由调用方持有的规范 Skill 绑定副本。
func ManagedSemanticSkillNames() []string {
	return append([]string(nil), managedSemanticSkillNames...)
}

// IsManagedSemanticSkillName 判断名称是否为托管 Goal/Execution Skill 绑定。
func IsManagedSemanticSkillName(name string) bool {
	name = strings.TrimSpace(name)
	for _, candidate := range managedSemanticSkillNames {
		if strings.EqualFold(name, candidate) {
			return true
		}
	}
	return false
}

// BindManagedSemanticSkills 强制绑定 Goal/Execution Skill，同时保留其他 Skill 的调用方顺序。
// 托管名称会被规范化、去重，并从禁用集合中移除。
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
