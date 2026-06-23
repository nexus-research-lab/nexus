package message

import (
	"strconv"
	"strings"
)

// RuntimeMetadata 把标准入站消息投影为 runtime/history 可持久化 metadata。
func RuntimeMetadata(message *Inbound) map[string]string {
	if message == nil {
		return nil
	}
	metadata := make(map[string]string)
	addMetadata(metadata, "im.direction", string(message.Direction))
	addMetadata(metadata, "im.channel", message.Channel)
	addMetadata(metadata, "im.target", message.Target)
	addMetadata(metadata, "im.platform_message_id", message.PlatformMessageID)
	addMetadata(metadata, "im.thread_id", message.ThreadID)
	addMetadata(metadata, "im.reply_to_id", message.ReplyToID)
	addMetadata(metadata, "im.sender_id", message.SenderID)
	addMetadata(metadata, "im.sender_name", message.SenderName)
	addMetadata(metadata, "im.chat_type", message.ChatType)
	if message.Edited {
		metadata["im.edited"] = "true"
	}
	if !message.ReceivedAt.IsZero() {
		metadata["im.received_at_unix_ms"] = strconv.FormatInt(message.ReceivedAt.UnixMilli(), 10)
	}
	for key, value := range message.Metadata {
		addMetadata(metadata, "im.meta."+key, value)
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func addMetadata(target map[string]string, key string, value string) {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return
	}
	target[key] = value
}
