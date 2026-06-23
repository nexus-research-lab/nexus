package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
)

type recordingPersonalWeixinIngress struct {
	requests []channelcontract.IngressRequest
}

func (r *recordingPersonalWeixinIngress) Accept(_ context.Context, request channelcontract.IngressRequest) (*channelcontract.IngressResult, error) {
	r.requests = append(r.requests, request)
	return &channelcontract.IngressResult{
		Channel: request.Channel,
		AgentID: request.AgentID,
	}, nil
}

func TestPersonalWeixinChannelSendDeliveryMessage(t *testing.T) {
	var receivedPath string
	var receivedAuth string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedPath = request.URL.Path
		receivedAuth = request.Header.Get("Authorization")
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("解析个人微信回投请求失败: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ret":0}`))
	}))
	defer server.Close()

	channel := NewPersonalWeixinChannel(PersonalWeixinClientConfig{
		BaseURL:   server.URL,
		Token:     "token-1",
		AccountID: "account-1",
	}, server.Client())
	_, err := channel.SendDeliveryMessage(context.Background(), channelcontract.DeliveryTarget{
		Mode:     channelcontract.DeliveryModeExplicit,
		Channel:  channelcontract.ChannelTypeWeixinPersonal,
		To:       "wx-user-1",
		ThreadID: "ctx-token-1",
	}, "你好")
	if err != nil {
		t.Fatalf("个人微信回投失败: %v", err)
	}
	if receivedPath != "/ilink/bot/sendmessage" {
		t.Fatalf("个人微信回投路径不正确: %s", receivedPath)
	}
	if receivedAuth != "Bearer token-1" {
		t.Fatalf("个人微信回投 Authorization 不正确: %q", receivedAuth)
	}
	message, ok := payload["msg"].(map[string]any)
	if !ok {
		t.Fatalf("个人微信回投 msg 不正确: %+v", payload)
	}
	if message["to_user_id"] != "wx-user-1" || message["context_token"] != "ctx-token-1" {
		t.Fatalf("个人微信回投目标不正确: %+v", message)
	}
	items, ok := message["item_list"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("个人微信回投 item_list 不正确: %+v", message)
	}
	textItem := items[0].(map[string]any)["text_item"].(map[string]any)
	if textItem["text"] != "你好" {
		t.Fatalf("个人微信回投文本不正确: %+v", textItem)
	}
	if _, ok := payload["base_info"].(map[string]any); !ok {
		t.Fatalf("个人微信回投应携带 base_info: %+v", payload)
	}
}

func TestPersonalWeixinMultiAccountChannelRoutesByAccountID(t *testing.T) {
	var receivedAuth string
	var receivedTo string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedAuth = request.Header.Get("Authorization")
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("解析个人微信回投请求失败: %v", err)
		}
		message, _ := payload["msg"].(map[string]any)
		receivedTo, _ = message["to_user_id"].(string)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ret":0}`))
	}))
	defer server.Close()

	channel := NewPersonalWeixinMultiAccountChannel([]*PersonalWeixinChannel{
		NewPersonalWeixinChannel(PersonalWeixinClientConfig{
			BaseURL:   server.URL,
			Token:     "token-1",
			AccountID: "account-1",
		}, server.Client()),
		NewPersonalWeixinChannel(PersonalWeixinClientConfig{
			BaseURL:   server.URL,
			Token:     "token-2",
			AccountID: "account-2",
		}, server.Client()),
	})

	_, err := channel.SendDeliveryMessage(context.Background(), channelcontract.DeliveryTarget{
		Mode:      channelcontract.DeliveryModeExplicit,
		Channel:   channelcontract.ChannelTypeWeixinPersonal,
		To:        "wx-user-1",
		AccountID: "account-2",
	}, "你好")
	if err != nil {
		t.Fatalf("多账号个人微信回投失败: %v", err)
	}
	if receivedAuth != "Bearer token-2" || receivedTo != "wx-user-1" {
		t.Fatalf("多账号个人微信未按 account_id 分流: auth=%q to=%q", receivedAuth, receivedTo)
	}

	_, err = channel.SendDeliveryMessage(context.Background(), channelcontract.DeliveryTarget{
		Mode:    channelcontract.DeliveryModeExplicit,
		Channel: channelcontract.ChannelTypeWeixinPersonal,
		To:      "wx-user-1",
	}, "你好")
	if err == nil || !strings.Contains(err.Error(), "requires account_id") {
		t.Fatalf("多账号个人微信缺少 account_id 应拒绝: %v", err)
	}
}

func TestPersonalWeixinMultiAccountChannelStopsStartedAccountsOnStartFailure(t *testing.T) {
	startErr := errors.New("start account failed")
	accounts := []*PersonalWeixinChannel{
		NewPersonalWeixinChannel(PersonalWeixinClientConfig{AccountID: "account-1"}, nil),
		NewPersonalWeixinChannel(PersonalWeixinClientConfig{AccountID: "account-2"}, nil),
		NewPersonalWeixinChannel(PersonalWeixinClientConfig{AccountID: "account-3"}, nil),
	}
	started := make([]string, 0)
	stopped := make([]string, 0)

	err := startPersonalWeixinAccounts(
		context.Background(),
		accounts,
		func(account *PersonalWeixinChannel, _ context.Context) error {
			if account.accountID == "account-2" {
				return startErr
			}
			started = append(started, account.accountID)
			return nil
		},
		func(account *PersonalWeixinChannel, _ context.Context) error {
			stopped = append(stopped, account.accountID)
			return nil
		},
	)

	if !errors.Is(err, startErr) {
		t.Fatalf("多账号启动失败应返回原始错误: %v", err)
	}
	if strings.Join(started, ",") != "account-1" {
		t.Fatalf("启动顺序不正确: %+v", started)
	}
	if strings.Join(stopped, ",") != "account-1" {
		t.Fatalf("第二个账号启动失败时应回滚已启动账号: %+v", stopped)
	}
}

func TestPersonalWeixinMultiAccountChannelAdoptsRunningReplacedAccount(t *testing.T) {
	authByRecipient := map[string]string{}
	var authMu sync.Mutex
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(request.URL.Path, "/ilink/bot/getupdates"):
			<-request.Context().Done()
			return nil, request.Context().Err()
		case strings.Contains(request.URL.Path, "/ilink/bot/sendmessage"):
			var payload map[string]any
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Fatalf("解析个人微信回投请求失败: %v", err)
			}
			message, _ := payload["msg"].(map[string]any)
			to, _ := message["to_user_id"].(string)
			authMu.Lock()
			authByRecipient[to] = request.Header.Get("Authorization")
			authMu.Unlock()
			return jsonResponse(`{"ret":0}`), nil
		default:
			t.Fatalf("未知个人微信请求路径: %s", request.URL.Path)
			return nil, nil
		}
	})}

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	accountOne := NewPersonalWeixinChannel(PersonalWeixinClientConfig{
		BaseURL:   "https://weixin.test",
		Token:     "token-1",
		AccountID: "account-1",
	}, client)
	if err := accountOne.Start(runCtx); err != nil {
		t.Fatalf("启动第一个个人微信账号失败: %v", err)
	}

	replacement := NewPersonalWeixinMultiAccountChannel([]*PersonalWeixinChannel{
		NewPersonalWeixinChannel(PersonalWeixinClientConfig{
			BaseURL:   "https://weixin.test",
			Token:     "token-1",
			AccountID: "account-1",
		}, client),
		NewPersonalWeixinChannel(PersonalWeixinClientConfig{
			BaseURL:   "https://weixin.test",
			Token:     "token-2",
			AccountID: "account-2",
		}, client),
	})
	if !replacement.AdoptReplacedChannel(accountOne) {
		t.Fatal("多账号个人微信应接管同账号的已运行 channel")
	}
	if err := replacement.Start(runCtx); err != nil {
		t.Fatalf("启动多账号个人微信 channel 失败: %v", err)
	}
	defer replacement.Stop(context.Background())

	accountOne.mu.RLock()
	accountOneStillRunning := accountOne.cancel != nil
	accountOne.mu.RUnlock()
	if !accountOneStillRunning {
		t.Fatal("第二个微信扫码后不应停止第一个已运行账号")
	}

	if _, err := replacement.SendDeliveryMessage(context.Background(), channelcontract.DeliveryTarget{
		Mode:      channelcontract.DeliveryModeExplicit,
		Channel:   channelcontract.ChannelTypeWeixinPersonal,
		To:        "wx-user-1",
		AccountID: "account-1",
	}, "给账号一"); err != nil {
		t.Fatalf("账号一回投失败: %v", err)
	}
	if _, err := replacement.SendDeliveryMessage(context.Background(), channelcontract.DeliveryTarget{
		Mode:      channelcontract.DeliveryModeExplicit,
		Channel:   channelcontract.ChannelTypeWeixinPersonal,
		To:        "wx-user-2",
		AccountID: "account-2",
	}, "给账号二"); err != nil {
		t.Fatalf("账号二回投失败: %v", err)
	}

	authMu.Lock()
	defer authMu.Unlock()
	if authByRecipient["wx-user-1"] != "Bearer token-1" {
		t.Fatalf("账号一应复用已运行 channel，不应切到新建 channel: %+v", authByRecipient)
	}
	if authByRecipient["wx-user-2"] != "Bearer token-2" {
		t.Fatalf("账号二应使用新扫码账号 channel: %+v", authByRecipient)
	}
}

func TestPersonalWeixinChannelSendDeliveryTyping(t *testing.T) {
	var getConfigCalls int
	statuses := make([]float64, 0, 2)
	var getConfigPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("解析个人微信 typing 请求失败: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/ilink/bot/getconfig":
			getConfigCalls++
			getConfigPayload = payload
			_, _ = writer.Write([]byte(`{"ret":0,"typing_ticket":"ticket-1"}`))
		case "/ilink/bot/sendtyping":
			status, _ := payload["status"].(float64)
			statuses = append(statuses, status)
			if payload["ilink_user_id"] != "wx-user-1" || payload["typing_ticket"] != "ticket-1" {
				t.Fatalf("个人微信 typing payload 不正确: %+v", payload)
			}
			_, _ = writer.Write([]byte(`{"ret":0}`))
		default:
			t.Fatalf("未知个人微信请求路径: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	channel := NewPersonalWeixinChannel(PersonalWeixinClientConfig{
		BaseURL: server.URL,
		Token:   "token-1",
	}, server.Client())
	target := channelcontract.DeliveryTarget{
		Mode:     channelcontract.DeliveryModeExplicit,
		Channel:  channelcontract.ChannelTypeWeixinPersonal,
		To:       "wx-user-1",
		ThreadID: "ctx-token-1",
	}

	if err := channel.SendDeliveryTyping(context.Background(), target, true); err != nil {
		t.Fatalf("个人微信 typing start 失败: %v", err)
	}
	if err := channel.SendDeliveryTyping(context.Background(), target, false); err != nil {
		t.Fatalf("个人微信 typing cancel 失败: %v", err)
	}

	if getConfigCalls != 1 {
		t.Fatalf("typing ticket 应缓存，getconfig calls=%d", getConfigCalls)
	}
	if getConfigPayload["ilink_user_id"] != "wx-user-1" || getConfigPayload["context_token"] != "ctx-token-1" {
		t.Fatalf("getconfig payload 不正确: %+v", getConfigPayload)
	}
	if len(statuses) != 2 || statuses[0] != personalWeixinTypingActive || statuses[1] != personalWeixinTypingCancel {
		t.Fatalf("typing status 顺序不正确: %+v", statuses)
	}
}

func TestPersonalWeixinChannelTypingIgnoresGetConfigFailure(t *testing.T) {
	getConfigCalls := 0
	sendTypingCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/ilink/bot/getconfig":
			getConfigCalls++
			_, _ = writer.Write([]byte(`{"ret":1,"errmsg":"typing unavailable"}`))
		case "/ilink/bot/sendtyping":
			sendTypingCalls++
			_, _ = writer.Write([]byte(`{"ret":0}`))
		default:
			t.Fatalf("未知个人微信请求路径: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	channel := NewPersonalWeixinChannel(PersonalWeixinClientConfig{
		BaseURL: server.URL,
		Token:   "token-1",
	}, server.Client())
	err := channel.SendDeliveryTyping(context.Background(), channelcontract.DeliveryTarget{
		Mode:    channelcontract.DeliveryModeExplicit,
		Channel: channelcontract.ChannelTypeWeixinPersonal,
		To:      "wx-user-1",
	}, true)
	if err != nil {
		t.Fatalf("typing getconfig 失败应静默降级: %v", err)
	}
	if getConfigCalls != 1 || sendTypingCalls != 0 {
		t.Fatalf("typing 降级不正确: getconfig=%d sendtyping=%d", getConfigCalls, sendTypingCalls)
	}
}

func TestPersonalWeixinChannelSendDeliveryMessageChecksBusinessError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ret":1,"errmsg":"send unavailable"}`))
	}))
	defer server.Close()

	channel := NewPersonalWeixinChannel(PersonalWeixinClientConfig{
		BaseURL: server.URL,
		Token:   "token-1",
	}, server.Client())
	_, err := channel.SendDeliveryMessage(context.Background(), channelcontract.DeliveryTarget{
		Mode:    channelcontract.DeliveryModeExplicit,
		Channel: channelcontract.ChannelTypeWeixinPersonal,
		To:      "wx-user-1",
	}, "你好")
	if err == nil || !strings.Contains(err.Error(), "ret=1") {
		t.Fatalf("个人微信业务失败应返回错误: %v", err)
	}
}

func TestPersonalWeixinChannelHandlesTextMessage(t *testing.T) {
	ingress := &recordingPersonalWeixinIngress{}
	channel := NewPersonalWeixinChannel(PersonalWeixinClientConfig{
		Token:     "token-1",
		AccountID: "account-1",
	}, nil).WithOwner("owner-a")
	channel.SetIngress(ingress)

	channel.handleMessage(context.Background(), personalWeixinMessage{
		FromUserID:   "wx-user-1",
		MessageID:    42,
		ClientID:     "client-42",
		CreateTimeMS: 1700000000000,
		SessionID:    "session-1",
		GroupID:      "group-1",
		ContextToken: "ctx-token-1",
		MessageType:  personalWeixinMessageTypeUser,
		ItemList: []personalWeixinMessageItem{{
			Type: personalWeixinItemTypeText,
			TextItem: personalWeixinTextItem{
				Text: "检查今日任务",
			},
		}},
	})

	if len(ingress.requests) != 1 {
		t.Fatalf("个人微信消息未进入 ingress: %+v", ingress.requests)
	}
	request := ingress.requests[0]
	if request.OwnerUserID != "owner-a" || request.Channel != channelcontract.ChannelTypeWeixinPersonal || request.Ref != "wx-user-1" {
		t.Fatalf("个人微信 ingress 基础字段不正确: %+v", request)
	}
	if request.ThreadID != "" {
		t.Fatalf("个人微信 session key 不应使用 context_token: %+v", request)
	}
	if request.Delivery == nil || request.Delivery.To != "wx-user-1" || request.Delivery.ThreadID != "ctx-token-1" {
		t.Fatalf("个人微信 remembered delivery 不正确: %+v", request.Delivery)
	}
	if !strings.Contains(request.Content, "检查今日任务") {
		t.Fatalf("个人微信文本不正确: %+v", request)
	}
	if request.RoundID != "42" || request.ReqID != "42" {
		t.Fatalf("个人微信消息 id 未进入入口请求: %+v", request)
	}
	if request.Message == nil ||
		request.Message.Channel != channelcontract.ChannelTypeWeixinPersonal ||
		request.Message.PlatformMessageID != "42" ||
		request.Message.ThreadID != "ctx-token-1" ||
		request.Message.SenderID != "wx-user-1" ||
		request.Message.Text != "检查今日任务" ||
		request.Message.Metadata["client_id"] != "client-42" ||
		request.Message.Metadata["session_id"] != "session-1" ||
		request.Message.Metadata["group_id"] != "group-1" {
		t.Fatalf("个人微信入口 envelope 不正确: %+v", request.Message)
	}
	if !request.Message.ReceivedAt.Equal(time.UnixMilli(1700000000000).UTC()) {
		t.Fatalf("个人微信入口时间不正确: %+v", request.Message.ReceivedAt)
	}
}
