package cli

import (
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"

	"github.com/spf13/cobra"
)

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
				item, err := service.UpdateHeartbeat(commandContext(cmd), args[0], automationdomain.HeartbeatUpdateInput{
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
		setCommand.Flags().StringVar(&targetMode, "target-mode", automationdomain.HeartbeatTargetNone, "none|last")
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
				request := automationdomain.HeartbeatWakeInput{Mode: mode}
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
		wakeCommand.Flags().StringVar(&mode, "mode", automationdomain.WakeModeNow, "now|next-heartbeat")
		wakeCommand.Flags().StringVar(&text, "text", "", "wake text")
		return wakeCommand
	}())

	return command
}
