package adapters

import (
	"testing"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
)

func TestDecodeDingTalkIngressCallbackIncludesMessageEnvelope(t *testing.T) {
	t.Parallel()

	request, ignored, err := DecodeDingTalkIngressCallback([]byte(`{
		"openConversationId": "cid-1",
		"conversationType": "2",
		"conversationTitle": "日报群",
		"chatbotCorpId": "corp-1",
		"sessionWebhook": "https://dingtalk.test/session-webhook",
		"senderStaffId": "staff-1",
		"senderId": "sender-1",
		"senderNick": "Alice",
		"msgId": "ding-message-1",
		"msgtype": "text",
		"text": {"content": "检查本周日报任务"}
	}`))
	if err != nil {
		t.Fatalf("解析钉钉 ingress 失败: %v", err)
	}
	if ignored != "" || request == nil {
		t.Fatalf("钉钉文本消息不应被忽略: request=%+v ignored=%s", request, ignored)
	}
	if request.Delivery == nil || request.Delivery.To != "https://dingtalk.test/session-webhook" {
		t.Fatalf("钉钉回投应优先使用 sessionWebhook: %+v", request.Delivery)
	}
	if request.Message == nil ||
		request.Message.Channel != channelcontract.ChannelTypeDingTalk ||
		request.Message.Target != "cid-1" ||
		request.Message.PlatformMessageID != "ding-message-1" ||
		request.Message.SenderID != "staff-1" ||
		request.Message.SenderName != "Alice" ||
		request.Message.ChatType != "group" ||
		request.Message.Text != "检查本周日报任务" ||
		request.Message.Metadata["conversation_title"] != "日报群" {
		t.Fatalf("钉钉 ingress envelope 不正确: %+v", request.Message)
	}
}

func TestDecodeDingTalkIngressCallbackKeepsDirectUsersSeparate(t *testing.T) {
	t.Parallel()

	payloads := []string{
		`{"conversationType":"1","senderStaffId":"staff-1","senderNick":"Alice","msgId":"ding-message-1","msgtype":"text","text":{"content":"hello"}}`,
		`{"conversationType":"1","senderStaffId":"staff-2","senderNick":"Bob","msgId":"ding-message-2","msgtype":"text","text":{"content":"hello"}}`,
	}
	seenRefs := map[string]bool{}
	for _, payload := range payloads {
		request, ignored, err := DecodeDingTalkIngressCallback([]byte(payload))
		if err != nil {
			t.Fatalf("解析钉钉私聊 ingress 失败: %v", err)
		}
		if ignored != "" || request == nil {
			t.Fatalf("钉钉私聊文本消息不应被忽略: request=%+v ignored=%s", request, ignored)
		}
		if request.ChatType != "dm" || request.Ref == "" {
			t.Fatalf("钉钉私聊 session ref 不正确: %+v", request)
		}
		if request.Delivery == nil || request.Delivery.To != request.Ref {
			t.Fatalf("钉钉私聊回投目标不正确: %+v", request.Delivery)
		}
		if seenRefs[request.Ref] {
			t.Fatalf("不同钉钉私聊用户不应复用 session ref: %+v", request)
		}
		seenRefs[request.Ref] = true
	}
}
