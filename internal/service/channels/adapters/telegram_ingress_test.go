package adapters

import (
	"context"
	"testing"
)

func TestTelegramChannelHandleUpdateKeepsDMUsersSeparate(t *testing.T) {
	t.Parallel()

	channel := NewTelegramChannel("token", nil).WithOwner("owner-a")
	ingress := &recordingIngressAcceptor{}
	channel.SetIngress(ingress)
	for _, userID := range []int64{101, 202} {
		channel.handleUpdate(context.Background(), telegramUpdate{
			Message: &telegramMessage{
				MessageID: int(userID),
				Text:      "hello",
				From:      &telegramUser{ID: userID},
				Chat:      telegramChat{ID: userID, Type: "private"},
			},
		})
	}
	if len(ingress.requests) != 2 {
		t.Fatalf("两个 Telegram 私聊用户都应进入 ingress: %+v", ingress.requests)
	}
	seenRefs := map[string]bool{}
	for _, request := range ingress.requests {
		if request.OwnerUserID != "owner-a" || request.ChatType != "dm" || request.Ref == "" {
			t.Fatalf("Telegram DM ingress 基础字段不正确: %+v", request)
		}
		if request.Delivery == nil || request.Delivery.To != request.Ref {
			t.Fatalf("Telegram DM 回投目标不正确: %+v", request.Delivery)
		}
		if seenRefs[request.Ref] {
			t.Fatalf("不同 Telegram 私聊用户不应复用 session ref: %+v", ingress.requests)
		}
		seenRefs[request.Ref] = true
	}
}
