package cli

import (
	memorysvc "github.com/nexus-research-lab/nexus/internal/workspace/memory"

	"github.com/spf13/cobra"
)

func newMemoryCommand() *cobra.Command {
	var workspacePath string
	command := &cobra.Command{
		Use:   "memory",
		Short: "memory 记忆系统命令",
	}
	command.PersistentFlags().StringVar(&workspacePath, "workspace", "", "workspace absolute path")
	_ = command.MarkPersistentFlagRequired("workspace")

	services := memoryCommandServices{workspacePath: &workspacePath}
	command.AddCommand(
		newMemorySearchCommand(services),
		newMemoryGetCommand(services),
		newMemoryListCommand(services),
		newMemoryRecallCommand(services),
		newMemoryAddCommand(services),
		newMemoryUpdateCommand(services),
		newMemoryDeleteCommand(services),
		newMemoryIgnoreCommand(services),
		newMemoryStatsCommand(services),
		newMemoryCleanupCommand(services),
		newMemorySessionSummaryCommand(services),
		newMemoryReviewCommand(services),
		newMemoryLogCommand(services),
		newMemoryPromoteCommand(services),
		newMemoryResolveCommand(services),
		newMemorySetStatusCommand(services),
	)

	return command
}

type memoryCommandServices struct {
	workspacePath *string
}

func (s memoryCommandServices) service() *memorysvc.Service {
	return memorysvc.NewService(*s.workspacePath)
}

func (s memoryCommandServices) engine() *memorysvc.Engine {
	return memorysvc.NewEngine(*s.workspacePath, memorysvc.DefaultOptions())
}
