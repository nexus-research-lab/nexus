package cli

import (
	"strings"

	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/protocol"

	"github.com/spf13/cobra"
)

func addScheduledTaskOperationsCommands(command *cobra.Command, services *cliServiceProvider) {
	command.AddCommand(newScheduledTaskInspectCommand(services))
	command.AddCommand(newScheduledTaskEventsCommand(services))
	command.AddCommand(newScheduledTaskReportCommand(services))
	command.AddCommand(newScheduledTaskRecoverCommand(services))
	command.AddCommand(newScheduledTaskRetryDeliveryCommand(services))
	command.AddCommand(newScheduledTaskEnableCommand(services))
	command.AddCommand(newScheduledTaskDisableCommand(services))
}

func scheduledTaskService(cmd *cobra.Command, services *cliServiceProvider) (*automationsvc.Service, error) {
	appServices, err := services.AppServices()
	if err != nil {
		return nil, err
	}
	return appServices.Automation, nil
}

func newScheduledTaskInspectCommand(services *cliServiceProvider) *cobra.Command {
	var runLimit int
	var eventLimit int
	command := &cobra.Command{
		Use:   "inspect [job_id]",
		Short: "读取任务状态、健康摘要、最近运行和审计",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := scheduledTaskService(cmd, services)
			if err != nil {
				return err
			}
			item, err := service.GetTaskStatus(commandContext(cmd), args[0], runLimit, eventLimit)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "automation.task",
				"action": "inspect",
				"item":   item,
			})
		},
	}
	command.Flags().IntVar(&runLimit, "run-limit", 10, "recent run limit")
	command.Flags().IntVar(&eventLimit, "event-limit", 10, "recent event limit")
	return command
}

func newScheduledTaskEventsCommand(services *cliServiceProvider) *cobra.Command {
	var limit int
	command := &cobra.Command{
		Use:   "events [job_id]",
		Short: "读取任务管理审计事件",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := scheduledTaskService(cmd, services)
			if err != nil {
				return err
			}
			items, err := service.ListTaskEvents(commandContext(cmd), args[0], limit)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "automation.task",
				"action": "events",
				"items":  items,
			})
		},
	}
	command.Flags().IntVar(&limit, "limit", 20, "event limit")
	return command
}

func newScheduledTaskReportCommand(services *cliServiceProvider) *cobra.Command {
	var input automationdomain.CronDailyReportInput
	command := &cobra.Command{
		Use:   "report",
		Short: "按日期汇总定时任务执行和投递状态",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := scheduledTaskService(cmd, services)
			if err != nil {
				return err
			}
			item, err := service.GetDailyReport(commandContext(cmd), input)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "automation.task",
				"action": "report",
				"item":   item,
			})
		},
	}
	command.Flags().StringVar(&input.Date, "date", "", "report date in YYYY-MM-DD")
	command.Flags().StringVar(&input.Timezone, "timezone", "Asia/Shanghai", "report timezone")
	command.Flags().StringVar(&input.AgentID, "agent-id", "", "agent id")
	command.Flags().StringVar(&input.JobID, "job-id", "", "job id")
	return command
}

func newScheduledTaskRecoverCommand(services *cliServiceProvider) *cobra.Command {
	var runID string
	command := &cobra.Command{
		Use:   "recover [job_id]",
		Short: "释放卡住的任务运行占用并取消未完成 run",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := scheduledTaskService(cmd, services)
			if err != nil {
				return err
			}
			item, err := service.RecoverTaskRunningRun(commandContext(cmd), args[0], runID)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "automation.task",
				"action": "recover",
				"item":   item,
			})
		},
	}
	command.Flags().StringVar(&runID, "run-id", "", "expected running run id")
	return command
}

func newScheduledTaskRetryDeliveryCommand(services *cliServiceProvider) *cobra.Command {
	return &cobra.Command{
		Use:   "retry-delivery [job_id] [run_id]",
		Short: "只重试某次 run 的结果投递，不重新执行任务",
		Args:  exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := scheduledTaskService(cmd, services)
			if err != nil {
				return err
			}
			item, err := service.RetryRunDelivery(commandContext(cmd), args[0], args[1])
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "automation.task",
				"action": "retry_delivery",
				"item":   item,
			})
		},
	}
}

func newScheduledTaskEnableCommand(services *cliServiceProvider) *cobra.Command {
	return &cobra.Command{
		Use:   "enable [job_id]",
		Short: "启用定时任务",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := scheduledTaskService(cmd, services)
			if err != nil {
				return err
			}
			item, err := service.UpdateTaskStatus(commandContext(cmd), args[0], true)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "automation.task",
				"action": "enable",
				"item":   item,
			})
		},
	}
}

func newScheduledTaskDisableCommand(services *cliServiceProvider) *cobra.Command {
	var cancelActiveRun bool
	var runID string
	command := &cobra.Command{
		Use:   "disable [job_id]",
		Short: "停用定时任务，可选择取消当前 active run",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := scheduledTaskService(cmd, services)
			if err != nil {
				return err
			}
			item, err := service.UpdateTaskStatus(commandContext(cmd), args[0], false)
			if err != nil {
				return err
			}
			if cancelActiveRun {
				expectedRunID := strings.TrimSpace(runID)
				if expectedRunID == "" {
					expectedRunID = strings.TrimSpace(item.RunningRunID)
				}
				if expectedRunID != "" {
					item, err = service.RecoverTaskRunningRun(commandContext(cmd), args[0], expectedRunID)
					if err != nil {
						return err
					}
				}
			}
			return emitJSON(map[string]any{
				"domain": "automation.task",
				"action": "disable",
				"item":   item,
			})
		},
	}
	command.Flags().BoolVar(&cancelActiveRun, "cancel-active-run", false, "cancel active running run after disabling")
	command.Flags().StringVar(&runID, "run-id", "", "expected running run id")
	return command
}
