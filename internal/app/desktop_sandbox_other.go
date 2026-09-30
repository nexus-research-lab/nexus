//go:build !darwin && !windows

// INPUT: 非 macOS App 装配。
// OUTPUT: 保留现有平台启动行为。
// POS: macOS 监督接线的平台边界，不声明其他平台验收。
package app

import (
	"github.com/nexus-research-lab/nexus/internal/config"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func (s *AppServices) prepareDesktopSandbox(config.Config, runtimectx.SandboxProcessRecoveryOwnership) error {
	return nil
}
