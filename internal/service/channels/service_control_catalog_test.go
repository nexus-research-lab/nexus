package channels

import (
	"slices"
	"testing"

	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
)

func TestChannelCatalogMarksImplementedChannelsReady(t *testing.T) {
	for _, item := range channelCatalog() {
		if item.RuntimeStatus == "planned" {
			t.Fatalf("%s 不应再标记为未上线", item.ChannelType)
		}
	}
	wechat, ok := channelCatalogByType(ChannelTypeWeChat)
	if !ok {
		t.Fatal("缺少企业微信通道")
	}
	if !wechat.SupportsGroup {
		t.Fatal("企业微信智能机器人通道应标记群聊能力")
	}
	weixinPersonal, ok := channelCatalogByType(ChannelTypeWeixinPersonal)
	if !ok {
		t.Fatal("缺少个人微信通道")
	}
	if weixinPersonal.RuntimeStatus != "ready" {
		t.Fatalf("个人微信应标记为内置可用，实际: %s", weixinPersonal.RuntimeStatus)
	}
	if weixinPersonal.Title != "微信" {
		t.Fatalf("微信通道前台标题不正确: %q", weixinPersonal.Title)
	}
	if !weixinPersonal.SupportsQRCode || weixinPersonal.SupportsGroup {
		t.Fatalf("个人微信能力标记不正确: %+v", weixinPersonal)
	}
	if !catalogHasCapability(weixinPersonal, channelmessage.CapabilityReceipt) {
		t.Fatalf("个人微信应声明本地消息回执能力: %+v", weixinPersonal.Capabilities)
	}
	feishu, ok := channelCatalogByType(ChannelTypeFeishu)
	if !ok {
		t.Fatal("缺少飞书通道")
	}
	if field, ok := catalogCredentialField(feishu, "connection_mode"); !ok || field.Required {
		t.Fatalf("飞书应暴露可选 connection_mode 便于线上切换长连接或 webhook: field=%+v ok=%v", field, ok)
	}
	for _, capability := range []channelmessage.Capability{
		channelmessage.CapabilityTyping,
		channelmessage.CapabilityThread,
		channelmessage.CapabilityReply,
		channelmessage.CapabilityReceipt,
	} {
		if !catalogHasCapability(feishu, capability) {
			t.Fatalf("飞书应声明 %s 能力: %+v", capability, feishu.Capabilities)
		}
	}
	telegram, ok := channelCatalogByType(ChannelTypeTelegram)
	if !ok {
		t.Fatal("缺少 Telegram 通道")
	}
	if !catalogHasCapability(telegram, channelmessage.CapabilityThread) ||
		!catalogHasCapability(telegram, channelmessage.CapabilityTyping) ||
		!catalogHasCapability(telegram, channelmessage.CapabilityReceipt) {
		t.Fatalf("Telegram 能力矩阵不完整: %+v", telegram.Capabilities)
	}
	if catalogHasCapability(wechat, channelmessage.CapabilityReceipt) {
		t.Fatalf("企业微信未返回稳定 message id，不应声明 receipt 能力: %+v", wechat.Capabilities)
	}
	dingtalk, ok := channelCatalogByType(ChannelTypeDingTalk)
	if !ok {
		t.Fatal("缺少钉钉通道")
	}
	if field, ok := catalogCredentialField(dingtalk, "robot_code"); !ok || field.Required {
		t.Fatalf("钉钉 Stream 回复不应强制要求 Robot Code: field=%+v ok=%v", field, ok)
	}
	for _, key := range []string{"base_url", "stream_base_url"} {
		if _, ok := catalogCredentialField(dingtalk, key); !ok {
			t.Fatalf("钉钉应暴露运行时可选字段 %s", key)
		}
	}
	for _, channelCase := range []struct {
		channelType string
		fieldKey    string
	}{
		{channelType: ChannelTypeWeChat, fieldKey: "base_url"},
		{channelType: ChannelTypeTelegram, fieldKey: "base_url"},
		{channelType: ChannelTypeDiscord, fieldKey: "base_url"},
	} {
		item, found := channelCatalogByType(channelCase.channelType)
		if !found {
			t.Fatalf("缺少通道 %s", channelCase.channelType)
		}
		if _, ok := catalogCredentialField(item, channelCase.fieldKey); !ok {
			t.Fatalf("%s 应暴露运行时可选字段 %s", channelCase.channelType, channelCase.fieldKey)
		}
	}
}

func catalogHasCapability(item ChannelCatalogItem, capability channelmessage.Capability) bool {
	return slices.Contains(item.Capabilities, capability)
}

func catalogCredentialField(item ChannelCatalogItem, key string) (ChannelCredentialField, bool) {
	for _, field := range item.CredentialFields {
		if field.Key == key {
			return field, true
		}
	}
	return ChannelCredentialField{}, false
}
