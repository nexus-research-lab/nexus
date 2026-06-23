package semantic

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/internal/argx"
)

var currentChannelDeliveryIntentKeywords = []string{
	"发送", "发到", "发给", "发回", "发群", "群发", "转发", "投递", "推送", "播报",
	"send", "post", "deliver", "push", "broadcast",
}

var currentChannelTargetKeywords = []string{
	"这里", "当前会话", "这个群", "本群", "群里", "当前群", "这个飞书群", "飞书群", "im群", "im 群", "频道",
	"here", "this group", "current group", "this channel", "current channel", "chat", "group", "channel",
}

var currentChannelStandaloneDeliveryKeywords = []string{
	"发送", "群发", "投递", "推送", "播报",
	"send", "post", "deliver", "push", "broadcast",
}

var currentChannelDeliveryOptOutKeywords = []string{
	"不发送", "不要发送", "不用发送", "无需发送",
	"不推送", "不要推送", "不用推送", "无需推送",
	"不播报", "不要播报", "不用播报", "无需播报",
	"不投递", "不要投递", "不用投递", "无需投递",
	"不通知", "不要通知", "不用通知", "无需通知",
	"不发", "别发", "不要发", "不用发", "无需发",
	"静默",
	"do not send", "don't send", "no send", "do not post", "don't post",
	"do not push", "don't push", "no notification", "silent",
}

// ApplyCurrentChannelDefaults 在当前外部 IM 会话里，把明确“发送/推送/播报”的任务默认回投当前频道。
// 复杂任务默认走临时执行会话，避免长期污染用户正在聊天的会话上下文。
func ApplyCurrentChannelDefaults(args map[string]any, sctx contract.ServerContext) map[string]any {
	if !canDefaultToCurrentChannel(args, sctx) {
		return args
	}
	if strings.TrimSpace(argx.String(args, "execution_mode")) == "" {
		args["execution_mode"] = "temporary"
	}
	if strings.TrimSpace(argx.String(args, "reply_mode")) == "" {
		args["reply_mode"] = "channel"
	}
	return args
}

func canDefaultToCurrentChannel(args map[string]any, sctx contract.ServerContext) bool {
	if args == nil || strings.TrimSpace(argx.String(args, "execution_kind")) == "script" {
		return false
	}
	replyMode := strings.TrimSpace(argx.String(args, "reply_mode"))
	if replyMode != "" && replyMode != "channel" {
		return false
	}
	if !currentSessionKeyCanDeliverToExternalChannel(sctx.CurrentSessionKey) || !hasRunnableScheduleShape(args) {
		return false
	}
	if replyMode == "channel" {
		return true
	}
	text := defaultIntentText(args)
	if containsAnyKeyword(text, currentChannelDeliveryOptOutKeywords) {
		return false
	}
	if containsAnyKeyword(text, currentChannelDeliveryIntentKeywords) &&
		containsAnyKeyword(text, currentChannelTargetKeywords) {
		return true
	}
	return containsAnyKeyword(text, currentChannelStandaloneDeliveryKeywords)
}

func defaultIntentText(args map[string]any) string {
	parts := []string{
		strings.TrimSpace(argx.String(args, "name")),
		strings.TrimSpace(argx.String(args, "instruction")),
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func containsAnyKeyword(text string, keywords []string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}
