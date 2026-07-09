package cli

import (
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation"

	"github.com/spf13/cobra"
)

type scheduledTaskDeliveryFlags struct {
	mode      string
	channel   string
	to        string
	accountID string
	threadID  string
}

func bindScheduledTaskDeliveryFlags(command *cobra.Command, flags *scheduledTaskDeliveryFlags, defaultMode string) {
	command.Flags().StringVar(&flags.mode, "delivery-mode", defaultMode, "none|last|explicit")
	command.Flags().StringVar(&flags.channel, "delivery-channel", "", "websocket|internal|feishu|dingtalk|telegram|discord|wechat")
	command.Flags().StringVar(&flags.to, "delivery-to", "", "delivery target id or session key")
	command.Flags().StringVar(&flags.accountID, "delivery-account-id", "", "delivery account id")
	command.Flags().StringVar(&flags.threadID, "delivery-thread-id", "", "delivery thread id")
}

func (f scheduledTaskDeliveryFlags) target() automationdomain.DeliveryTarget {
	return automationdomain.DeliveryTarget{
		Mode:      f.mode,
		Channel:   f.channel,
		To:        f.to,
		AccountID: f.accountID,
		ThreadID:  f.threadID,
	}
}

func (f scheduledTaskDeliveryFlags) changed(command *cobra.Command) bool {
	return command.Flags().Changed("delivery-mode") ||
		command.Flags().Changed("delivery-channel") ||
		command.Flags().Changed("delivery-to") ||
		command.Flags().Changed("delivery-account-id") ||
		command.Flags().Changed("delivery-thread-id")
}

func (f scheduledTaskDeliveryFlags) apply(command *cobra.Command, target *automationdomain.DeliveryTarget) {
	if command.Flags().Changed("delivery-mode") {
		target.Mode = f.mode
	}
	if command.Flags().Changed("delivery-channel") {
		target.Channel = f.channel
	}
	if command.Flags().Changed("delivery-to") {
		target.To = f.to
	}
	if command.Flags().Changed("delivery-account-id") {
		target.AccountID = f.accountID
	}
	if command.Flags().Changed("delivery-thread-id") {
		target.ThreadID = f.threadID
	}
}
