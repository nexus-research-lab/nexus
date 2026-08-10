// INPUT: Agent 绑定的 runtime Skill 白名单/拒绝集与当前可信配置角色。
// OUTPUT: 仅启用当前角色 Skill、显式拒绝其余角色 Skill 的运行时投影。
// POS: DM/Room runtime 之间共享的配置 Skill 渐进披露边界。
package workspace

import "strings"

const (
	ConfigurationSkillOwnerMain  = "nexus-owner-configuration"
	ConfigurationSkillAgentSelf  = "nexus-agent-self-configuration"
	ConfigurationSkillRoomHost   = "nexus-room-host-configuration"
	ConfigurationSkillRoomMember = "nexus-room-member-configuration"
)

var configurationRoleSkillNames = []string{
	ConfigurationSkillOwnerMain,
	ConfigurationSkillAgentSelf,
	ConfigurationSkillRoomHost,
	ConfigurationSkillRoomMember,
}

// WithRuntimeConfigurationRoleSkill makes role guidance contextual instead of
// persisting it in Agent options or every system prompt. Unknown roles fail
// closed by disabling all configuration role Skills.
func WithRuntimeConfigurationRoleSkill(
	enabled []string,
	disabled []string,
	active string,
) ([]string, []string) {
	active = canonicalConfigurationRoleSkill(active)
	enabledResult := filterConfigurationRoleSkills(enabled)
	disabledResult := filterConfigurationRoleSkills(disabled)
	if active != "" {
		enabledResult = appendSkillNameOnce(enabledResult, active)
	}
	for _, name := range configurationRoleSkillNames {
		if !strings.EqualFold(name, active) {
			disabledResult = appendSkillNameOnce(disabledResult, name)
		}
	}
	return enabledResult, disabledResult
}

func canonicalConfigurationRoleSkill(name string) string {
	for _, candidate := range configurationRoleSkillNames {
		if strings.EqualFold(strings.TrimSpace(name), candidate) {
			return candidate
		}
	}
	return ""
}

func filterConfigurationRoleSkills(names []string) []string {
	result := make([]string, 0, len(names))
	for _, name := range names {
		if canonicalConfigurationRoleSkill(name) != "" {
			continue
		}
		result = appendSkillNameOnce(result, name)
	}
	return result
}
