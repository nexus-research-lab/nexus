// INPUT: 混合了重复/越权角色 Skill 的运行时启用与拒绝集合。
// OUTPUT: 只启用可信当前角色，其他三种角色始终被拒绝。
// POS: 对话配置渐进披露与角色隔离的纯函数回归测试。
package workspace

import (
	"slices"
	"testing"
)

func TestWithRuntimeConfigurationRoleSkillSelectsExactlyOneRole(t *testing.T) {
	enabled, disabled := WithRuntimeConfigurationRoleSkill(
		[]string{"imagegen", ConfigurationSkillOwnerMain, ConfigurationSkillRoomHost},
		[]string{"custom-off", ConfigurationSkillRoomMember},
		ConfigurationSkillRoomHost,
	)
	if !slices.Contains(enabled, "imagegen") ||
		!slices.Contains(enabled, ConfigurationSkillRoomHost) ||
		slices.Contains(enabled, ConfigurationSkillOwnerMain) ||
		slices.Contains(enabled, ConfigurationSkillRoomMember) {
		t.Fatalf("enabled role Skills = %#v", enabled)
	}
	if slices.Contains(disabled, ConfigurationSkillRoomHost) ||
		!slices.Contains(disabled, ConfigurationSkillOwnerMain) ||
		!slices.Contains(disabled, ConfigurationSkillAgentSelf) ||
		!slices.Contains(disabled, ConfigurationSkillRoomMember) ||
		!slices.Contains(disabled, "custom-off") {
		t.Fatalf("disabled role Skills = %#v", disabled)
	}
}

func TestWithRuntimeConfigurationRoleSkillFailsClosedForUnknownRole(t *testing.T) {
	enabled, disabled := WithRuntimeConfigurationRoleSkill(
		[]string{ConfigurationSkillOwnerMain},
		nil,
		"forged-role",
	)
	if slices.Contains(enabled, ConfigurationSkillOwnerMain) {
		t.Fatalf("unknown role retained configuration Skill: %#v", enabled)
	}
	for _, name := range configurationRoleSkillNames {
		if !slices.Contains(disabled, name) {
			t.Fatalf("unknown role did not disable %s: %#v", name, disabled)
		}
	}
}
