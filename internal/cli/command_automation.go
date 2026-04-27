package cli

import (
	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/spf13/cobra"
)

func newAutomationCommand(services *cliServiceProvider) *cobra.Command {
	command := &cobra.Command{
		Use:   "automation",
		Short: "automation 领域命令",
	}
	command.AddCommand(newScheduledTaskCommand(services))
	command.AddCommand(newHeartbeatCommand(services))
	return command
}

func newScheduledTaskCommand(services *cliServiceProvider) *cobra.Command {
	command := &cobra.Command{
		Use:   "task",
		Short: "scheduled task 命令",
	}

	command.AddCommand(func() *cobra.Command {
		var agentID string
		listCommand := &cobra.Command{
			Use:   "list",
			Short: "列出定时任务",
			RunE: func(cmd *cobra.Command, args []string) error {
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Automation
				items, err := service.ListTasks(commandContext(cmd), agentID)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "automation.task",
					"action": "list",
					"items":  items,
				})
			},
		}
		listCommand.Flags().StringVar(&agentID, "agent-id", "", "agent id")
		return listCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var (
			name            string
			agentID         string
			instruction     string
			scheduleKind    string
			runAt           string
			intervalSeconds int
			cronExpression  string
			timezone        string
			targetKind      string
			boundSessionKey string
			namedSessionKey string
			wakeMode        string
			deliveryMode    string
			enabled         bool
		)
		createCommand := &cobra.Command{
			Use:   "create",
			Short: "创建定时任务",
			RunE: func(cmd *cobra.Command, args []string) error {
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Automation
				payload := protocol.CreateJobInput{
					Name:        name,
					AgentID:     agentID,
					Instruction: instruction,
					Schedule: protocol.Schedule{
						Kind:     scheduleKind,
						Timezone: timezone,
					},
					SessionTarget: protocol.SessionTarget{
						Kind:            targetKind,
						BoundSessionKey: boundSessionKey,
						NamedSessionKey: namedSessionKey,
						WakeMode:        wakeMode,
					},
					Delivery: protocol.DeliveryTarget{
						Mode: deliveryMode,
					},
					Enabled: enabled,
				}
				if runAt != "" {
					payload.Schedule.RunAt = stringRef(runAt)
				}
				if intervalSeconds > 0 {
					payload.Schedule.IntervalSeconds = intRef(intervalSeconds)
				}
				if cronExpression != "" {
					payload.Schedule.CronExpression = stringRef(cronExpression)
				}
				item, err := service.CreateTask(commandContext(cmd), payload)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "automation.task",
					"action": "create",
					"item":   item,
				})
			},
		}
		createCommand.Flags().StringVar(&name, "name", "", "task name")
		createCommand.Flags().StringVar(&agentID, "agent-id", "", "agent id")
		createCommand.Flags().StringVar(&instruction, "instruction", "", "task instruction")
		createCommand.Flags().StringVar(&scheduleKind, "schedule-kind", protocol.ScheduleKindEvery, "every|cron|at")
		createCommand.Flags().StringVar(&runAt, "run-at", "", "run at")
		createCommand.Flags().IntVar(&intervalSeconds, "interval-seconds", 0, "interval seconds")
		createCommand.Flags().StringVar(&cronExpression, "cron-expression", "", "cron expression")
		createCommand.Flags().StringVar(&timezone, "timezone", "Asia/Shanghai", "timezone")
		createCommand.Flags().StringVar(&targetKind, "target-kind", protocol.SessionTargetIsolated, "isolated|main|bound|named")
		createCommand.Flags().StringVar(&boundSessionKey, "bound-session-key", "", "bound session key")
		createCommand.Flags().StringVar(&namedSessionKey, "named-session-key", "", "named session key")
		createCommand.Flags().StringVar(&wakeMode, "wake-mode", protocol.WakeModeNextHeartbeat, "now|next-heartbeat")
		createCommand.Flags().StringVar(&deliveryMode, "delivery-mode", protocol.DeliveryModeNone, "none|last|explicit")
		createCommand.Flags().BoolVar(&enabled, "enabled", true, "enabled")
		_ = createCommand.MarkFlagRequired("name")
		_ = createCommand.MarkFlagRequired("agent-id")
		_ = createCommand.MarkFlagRequired("instruction")
		return createCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var enabled bool
		var name string
		var instruction string
		var scheduleKind string
		var runAt string
		var intervalSeconds int
		var cronExpression string
		var timezone string
		var targetKind string
		var boundSessionKey string
		var namedSessionKey string
		var wakeMode string
		var deliveryMode string

		updateCommand := &cobra.Command{
			Use:   "update [job_id]",
			Short: "更新定时任务",
			Args:  exactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Automation
				payload := protocol.UpdateJobInput{}
				if name != "" {
					payload.Name = stringRef(name)
				}
				if instruction != "" {
					payload.Instruction = stringRef(instruction)
				}
				if scheduleKind != "" {
					schedule := protocol.Schedule{
						Kind:     scheduleKind,
						Timezone: timezone,
					}
					if runAt != "" {
						schedule.RunAt = stringRef(runAt)
					}
					if intervalSeconds > 0 {
						schedule.IntervalSeconds = intRef(intervalSeconds)
					}
					if cronExpression != "" {
						schedule.CronExpression = stringRef(cronExpression)
					}
					payload.Schedule = &schedule
				}
				if targetKind != "" {
					target := protocol.SessionTarget{
						Kind:            targetKind,
						BoundSessionKey: boundSessionKey,
						NamedSessionKey: namedSessionKey,
						WakeMode:        wakeMode,
					}
					payload.SessionTarget = &target
				}
				if deliveryMode != "" {
					payload.Delivery = &protocol.DeliveryTarget{Mode: deliveryMode}
				}
				if cmd.Flags().Changed("enabled") {
					payload.Enabled = &enabled
				}
				item, err := service.UpdateTask(commandContext(cmd), args[0], payload)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "automation.task",
					"action": "update",
					"item":   item,
				})
			},
		}
		updateCommand.Flags().StringVar(&name, "name", "", "task name")
		updateCommand.Flags().StringVar(&instruction, "instruction", "", "task instruction")
		updateCommand.Flags().StringVar(&scheduleKind, "schedule-kind", "", "every|cron|at")
		updateCommand.Flags().StringVar(&runAt, "run-at", "", "run at")
		updateCommand.Flags().IntVar(&intervalSeconds, "interval-seconds", 0, "interval seconds")
		updateCommand.Flags().StringVar(&cronExpression, "cron-expression", "", "cron expression")
		updateCommand.Flags().StringVar(&timezone, "timezone", "Asia/Shanghai", "timezone")
		updateCommand.Flags().StringVar(&targetKind, "target-kind", "", "isolated|main|bound|named")
		updateCommand.Flags().StringVar(&boundSessionKey, "bound-session-key", "", "bound session key")
		updateCommand.Flags().StringVar(&namedSessionKey, "named-session-key", "", "named session key")
		updateCommand.Flags().StringVar(&wakeMode, "wake-mode", protocol.WakeModeNextHeartbeat, "now|next-heartbeat")
		updateCommand.Flags().StringVar(&deliveryMode, "delivery-mode", "", "none|last|explicit")
		updateCommand.Flags().BoolVar(&enabled, "enabled", false, "enabled")
		return updateCommand
	}())

	command.AddCommand(&cobra.Command{
		Use:   "delete [job_id]",
		Short: "删除定时任务",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appServices, err := services.AppServices()
			if err != nil {
				return err
			}
			service := appServices.Automation
			if err := service.DeleteTask(commandContext(cmd), args[0]); err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "automation.task",
				"action": "delete",
				"item": map[string]any{
					"job_id": args[0],
				},
			})
		},
	})

	command.AddCommand(&cobra.Command{
		Use:   "run [job_id]",
		Short: "立即运行定时任务",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appServices, err := services.AppServices()
			if err != nil {
				return err
			}
			service := appServices.Automation
			item, err := service.RunTaskNow(commandContext(cmd), args[0])
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "automation.task",
				"action": "run",
				"item":   item,
			})
		},
	})

	command.AddCommand(&cobra.Command{
		Use:   "runs [job_id]",
		Short: "读取任务运行历史",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appServices, err := services.AppServices()
			if err != nil {
				return err
			}
			service := appServices.Automation
			items, err := service.ListTaskRuns(commandContext(cmd), args[0])
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "automation.task",
				"action": "runs",
				"items":  items,
			})
		},
	})

	command.AddCommand(func() *cobra.Command {
		var enabled bool
		statusCommand := &cobra.Command{
			Use:   "status [job_id]",
			Short: "切换任务启停状态",
			Args:  exactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Automation
				item, err := service.UpdateTaskStatus(commandContext(cmd), args[0], enabled)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "automation.task",
					"action": "status",
					"item":   item,
				})
			},
		}
		statusCommand.Flags().BoolVar(&enabled, "enabled", true, "enabled")
		return statusCommand
	}())

	return command
}

func newHeartbeatCommand(services *cliServiceProvider) *cobra.Command {
	command := &cobra.Command{
		Use:   "heartbeat",
		Short: "heartbeat 自动化命令",
	}

	command.AddCommand(&cobra.Command{
		Use:   "get [agent_id]",
		Short: "读取 heartbeat 状态",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appServices, err := services.AppServices()
			if err != nil {
				return err
			}
			service := appServices.Automation
			item, err := service.GetHeartbeatStatus(commandContext(cmd), args[0])
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "automation.heartbeat",
				"action": "get",
				"item":   item,
			})
		},
	})

	command.AddCommand(func() *cobra.Command {
		var enabled bool
		var everySeconds int
		var targetMode string
		var ackMaxChars int
		setCommand := &cobra.Command{
			Use:   "set [agent_id]",
			Short: "更新 heartbeat 配置",
			Args:  exactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Automation
				item, err := service.UpdateHeartbeat(commandContext(cmd), args[0], protocol.HeartbeatUpdateInput{
					Enabled:      enabled,
					EverySeconds: everySeconds,
					TargetMode:   targetMode,
					AckMaxChars:  ackMaxChars,
				})
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "automation.heartbeat",
					"action": "set",
					"item":   item,
				})
			},
		}
		setCommand.Flags().BoolVar(&enabled, "enabled", false, "enabled")
		setCommand.Flags().IntVar(&everySeconds, "every-seconds", 1800, "every seconds")
		setCommand.Flags().StringVar(&targetMode, "target-mode", protocol.HeartbeatTargetNone, "none|last")
		setCommand.Flags().IntVar(&ackMaxChars, "ack-max-chars", 300, "ack max chars")
		return setCommand
	}())

	command.AddCommand(func() *cobra.Command {
		var mode string
		var text string
		wakeCommand := &cobra.Command{
			Use:   "wake [agent_id]",
			Short: "手动唤醒 heartbeat",
			Args:  exactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				appServices, err := services.AppServices()
				if err != nil {
					return err
				}
				service := appServices.Automation
				request := protocol.HeartbeatWakeRequest{Mode: mode}
				if text != "" {
					request.Text = stringRef(text)
				}
				item, err := service.WakeHeartbeat(commandContext(cmd), args[0], request)
				if err != nil {
					return err
				}
				return emitJSON(map[string]any{
					"domain": "automation.heartbeat",
					"action": "wake",
					"item":   item,
				})
			},
		}
		wakeCommand.Flags().StringVar(&mode, "mode", protocol.WakeModeNow, "now|next-heartbeat")
		wakeCommand.Flags().StringVar(&text, "text", "", "wake text")
		return wakeCommand
	}())

	return command
}

func stringRef(value string) *string {
	result := value
	return &result
}

func intRef(value int) *int {
	result := value
	return &result
}
