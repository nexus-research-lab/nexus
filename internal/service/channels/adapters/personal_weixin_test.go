package adapters

import (
	"context"
	"encoding/json"
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

type recordingPersonalWeixinRuntimeStore struct {
	mu          sync.Mutex
	loadCursor  string
	savedCursor string
	expired     string
	saved       chan struct{}
	marked      chan struct{}
}

func (s *recordingPersonalWeixinRuntimeStore) LoadPersonalWeixinCursor(context.Context, string, string) (string, error) {
	return s.loadCursor, nil
}

func (s *recordingPersonalWeixinRuntimeStore) SavePersonalWeixinCursor(_ context.Context, _ string, _ string, cursor string) error {
	s.mu.Lock()
	s.savedCursor = cursor
	s.mu.Unlock()
	select {
	case s.saved <- struct{}{}:
	default:
	}
	return nil
}

func (s *recordingPersonalWeixinRuntimeStore) MarkPersonalWeixinLoginExpired(_ context.Context, _ string, _ string, message string) error {
	s.mu.Lock()
	s.expired = message
	s.mu.Unlock()
	select {
	case s.marked <- struct{}{}:
	default:
	}
	return nil
}

func (r *recordingPersonalWeixinIngress) Accept(_ context.Context, request channelcontract.IngressRequest) (*channelcontract.IngressResult, error) {
	r.requests = append(r.requests, request)
	return &channelcontract.IngressResult{
		Channel: request.Channel,
		AgentID: request.AgentID,
	}, nil
}

func TestPersonalWeixinMultiAccountChannelAdoptsRunningReplacedAccount(t *testing.T) {
	authByRecipient := map[string]string{}
	var authMu sync.Mutex
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(request.URL.Path, "/ilink/bot/msg/notifystart"),
			strings.Contains(request.URL.Path, "/ilink/bot/msg/notifystop"):
			return jsonResponse(`{"ret":0}`), nil
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
	oldIngress := &recordingPersonalWeixinIngress{}
	accountOne.SetIngress(oldIngress)
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
	newIngress := &recordingPersonalWeixinIngress{}
	replacement.SetIngress(newIngress)
	if err := replacement.Start(runCtx); err != nil {
		t.Fatalf("启动多账号个人微信 channel 失败: %v", err)
	}
	defer replacement.Stop(context.Background())
	if !replacement.AdoptReplacedChannel(accountOne) {
		t.Fatal("多账号个人微信应接管同账号的已运行 channel")
	}
	for _, id := range []string{"account-1", "account-2"} {
		account := replacement.snapshotAccountMap()[id]
		account.handleMessage(context.Background(), personalWeixinMessage{
			FromUserID:  "wx-user",
			MessageType: personalWeixinMessageTypeUser,
			ItemList: []personalWeixinMessageItem{{
				Type:     personalWeixinItemTypeText,
				TextItem: personalWeixinTextItem{Text: "检查任务"},
			}},
		})
	}
	if len(oldIngress.requests) != 0 {
		t.Fatalf("接管后的账号仍使用旧入口: %+v", oldIngress.requests)
	}
	if len(newIngress.requests) != 2 ||
		newIngress.requests[0].AccountID != "account-1" ||
		newIngress.requests[1].AccountID != "account-2" {
		t.Fatalf("两个账号应通过新入口提交消息: %+v", newIngress.requests)
	}

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

func TestPersonalWeixinMultiAccountChannelDoesNotRestoreRemovedAccount(t *testing.T) {
	removed := NewPersonalWeixinChannel(PersonalWeixinClientConfig{
		BaseURL:   "https://weixin.test",
		Token:     "token-removed",
		AccountID: "account-removed",
	}, nil)
	replacement := NewPersonalWeixinMultiAccountChannel(nil)

	if replacement.AdoptReplacedChannel(removed) {
		t.Fatal("新配置中已不存在的账号不得从旧 runtime 接管回来")
	}
	if accounts := replacement.snapshotAccounts(); len(accounts) != 0 {
		t.Fatalf("删除账号热重载后不得复活旧账号: %+v", accounts)
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
		Mode:         channelcontract.DeliveryModeExplicit,
		Channel:      channelcontract.ChannelTypeWeixinPersonal,
		To:           "wx-user-1",
		ContextToken: "ctx-token-1",
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

func TestPersonalWeixinChannelStopsPollingWhenLoginExpires(t *testing.T) {
	store := &recordingPersonalWeixinRuntimeStore{
		saved:  make(chan struct{}, 1),
		marked: make(chan struct{}, 1),
	}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/ilink/bot/msg/notifystart", "/ilink/bot/msg/notifystop":
			return jsonResponse(`{"ret":0}`), nil
		case "/ilink/bot/getupdates":
			return jsonResponse(`{"ret":-14,"errcode":-14,"errmsg":"session timeout"}`), nil
		default:
			t.Fatalf("未知个人微信请求路径: %s", request.URL.Path)
			return nil, nil
		}
	})}
	channel := NewPersonalWeixinChannel(PersonalWeixinClientConfig{
		BaseURL:      "https://weixin.test",
		Token:        "expired-token",
		AccountID:    "account-1",
		RuntimeStore: store,
	}, client).WithOwner("owner-a")

	if err := channel.Start(context.Background()); err != nil {
		t.Fatalf("启动个人微信 channel 失败: %v", err)
	}
	select {
	case <-store.marked:
	case <-time.After(2 * time.Second):
		t.Fatal("iLink -14 未投影为登录失效")
	}
	if err := channel.Stop(context.Background()); err != nil {
		t.Fatalf("停止登录失效 channel 失败: %v", err)
	}
	store.mu.Lock()
	expired := store.expired
	store.mu.Unlock()
	if !strings.Contains(expired, "scan QR code") || !strings.Contains(expired, "-14") {
		t.Fatalf("登录失效说明不完整: %q", expired)
	}
}
