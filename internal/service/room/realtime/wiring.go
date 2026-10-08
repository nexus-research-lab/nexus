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
		{"agents", s.agents != nil},
		{"runtime", s.runtime != nil},
		{"permission", s.permission != nil},
		{"providers", s.providers != nil},
		{"admission", s.admission != nil},
		{"preferences", s.prefs != nil},
		{"queue admission store", s.queueTrust != nil},
		{"usage recorder", s.usage != nil},
		{"quota checker", s.quota != nil},
		{"goal context provider", s.goals != nil},
		{"execution context provider", s.executionContext != nil},
		{"subagent admission provider", s.subagentAdmission != nil},
		{"MCP server builder", s.mcpServers != nil},
		{"configuration runtime environment builder", s.configurationRuntimeEnv != nil},
		{"Nexus MCP server builder", s.nexusMCP != nil},
		{"runtime slash expander", s.runtimeSlashExpander != nil},
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
