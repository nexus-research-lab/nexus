// INPUT: 宿主注入的 Nexus runtime command broker 环境。
// OUTPUT: nxs/Claude 共用、只通过 round capability 调用宿主的 nexus CLI。
// POS: Agent-facing Nexus CLI 根；不装配 AppServices，也不直接打开数据库。
package cli

import (
	"github.com/nexus-research-lab/nexus/internal/config"

	"github.com/spf13/cobra"
)

// NewRuntime 创建 Agent-facing nexus CLI。
func NewRuntime(cfg config.Config) (*App, error) {
	root := &cobra.Command{
		Use:           "nexus",
		Short:         "Nexus Agent runtime CLI",
		Long:          "通过宿主签发的 physical-round capability 调用 Nexus 领域服务；不接受 owner、Agent、Session 或权限覆盖。",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	output := configureRootOutput(root)
	root.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		return applyOutputOptions(cfg, nil, *output)
	}
	root.AddCommand(newRuntimeAutomationCommand())
	root.AddCommand(newRuntimeSemanticCommand("goal", "管理当前 round 的 durable Goal"))
	root.AddCommand(newRuntimeSemanticCommand("execution", "管理当前 round 的 Execution 与 WorkGraph"))
	return &App{command: root}, nil
}
