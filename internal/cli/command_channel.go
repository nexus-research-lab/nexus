package cli

import (
	"github.com/nexus-research-lab/nexus/internal/channels"

	"github.com/spf13/cobra"
)

func newChannelCommand(service *channels.IngressService) *cobra.Command {
	command := &cobra.Command{
		Use:   "channel",
		Short: "channel 入口命令",
	}
	command.AddCommand(newChannelIngressCommand(service))
	return command
}

func newChannelIngressCommand(service *channels.IngressService) *cobra.Command {
	var (
		channel          string
		sessionKey       string
		agentID          string
		chatType         string
		ref              string
		threadID         string
		content          string
		roundID          string
		reqID            string
		permissionMode   string
		autoApproveAll   bool
		autoApproveTools []string
		deliveryTo       string
		deliveryThreadID string
		deliveryAccount  string
	)

	ingressCommand := &cobra.Command{
		Use:   "ingress",
		Short: "把外部通道消息注入统一聊天入口",
		RunE: func(cmd *cobra.Command, args []string) error {
			var delivery *channels.DeliveryTarget
			if deliveryTo != "" || deliveryThreadID != "" || deliveryAccount != "" {
				delivery = &channels.DeliveryTarget{
					Mode:      channels.DeliveryModeExplicit,
					Channel:   channel,
					To:        deliveryTo,
					ThreadID:  deliveryThreadID,
					AccountID: deliveryAccount,
				}
			}
			result, err := service.Accept(commandContext(cmd), channels.IngressRequest{
				Channel:          channel,
				SessionKey:       sessionKey,
				AgentID:          agentID,
				ChatType:         chatType,
				Ref:              ref,
				ThreadID:         threadID,
				Content:          content,
				RoundID:          roundID,
				ReqID:            reqID,
				PermissionMode:   permissionMode,
				AutoApproveAll:   autoApproveAll,
				AutoApproveTools: autoApproveTools,
				Delivery:         delivery,
			})
			if err != nil {
				return err
			}
			return emitJSON(map[string]any{
				"domain": "channel",
				"action": "ingress",
				"item":   result,
			})
		},
	}

	ingressCommand.Flags().StringVar(&channel, "channel", "", "channel type: internal|discord|telegram")
	ingressCommand.Flags().StringVar(&sessionKey, "session-key", "", "structured session_key")
	ingressCommand.Flags().StringVar(&agentID, "agent-id", "", "target agent id")
	ingressCommand.Flags().StringVar(&chatType, "chat-type", "dm", "dm|group")
	ingressCommand.Flags().StringVar(&ref, "ref", "", "channel scoped ref")
	ingressCommand.Flags().StringVar(&threadID, "thread-id", "", "optional thread/topic id")
	ingressCommand.Flags().StringVar(&content, "content", "", "message content")
	ingressCommand.Flags().StringVar(&roundID, "round-id", "", "optional round id")
	ingressCommand.Flags().StringVar(&reqID, "req-id", "", "optional request id")
	ingressCommand.Flags().StringVar(&permissionMode, "permission-mode", "", "optional sdk permission mode override")
	ingressCommand.Flags().BoolVar(&autoApproveAll, "auto-approve-all", false, "auto approve all non-interactive tools")
	ingressCommand.Flags().StringSliceVar(&autoApproveTools, "auto-approve-tool", nil, "append auto approved tool name")
	ingressCommand.Flags().StringVar(&deliveryTo, "delivery-to", "", "explicit reply route target")
	ingressCommand.Flags().StringVar(&deliveryThreadID, "delivery-thread-id", "", "explicit reply route thread id")
	ingressCommand.Flags().StringVar(&deliveryAccount, "delivery-account-id", "", "explicit reply route account id")
	_ = ingressCommand.MarkFlagRequired("content")
	return ingressCommand
}
