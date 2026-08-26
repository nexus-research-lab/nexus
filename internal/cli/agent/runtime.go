// INPUT: 宿主注入的 Nexus runtime command broker 环境。
// OUTPUT: nxs/Claude 共用、只通过 round capability 调用宿主的 nexus CLI。
// POS: Agent-facing Nexus CLI 根；不装配 AppServices，也不直接打开数据库。
package agent

import (
	"io"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/spf13/cobra"
)

// App 持有 Agent CLI 根命令；与宿主 CLI 同形但不同信任模型。
type App struct {
	command *cobra.Command
}

func (a *App) SetArgs(arguments []string) {
	a.command.SetArgs(arguments)
}

func (a *App) Execute() error {
	return a.command.Execute()
}

// RunRuntime executes the Agent-facing runtime CLI and owns its error envelope.
func RunRuntime(arguments []string, stderr io.Writer) int {
	command := NewRuntime()
	command.SetArgs(arguments)
	err := command.Execute()
	if err == nil {
		return 0
	}
	writeCommandError(stderr, err, requestedJSON(arguments))
	return exitCode(err)
}

// RuntimeEntrypointArgs recognizes the private multicall entrypoint used by
// generated runtime shims. Broker capability validation remains authoritative.
func RuntimeEntrypointArgs(arguments []string) ([]string, bool) {
	if len(arguments) == 0 || strings.TrimSpace(arguments[0]) != protocol.NexusCommandHostEntrypointArgument {
		return nil, false
	}
	return arguments[1:], true
}

// NewRuntime 创建 Agent-facing nexus CLI。
func NewRuntime() *App {
	root := &cobra.Command{
		Use:           "nexus",
		Short:         "Nexus Agent runtime CLI",
		Long:          "通过宿主签发的 physical-round capability 调用 Nexus 领域服务；不接受 owner、Agent、Session 或权限覆盖。",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	output := configureRootOutput(root)
	root.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		return applyOutputOptions(*output)
	}
	root.AddCommand(newRuntimeAutomationCommand())
	root.AddCommand(newRuntimeSemanticCommand("goal", "管理当前 round 的 durable Goal"))
	root.AddCommand(newRuntimeSemanticCommand("execution", "管理当前 round 的 Execution 与 WorkGraph"))
	root.AddCommand(newRuntimeSemanticCommand("computer", "观察和操作当前 round 的原生桌面目标"))
	return &App{command: root}
}
