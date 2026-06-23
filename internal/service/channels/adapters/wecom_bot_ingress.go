package adapters

import (
	"encoding/json"
	"strconv"
	"strings"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
)

type weComBotParsedMessage struct {
	Kind       string
	MsgType    string
	MsgID      string
	FromUser   string
	SenderName string
	ChatType   string
	ChatID     string
	Content    string
	ReqID      string
}

func (c *WeComBotChannel) ingressRequestFromParsed(parsed weComBotParsedMessage) channelcontract.IngressRequest {
	chatType := "dm"
	ref := parsed.FromUser
	if parsed.ChatType == "group" || parsed.ChatID != "" {
		chatType = "group"
		ref = channelcontract.FirstNonEmpty(parsed.ChatID, parsed.FromUser)
	}
	streamID := channelcontract.NewID("stream")
	metadata := map[string]string{
		"req_id":    parsed.ReqID,
		"stream_id": streamID,
		"msg_type":  parsed.MsgType,
	}
	if parsed.ChatID != "" {
		metadata["chat_id"] = parsed.ChatID
	}
	return channelcontract.IngressRequest{
		Channel:      channelcontract.ChannelTypeWeChat,
		OwnerUserID:  c.ownerUserID,
		AccountID:    strings.TrimSpace(c.botID),
		ChatType:     chatType,
		Ref:          ref,
		ExternalName: channelcontract.FirstNonEmpty(parsed.SenderName, parsed.FromUser, parsed.ChatID),
		Content:      parsed.Content,
		RoundID:      parsed.MsgID,
		ReqID:        parsed.MsgID,
		Delivery: &channelcontract.DeliveryTarget{
			Mode:      channelcontract.DeliveryModeExplicit,
			Channel:   channelcontract.ChannelTypeWeChat,
			To:        ref,
			AccountID: parsed.ReqID,
			ThreadID:  streamID,
		},
		Message: channelmessage.NewInbound(channelmessage.InboundParams{
			Channel:           channelcontract.ChannelTypeWeChat,
			Target:            ref,
			PlatformMessageID: parsed.MsgID,
			SenderID:          parsed.FromUser,
			SenderName:        parsed.SenderName,
			ChatType:          chatType,
			Text:              parsed.Content,
			Metadata:          metadata,
		}),
	}
}

func parseWeComBotInboundMessage(raw json.RawMessage, reqID string) (weComBotParsedMessage, string, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return weComBotParsedMessage{}, "", err
	}
	source := weComBotMessageSource(payload)
	msgType := strings.ToLower(channelcontract.FirstNonEmpty(
		weComBotStringAt(source, "msgtype"),
		weComBotStringAt(source, "msg_type"),
		weComBotStringAt(source, "msgType"),
		weComBotStringAt(source, "message_type"),
		weComBotStringAt(source, "messageType"),
		weComBotStringAt(source, "type"),
	))
	if msgType == "" && weComBotTextContent(source) != "" {
		msgType = "text"
	}
	if msgType == "event" {
		return weComBotParsedMessage{Kind: "event", MsgType: msgType}, "event", nil
	}
	if msgType == "" {
		return weComBotParsedMessage{}, "empty_msg_type", nil
	}
	if msgType == "stream" {
		return weComBotParsedMessage{Kind: "stream-refresh", MsgType: msgType}, "stream_refresh", nil
	}

	content := ""
	switch msgType {
	case "text":
		content = weComBotTextContent(source)
	case "mixed":
		content = weComBotMixedText(source)
	default:
		return weComBotParsedMessage{Kind: "unsupported", MsgType: msgType}, "unsupported_msg_type", nil
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return weComBotParsedMessage{}, "empty_text", nil
	}

	fromUser := channelcontract.FirstNonEmpty(
		weComBotStringAt(source, "from", "userid"),
		weComBotStringAt(source, "from", "user_id"),
		weComBotStringAt(source, "from", "userId"),
		weComBotStringAt(source, "sender", "userid"),
		weComBotStringAt(source, "sender", "user_id"),
		weComBotStringAt(source, "sender", "userId"),
		weComBotStringAt(source, "sender", "id"),
		weComBotStringAt(source, "userid"),
		weComBotStringAt(source, "user_id"),
		weComBotStringAt(source, "userId"),
	)
	if fromUser == "" {
		return weComBotParsedMessage{}, "empty_from_user", nil
	}

	msgID := channelcontract.FirstNonEmpty(
		weComBotStringAt(source, "msgid"),
		weComBotStringAt(source, "msg_id"),
		weComBotStringAt(source, "msgId"),
		weComBotStringAt(source, "message_id"),
		weComBotStringAt(source, "messageId"),
		weComBotStringAt(source, "id"),
	)
	if msgID == "" {
		msgID = channelcontract.NewID("wecom_msg")
	}
	return weComBotParsedMessage{
		Kind:     "message",
		MsgType:  msgType,
		MsgID:    msgID,
		FromUser: fromUser,
		SenderName: channelcontract.FirstNonEmpty(
			weComBotStringAt(source, "sender", "name"),
			weComBotStringAt(source, "from", "name"),
			weComBotStringAt(source, "sender_name"),
			weComBotStringAt(source, "senderName"),
			weComBotStringAt(source, "nickname"),
		),
		ChatType: strings.ToLower(channelcontract.FirstNonEmpty(
			weComBotStringAt(source, "chattype"),
			weComBotStringAt(source, "chat_type"),
			weComBotStringAt(source, "chatType"),
			"single",
		)),
		ChatID: channelcontract.FirstNonEmpty(
			weComBotStringAt(source, "chatid"),
			weComBotStringAt(source, "chat_id"),
			weComBotStringAt(source, "chatId"),
			weComBotStringAt(source, "conversation_id"),
			weComBotStringAt(source, "conversationId"),
		),
		Content: content,
		ReqID:   channelcontract.FirstNonEmpty(reqID, msgID),
	}, "", nil
}

func weComBotMessageSource(payload map[string]any) map[string]any {
	candidates := []map[string]any{
		payload,
		weComBotMapAt(payload, "message"),
		weComBotMapAt(payload, "msg"),
		weComBotMapAt(payload, "data"),
		weComBotMapAt(payload, "event", "message"),
	}
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		if channelcontract.FirstNonEmpty(
			weComBotStringAt(candidate, "msgtype"),
			weComBotStringAt(candidate, "msg_type"),
			weComBotStringAt(candidate, "msgType"),
			weComBotStringAt(candidate, "message_type"),
			weComBotStringAt(candidate, "messageType"),
			weComBotStringAt(candidate, "type"),
			weComBotTextContent(candidate),
		) != "" {
			return candidate
		}
	}
	return payload
}

func weComBotTextContent(source map[string]any) string {
	return channelcontract.FirstNonEmpty(
		weComBotStringAt(source, "text", "content"),
		weComBotStringAt(source, "text", "text"),
		weComBotStringAt(source, "message", "text", "content"),
		weComBotStringAt(source, "content"),
		weComBotStringAt(source, "text_content"),
		weComBotStringAt(source, "textContent"),
	)
}

func weComBotMixedText(source map[string]any) string {
	items := firstNonEmptySlice(
		weComBotSliceAt(source, "mixed", "msg_item"),
		weComBotSliceAt(source, "mixed", "msgItem"),
		weComBotSliceAt(source, "mixed", "items"),
		weComBotSliceAt(source, "items"),
		weComBotSliceAt(source, "attachments"),
		weComBotSliceAt(source, "message", "items"),
		weComBotSliceAt(source, "message", "msg_item"),
	)
	parts := make([]string, 0, len(items))
	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		itemType := strings.ToLower(channelcontract.FirstNonEmpty(
			weComBotStringAt(itemMap, "msgtype"),
			weComBotStringAt(itemMap, "msg_type"),
			weComBotStringAt(itemMap, "msgType"),
			weComBotStringAt(itemMap, "type"),
		))
		if itemType != "" && itemType != "text" {
			continue
		}
		text := channelcontract.FirstNonEmpty(
			weComBotStringAt(itemMap, "text", "content"),
			weComBotStringAt(itemMap, "content"),
			weComBotStringAt(itemMap, "text"),
		)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func weComBotMapAt(source map[string]any, path ...string) map[string]any {
	value := weComBotValueAt(source, path...)
	result, _ := value.(map[string]any)
	return result
}

func weComBotSliceAt(source map[string]any, path ...string) []any {
	value := weComBotValueAt(source, path...)
	result, _ := value.([]any)
	return result
}

func weComBotStringAt(source map[string]any, path ...string) string {
	value := weComBotValueAt(source, path...)
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}

func weComBotIntAt(source map[string]any, path ...string) (int, bool) {
	value := weComBotValueAt(source, path...)
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		result, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return int(result), true
	case string:
		result, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, false
		}
		return result, true
	default:
		return 0, false
	}
}

func weComBotValueAt(source map[string]any, path ...string) any {
	if source == nil || len(path) == 0 {
		return nil
	}
	var current any = source
	for _, segment := range path {
		currentMap, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = currentMap[strings.TrimSpace(segment)]
	}
	return current
}

func firstNonEmptySlice(values ...[]any) []any {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return nil
}
