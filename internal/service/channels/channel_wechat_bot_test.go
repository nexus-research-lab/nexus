package channels

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type fakeWeComBotSocket struct {
	writes  []map[string]any
	onWrite func(map[string]any)
}

func (f *fakeWeComBotSocket) ReadMessage() (int, []byte, error) {
	return 0, nil, errors.New("not implemented")
}

func (f *fakeWeComBotSocket) WriteJSON(value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var frame map[string]any
	if err = json.Unmarshal(payload, &frame); err != nil {
		return err
	}
	f.writes = append(f.writes, frame)
	if f.onWrite != nil {
		f.onWrite(frame)
	}
	return nil
}

func (f *fakeWeComBotSocket) Close() error {
	return nil
}

func TestWeComBotChannelHandlesLongConnectionTextMessage(t *testing.T) {
	ingress := &recordingIngressAcceptor{}
	channel := newWeComBotChannel("bot-1", "secret-1").WithOwner("owner-a")
	channel.SetIngress(ingress)

	frame := []byte(`{
		"cmd": "aibot_msg_callback",
		"headers": {"req_id": "callback-1"},
		"body": {
			"msgid": "msg-1",
			"msgtype": "text",
			"from": {"userid": "user-1", "name": "张三"},
			"chattype": "group",
			"chatid": "chat-1",
			"text": {"content": "检查今天日报"}
		}
	}`)
	if err := channel.handleFrame(context.Background(), frame); err != nil {
		t.Fatalf("企业微信智能机器人入站处理失败: %v", err)
	}
	if len(ingress.requests) != 1 {
		t.Fatalf("企业微信智能机器人入站数量不正确: %d", len(ingress.requests))
	}
	accepted := ingress.requests[0]
	if accepted.OwnerUserID != "owner-a" ||
		accepted.Channel != ChannelTypeWeChat ||
		accepted.ChatType != "group" ||
		accepted.Ref != "chat-1" ||
		accepted.ThreadID != "" ||
		accepted.Content != "检查今天日报" ||
		accepted.ReqID != "msg-1" {
		t.Fatalf("企业微信智能机器人 ingress 请求不正确: %+v", accepted)
	}
	if accepted.Delivery == nil ||
		accepted.Delivery.Channel != ChannelTypeWeChat ||
		accepted.Delivery.To != "chat-1" ||
		accepted.Delivery.AccountID != "callback-1" ||
		!strings.HasPrefix(accepted.Delivery.ThreadID, "stream_") {
		t.Fatalf("企业微信智能机器人回投目标不正确: %+v", accepted.Delivery)
	}
	if accepted.Message == nil ||
		accepted.Message.PlatformMessageID != "msg-1" ||
		accepted.Message.SenderID != "user-1" ||
		accepted.Message.ChatType != "group" ||
		accepted.Message.ThreadID != "" ||
		accepted.Message.Metadata["req_id"] != "callback-1" ||
		accepted.Message.Metadata["stream_id"] != accepted.Delivery.ThreadID {
		t.Fatalf("企业微信智能机器人入站 envelope 不正确: %+v", accepted.Message)
	}
}

func TestWeComBotChannelKeepsDirectUsersSeparate(t *testing.T) {
	ingress := &recordingIngressAcceptor{}
	channel := newWeComBotChannel("bot-1", "secret-1").WithOwner("owner-a")
	channel.SetIngress(ingress)

	frames := []string{
		`{
			"cmd": "aibot_msg_callback",
			"headers": {"req_id": "callback-1"},
			"body": {
				"msgid": "msg-1",
				"msgtype": "text",
				"from": {"userid": "user-1", "name": "张三"},
				"chattype": "single",
				"text": {"content": "hello"}
			}
		}`,
		`{
			"cmd": "aibot_msg_callback",
			"headers": {"req_id": "callback-2"},
			"body": {
				"msgid": "msg-2",
				"msgtype": "text",
				"from": {"userid": "user-2", "name": "李四"},
				"chattype": "single",
				"text": {"content": "hello"}
			}
		}`,
	}
	for _, frame := range frames {
		if err := channel.handleFrame(context.Background(), []byte(frame)); err != nil {
			t.Fatalf("企业微信 direct 入站处理失败: %v", err)
		}
	}
	if len(ingress.requests) != 2 {
		t.Fatalf("两个企业微信 direct 用户都应进入 ingress: %+v", ingress.requests)
	}
	seenRefs := map[string]bool{}
	for _, request := range ingress.requests {
		if request.OwnerUserID != "owner-a" || request.ChatType != "dm" || request.Ref == "" {
			t.Fatalf("企业微信 direct ingress 基础字段不正确: %+v", request)
		}
		if request.Delivery == nil || request.Delivery.To != request.Ref || request.Delivery.AccountID == "" {
			t.Fatalf("企业微信 direct 回投目标不正确: %+v", request.Delivery)
		}
		if seenRefs[request.Ref] {
			t.Fatalf("不同企业微信 direct 用户不应复用 session ref: %+v", ingress.requests)
		}
		seenRefs[request.Ref] = true
	}
}

func TestWeComBotChannelSendDeliveryMessageUsesStreamReply(t *testing.T) {
	socket := &fakeWeComBotSocket{}
	channel := newWeComBotChannel("bot-1", "secret-1")
	socket.onWrite = func(map[string]any) {
		if err := channel.handleFrame(context.Background(), []byte(`{
			"cmd": "aibot_respond_msg",
			"headers": {"req_id": "callback-1"},
			"body": {"errcode": 0, "errmsg": "ok"}
		}`)); err != nil {
			t.Errorf("企业微信智能机器人 ack 处理失败: %v", err)
		}
	}
	channel.setSocket(socket, true)

	_, err := channel.SendDeliveryMessage(context.Background(), DeliveryTarget{
		Mode:      DeliveryModeExplicit,
		Channel:   ChannelTypeWeChat,
		To:        "chat-1",
		AccountID: "callback-1",
		ThreadID:  "stream-1",
	}, "今日日报正常")
	if err != nil {
		t.Fatalf("企业微信智能机器人回投失败: %v", err)
	}
	if len(socket.writes) != 1 {
		t.Fatalf("企业微信智能机器人回投 frame 数量不正确: %d", len(socket.writes))
	}
	frame := socket.writes[0]
	if frame["cmd"] != weComBotResponseCommand {
		t.Fatalf("企业微信智能机器人回投命令不正确: %+v", frame)
	}
	headers, ok := frame["headers"].(map[string]any)
	if !ok || headers["req_id"] != "callback-1" {
		t.Fatalf("企业微信智能机器人回投 req_id 不正确: %+v", frame)
	}
	body, ok := frame["body"].(map[string]any)
	if !ok || body["msgtype"] != "stream" {
		t.Fatalf("企业微信智能机器人回投 body 不正确: %+v", frame)
	}
	stream, ok := body["stream"].(map[string]any)
	if !ok ||
		stream["id"] != "stream-1" ||
		stream["content"] != "今日日报正常" ||
		stream["finish"] != true {
		t.Fatalf("企业微信智能机器人 stream 回复不正确: %+v", frame)
	}
}

func TestWeComBotChannelHandlesBodyStatusFrame(t *testing.T) {
	channel := newWeComBotChannel("bot-1", "secret-1")
	channel.setSubscribeReqID("subscribe-1")

	if err := channel.handleFrame(context.Background(), []byte(`{
		"cmd": "aibot_subscribe",
		"headers": {"req_id": "subscribe-1"},
		"body": {"errcode": 0, "errmsg": "ok"}
	}`)); err != nil {
		t.Fatalf("企业微信智能机器人订阅 ack 处理失败: %v", err)
	}

	channel.mu.RLock()
	connected := channel.connected
	channel.mu.RUnlock()
	if !connected {
		t.Fatal("企业微信智能机器人订阅成功后应标记长连接 ready")
	}
}
