// INPUT: app 装配后的 DM Service。
// OUTPUT: 生产必需依赖缺失时的装配错误。
// POS: DM 依赖的唯一完整性检查；业务方法不再把“未装配”降级为“功能关闭”。
package dm

import (
	"fmt"
	"strings"
)

// RequireWiring 校验生产装配必须注入的依赖；缺失属于装配错误，应在启动时失败。
func (s *Service) RequireWiring() error {
	required := []struct {
		name  string
		wired bool
	}{
		{"agents", s.Agents != nil},
		{"runtime", s.Runtime != nil},
		{"permission", s.Permission != nil},
		{"providers", s.Providers != nil},
		{"admission", s.Admission != nil},
		{"preferences", s.Preferences != nil},
		{"room session store", s.roomStore != nil},
		{"room activity store", s.roomActivity != nil},
		{"queue admission store", s.QueueTrust != nil},
		{"usage recorder", s.Usage != nil},
		{"quota checker", s.Quota != nil},
		{"goal context provider", s.goals != nil},
		{"execution context provider", s.ExecutionContext != nil},
		{"subagent admission provider", s.SubagentAdmission != nil},
		{"MCP server builder", s.MCPServers != nil},
		{"configuration runtime environment builder", s.ConfigurationRuntimeEnv != nil},
		{"Nexus MCP server builder", s.NexusMCP != nil},
		{"runtime slash expander", s.RuntimeSlashExpander != nil},
		{"scoped session policy provider", s.scopedSessionPolicy != nil},
		{"title generator", s.titles != nil},
		{"external reply dispatcher", s.replies != nil},
		{"connector runtime state loader", s.connectorRuntimeStates != nil},
		{"IM delivery store", s.imReplies != nil && s.imReplyValidate != nil},
		{"IM automation policy", s.imAutomationPolicy != nil},
	}
	var missing []string
	for _, dependency := range required {
		if !dependency.wired {
			missing = append(missing, dependency.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("DM service missing dependencies: %s", strings.Join(missing, ", "))
	}
	return nil
}
