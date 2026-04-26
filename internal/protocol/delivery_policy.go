package protocol

import "strings"

// ChatDeliveryPolicy 表示新输入遇到运行中会话时的投递策略。
type ChatDeliveryPolicy string

const (
	// ChatDeliveryPolicyQueue 表示把新输入排入当前流式会话，不中断正在执行的 round。
	ChatDeliveryPolicyQueue ChatDeliveryPolicy = "queue"
	// ChatDeliveryPolicyGuide 表示把新输入暂存在运行时，等待 PostToolUse hook 注入当前 round。
	ChatDeliveryPolicyGuide ChatDeliveryPolicy = "guide"
	// ChatDeliveryPolicyInterrupt 表示先中断当前运行，再启动新的 round。
	ChatDeliveryPolicyInterrupt ChatDeliveryPolicy = "interrupt"
	// ChatDeliveryPolicyAuto 预留给后续智能选择；当前等价于 queue。
	ChatDeliveryPolicyAuto ChatDeliveryPolicy = "auto"
)

// NormalizeChatDeliveryPolicy 归一化前端传入的投递策略。
func NormalizeChatDeliveryPolicy(value string) ChatDeliveryPolicy {
	switch ChatDeliveryPolicy(strings.ToLower(strings.TrimSpace(value))) {
	case ChatDeliveryPolicyInterrupt:
		return ChatDeliveryPolicyInterrupt
	case ChatDeliveryPolicyGuide:
		return ChatDeliveryPolicyGuide
	case ChatDeliveryPolicyAuto:
		return ChatDeliveryPolicyAuto
	case ChatDeliveryPolicyQueue, "append":
		return ChatDeliveryPolicyQueue
	default:
		return ChatDeliveryPolicyQueue
	}
}

// ShouldQueueRunningRound 判断运行中会话默认是否接收排队输入。
func ShouldQueueRunningRound(policy ChatDeliveryPolicy) bool {
	return policy == ChatDeliveryPolicyQueue || policy == ChatDeliveryPolicyAuto
}

// ShouldGuideRunningRound 判断运行中会话是否接收 hook 引导输入。
func ShouldGuideRunningRound(policy ChatDeliveryPolicy) bool {
	return policy == ChatDeliveryPolicyGuide
}
