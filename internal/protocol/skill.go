package protocol

import "strings"

const externalSkillReferencePrefix = "external:"

// BuildExternalSkillReference 为用户级外部 Skill 构造不携带路径的稳定引用。
func BuildExternalSkillReference(skillName string) string {
	name := strings.TrimSpace(skillName)
	if !validExternalSkillName(name) {
		return ""
	}
	return externalSkillReferencePrefix + name
}

// ParseExternalSkillReference 将外部 Skill 引用还原为 canonical name。
func ParseExternalSkillReference(reference string) (string, bool) {
	value := strings.TrimSpace(reference)
	if len(value) < len(externalSkillReferencePrefix) ||
		!strings.EqualFold(value[:len(externalSkillReferencePrefix)], externalSkillReferencePrefix) {
		return "", false
	}
	name := strings.TrimSpace(value[len(externalSkillReferencePrefix):])
	return name, validExternalSkillName(name)
}

func validExternalSkillName(name string) bool {
	return name != "" && name != "." && name != ".." &&
		!strings.ContainsAny(name, "/\\\x00") &&
		!strings.HasPrefix(strings.ToLower(name), externalSkillReferencePrefix)
}
