package cli

import (
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"

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
	command.AddCommand(
		newScheduledTaskListCommand(services),
		newScheduledTaskCreateCommand(services),
		newScheduledTaskUpdateCommand(services),
		newScheduledTaskDeleteCommand(services),
		newScheduledTaskRunCommand(services),
		newScheduledTaskRunsCommand(services),
		newScheduledTaskStatusCommand(services),
	)
	addScheduledTaskOperationsCommands(command, services)
	return command
}

func newScheduledTaskListCommand(services *cliServiceProvider) *cobra.Command {
	var agentID string
	command := &cobra.Command{
		Use:   "list",
		Short: "列出定时任务",
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := automationService(services)
			if err != nil {
				return err
			}
			items, err := service.ListTasks(commandContext(cmd), agentID)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{"domain": "automation.task", "action": "list", "items": items})
		},
	}
	command.Flags().StringVar(&agentID, "agent-id", "", "agent id")
	return command
}

type scheduledTaskScheduleFlags struct {
	kind            string
	runAt           string
	intervalSeconds int
	cronExpression  string
	timezone        string
}

func (f *scheduledTaskScheduleFlags) bind(command *cobra.Command, defaultKind string) {
	command.Flags().StringVar(&f.kind, "schedule-kind", defaultKind, "every|cron|at")
	command.Flags().StringVar(&f.runAt, "run-at", "", "run at")
	command.Flags().IntVar(&f.intervalSeconds, "interval-seconds", 0, "interval seconds")
	command.Flags().StringVar(&f.cronExpression, "cron-expression", "", "cron expression")
	command.Flags().StringVar(&f.timezone, "timezone", "Asia/Shanghai", "timezone")
}

func (f scheduledTaskScheduleFlags) value() automationdomain.Schedule {
	schedule := automationdomain.Schedule{Kind: f.kind, Timezone: f.timezone}
	if f.runAt != "" {
		schedule.RunAt = stringRef(f.runAt)
	}
	if f.intervalSeconds > 0 {
		schedule.IntervalSeconds = intRef(f.intervalSeconds)
	}
	if f.cronExpression != "" {
		schedule.CronExpression = stringRef(f.cronExpression)
	}
	return schedule
}

type scheduledTaskTargetFlags struct {
	kind            string
	boundSessionKey string
	namedSessionKey string
	wakeMode        string
}

func (f *scheduledTaskTargetFlags) bind(command *cobra.Command, defaultKind string) {
	command.Flags().StringVar(&f.kind, "target-kind", defaultKind, "isolated|main|bound|named")
	command.Flags().StringVar(&f.boundSessionKey, "bound-session-key", "", "bound session key")
	command.Flags().StringVar(&f.namedSessionKey, "named-session-key", "", "named session key")
	command.Flags().StringVar(&f.wakeMode, "wake-mode", automationdomain.WakeModeNextHeartbeat, "now|next-heartbeat")
}

func (f scheduledTaskTargetFlags) value() automationdomain.SessionTarget {
	return automationdomain.SessionTarget{
		Kind:            f.kind,
		BoundSessionKey: f.boundSessionKey,
		NamedSessionKey: f.namedSessionKey,
		WakeMode:        f.wakeMode,
	}
}

type scheduledTaskCreateFlags struct {
	name          string
	agentID       string
	instruction   string
	schedule      scheduledTaskScheduleFlags
	target        scheduledTaskTargetFlags
	delivery      scheduledTaskDeliveryFlags
	overlapPolicy string
	enabled       bool
}

func newScheduledTaskCreateCommand(services *cliServiceProvider) *cobra.Command {
	flags := &scheduledTaskCreateFlags{}
	command := &cobra.Command{
		Use:   "create",
		Short: "创建定时任务",
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := automationService(services)
			if err != nil {
				return err
			}
			item, err := service.CreateTask(commandContext(cmd), flags.payload())
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{"domain": "automation.task", "action": "create", "item": item})
		},
	}
	flags.bind(command)
	return command
}

func (f *scheduledTaskCreateFlags) bind(command *cobra.Command) {
	command.Flags().StringVar(&f.name, "name", "", "task name")
	command.Flags().StringVar(&f.agentID, "agent-id", "", "agent id")
	command.Flags().StringVar(&f.instruction, "instruction", "", "task instruction")
	f.schedule.bind(command, automationdomain.ScheduleKindEvery)
	f.target.bind(command, automationdomain.SessionTargetIsolated)
	bindScheduledTaskDeliveryFlags(command, &f.delivery, automationdomain.DeliveryModeNone)
	command.Flags().StringVar(&f.overlapPolicy, "overlap-policy", automationdomain.OverlapPolicySkip, "skip|allow")
	command.Flags().BoolVar(&f.enabled, "enabled", true, "enabled")
	for _, name := range []string{"name", "agent-id", "instruction"} {
		_ = command.MarkFlagRequired(name)
	}
}

func (f scheduledTaskCreateFlags) payload() automationdomain.CreateJobInput {
	return automationdomain.CreateJobInput{
		Name:          f.name,
		AgentID:       f.agentID,
		Instruction:   f.instruction,
		Schedule:      f.schedule.value(),
		SessionTarget: f.target.value(),
		Delivery:      f.delivery.target(),
		Source:        automationdomain.Source{Kind: automationdomain.SourceKindCLI},
		OverlapPolicy: f.overlapPolicy,
		Enabled:       f.enabled,
	}
}

type scheduledTaskUpdateFlags struct {
	name          string
	instruction   string
	schedule      scheduledTaskScheduleFlags
	target        scheduledTaskTargetFlags
	delivery      scheduledTaskDeliveryFlags
	overlapPolicy string
	enabled       bool
}

func newScheduledTaskUpdateCommand(services *cliServiceProvider) *cobra.Command {
	flags := &scheduledTaskUpdateFlags{}
	command := &cobra.Command{
		Use:   "update [job_id]",
		Short: "更新定时任务",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := automationService(services)
			if err != nil {
				return err
			}
			current, err := flags.currentTaskForDelivery(cmd, service, args[0])
			if err != nil {
				return err
			}
			item, err := service.UpdateTask(commandContext(cmd), args[0], flags.payload(cmd, current))
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{"domain": "automation.task", "action": "update", "item": item})
		},
	}
	flags.bind(command)
	return command
}

func (f *scheduledTaskUpdateFlags) bind(command *cobra.Command) {
	command.Flags().StringVar(&f.name, "name", "", "task name")
	command.Flags().StringVar(&f.instruction, "instruction", "", "task instruction")
	f.schedule.bind(command, "")
	f.target.bind(command, "")
	bindScheduledTaskDeliveryFlags(command, &f.delivery, "")
	command.Flags().StringVar(&f.overlapPolicy, "overlap-policy", "", "skip|allow")
	command.Flags().BoolVar(&f.enabled, "enabled", false, "enabled")
}

func (f scheduledTaskUpdateFlags) currentTaskForDelivery(command *cobra.Command, service *automationsvc.Service, jobID string) (*automationdomain.ScheduledTask, error) {
	if !f.delivery.changed(command) {
		return nil, nil
	}
	current, err := service.GetTask(commandContext(command), jobID)
	if err != nil || current != nil {
		return current, err
	}
	return nil, automationdomain.ErrJobNotFound
}

func (f scheduledTaskUpdateFlags) payload(command *cobra.Command, current *automationdomain.ScheduledTask) automationdomain.UpdateJobInput {
	payload := automationdomain.UpdateJobInput{}
	applyOptionalString(&payload.Name, f.name)
	applyOptionalString(&payload.Instruction, f.instruction)
	applyOptionalString(&payload.OverlapPolicy, f.overlapPolicy)
	if f.schedule.kind != "" {
		schedule := f.schedule.value()
		payload.Schedule = &schedule
	}
	if f.target.kind != "" {
		target := f.target.value()
		payload.SessionTarget = &target
	}
	if current != nil {
		delivery := current.Delivery
		f.delivery.apply(command, &delivery)
		payload.Delivery = &delivery
	}
	if command.Flags().Changed("enabled") {
		payload.Enabled = &f.enabled
	}
	return payload
}

func applyOptionalString(target **string, value string) {
	if value != "" {
		*target = stringRef(value)
	}
}

func newScheduledTaskDeleteCommand(services *cliServiceProvider) *cobra.Command {
	return &cobra.Command{
		Use:   "delete [job_id]",
		Short: "删除定时任务",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := automationService(services)
			if err != nil {
				return err
			}
			item, err := service.DeleteTask(commandContext(cmd), args[0])
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{"domain": "automation.task", "action": "delete", "item": item})
		},
	}
}

func newScheduledTaskRunCommand(services *cliServiceProvider) *cobra.Command {
	return &cobra.Command{
		Use:   "run [job_id]",
		Short: "立即运行定时任务",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := automationService(services)
			if err != nil {
				return err
			}
			item, err := service.RunTaskNow(commandContext(cmd), args[0])
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{"domain": "automation.task", "action": "run", "item": item})
		},
	}
}

func newScheduledTaskRunsCommand(services *cliServiceProvider) *cobra.Command {
	return &cobra.Command{
		Use:   "runs [job_id]",
		Short: "读取任务运行历史",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := automationService(services)
			if err != nil {
				return err
			}
			items, err := service.ListTaskRuns(commandContext(cmd), args[0])
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{"domain": "automation.task", "action": "runs", "items": items})
		},
	}
}

func newScheduledTaskStatusCommand(services *cliServiceProvider) *cobra.Command {
	var enabled bool
	command := &cobra.Command{
		Use:   "status [job_id]",
		Short: "切换任务启停状态",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := automationService(services)
			if err != nil {
				return err
			}
			item, err := service.UpdateTaskStatus(commandContext(cmd), args[0], enabled)
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{"domain": "automation.task", "action": "status", "item": item})
		},
	}
	command.Flags().BoolVar(&enabled, "enabled", true, "enabled")
	return command
}

func automationService(services *cliServiceProvider) (*automationsvc.Service, error) {
	appServices, err := services.AppServices()
	if err != nil {
		return nil, err
	}
	return appServices.Automation, nil
}

func stringRef(value string) *string {
	result := value
	return &result
}

func intRef(value int) *int {
	result := value
	return &result
}
