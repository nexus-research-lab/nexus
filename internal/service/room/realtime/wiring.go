// INPUT: app 装配后的 Room realtime Service。
// OUTPUT: 生产必需依赖缺失时的装配错误。
// POS: Room realtime 依赖的唯一完整性检查；业务方法不再把“未装配”降级为“功能关闭”。
package realtime

import (
	"fmt"
	"strings"
)

// RequireWiring 校验生产装配必须注入的依赖；缺失属于装配错误，应在启动时失败。
// Room 广播器由 WebSocket handler 在 HTTP 装配阶段注入，不在此检查。
func (s *Service) RequireWiring() error {
	required := []struct {
		name  string
		wired bool
	}{
		{"rooms", s.rooms != nil},
		{"agents", s.Agents != nil},
		{"runtime", s.Runtime != nil},
		{"permission", s.Permission != nil},
		{"providers", s.Providers != nil},
		{"admission", s.Admission != nil},
		{"preferences", s.prefs != nil},
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
		{"title generator", s.titles != nil},
	}
	var missing []string
	for _, dependency := range required {
		if !dependency.wired {
			missing = append(missing, dependency.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("Room realtime service missing dependencies: %s", strings.Join(missing, ", "))
	}
	return nil
}
