// INPUT: owner 偏好、desktop host capability 与 runtime Skill allow/deny 集合。
// OUTPUT: 仅在 owner 明确开启时可见的 computer-use Skill 绑定。
// POS: DM runtime 启动前的 Computer Use 用户授权投影。
package dm

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
)

func (s *Service) bindComputerUseSkill(
	ctx context.Context,
	ownerUserID string,
	selected []string,
	disabled []string,
) ([]string, []string) {
	enabled := false
	if s.config.ComputerUseAvailable && s.prefs != nil {
		preferences, err := s.prefs.Get(ctx, ownerUserID)
		enabled = err == nil && preferences.ComputerUseEnabled
	}
	return runtimecommand.BindComputerUseSkill(selected, disabled, enabled)
}
