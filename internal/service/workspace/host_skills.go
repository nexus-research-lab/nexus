// INPUT: 桌面运行模式与宿主标准 ~/.agents/skills 源。
// OUTPUT: nxs/Claude 共用的用户全局 Skill 兼容根。
// POS: 宿主用户目录与 Nexus runtime 只读资源根之间的同步边界。
package workspace

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

var hostSkillLibraryState skillLibrarySyncState

// EnsureHostSkillLibrary 同步桌面用户的标准全局 Skill 源。
//
// 多用户服务不能读取宿主进程的 home；桌面模式下一名本机用户共享一份兼容根，
// Agent 只保存启用引用，不再把同一 Skill 复制到各自 workspace。
func EnsureHostSkillLibrary(cfg config.Config) error {
	if !strings.EqualFold(strings.TrimSpace(cfg.AppMode), "desktop") {
		return clearCompatibleSkillLibrary(appfs.HostSkillRoot(), &hostSkillLibraryState)
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return clearCompatibleSkillLibrary(appfs.HostSkillRoot(), &hostSkillLibraryState)
	}
	return ensureCompatibleSkillLibrary(
		filepath.Join(home, ".agents", "skills"),
		appfs.HostSkillRoot(),
		&hostSkillLibraryState,
	)
}
