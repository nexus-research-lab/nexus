package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
)

func TestFeishuIngressCreatedTaskCanDeliverToConfiguredFeishuGroup(t *testing.T) {
	fixture := newAutomationIngressFixture(t)
	feishu := newAutomationIngressFeishuServer(t)
	fixture.configureFeishuDeliveryChannel(feishu.URL(), feishu.Client())

	fixture.acceptFeishuMessage("每天 9 点搜索重要新闻并发到这个飞书群", "feishu-event-deliver", "feishu-message-deliver")

	var created protocol.CronJob
	waitForAutomationIngress(t, 3*time.Second, func() bool {
		items, err := fixture.automation.ListTasks(context.Background(), fixture.cfg.DefaultAgentID)
		if err != nil || len(items) != 1 {
			return false
		}
		created = items[0]
		return created.Delivery.Channel == protocol.SessionChannelFeishu &&
			created.Delivery.To == "oc_group_123"
	})

	runResult, err := fixture.automation.RunTaskNow(context.Background(), created.JobID)
	if err != nil {
		t.Fatalf("立即运行 Feishu 投递任务失败: %v", err)
	}
	if runResult.RunID == nil || strings.TrimSpace(*runResult.RunID) == "" {
		t.Fatalf("立即运行应返回 run_id: %+v", runResult)
	}
	runID := strings.TrimSpace(*runResult.RunID)

	waitForAutomationIngress(t, 3*time.Second, func() bool {
		runs, listErr := fixture.automation.ListTaskRuns(context.Background(), created.JobID)
		return listErr == nil &&
			len(runs) == 1 &&
			runs[0].RunID == runID &&
			runs[0].DeliveryStatus == protocol.DeliveryStatusSucceeded
	})
	runs, err := fixture.automation.ListTaskRuns(context.Background(), created.JobID)
	if err != nil || len(runs) != 1 {
		t.Fatalf("读取 Feishu 投递 run 失败: runs=%+v err=%v", runs, err)
	}
	run := runs[0]
	if run.Status != protocol.RunStatusSucceeded ||
		run.DeliveryTo != "explicit:feishu:oc_group_123" ||
		run.DeliveryAttempts != 1 ||
		run.DeliveredAt == nil ||
		run.DeliveryError != nil ||
		run.DeliveryNextAttemptAt != nil {
		t.Fatalf("Feishu 已配置时应记录投递成功 ledger: %+v", run)
	}
	feishu.assertTextMessage(t, "oc_group_123", "今日新闻摘要")
}

func (f *automationIngressFixture) configureFeishuDeliveryChannel(baseURL string, client *http.Client) {
	f.t.Helper()
	if err := f.channelRouter.Start(context.Background()); err != nil {
		f.t.Fatalf("启动通道路由失败: %v", err)
	}
	f.t.Cleanup(func() {
		f.channelRouter.Stop(context.Background())
	})
	f.channelControl.SetHTTPClient(client)
	view, err := f.channelControl.UpsertChannelConfig(context.Background(), "", channels.ChannelTypeFeishu, channels.UpsertChannelConfigRequest{
		AgentID: f.cfg.DefaultAgentID,
		Config: map[string]string{
			"app_id":   "cli_test",
			"base_url": strings.TrimSpace(baseURL),
		},
		Credentials: map[string]string{
			"app_secret": "secret_test",
		},
	})
	if err != nil {
		f.t.Fatalf("配置 Feishu 投递通道失败: %v", err)
	}
	if view == nil || view.ConnectionState != "connected" {
		f.t.Fatalf("Feishu 投递通道应处于 connected: %+v", view)
	}
}

type automationIngressFeishuServer struct {
	baseURL string

	mu            sync.Mutex
	tokenRequests int
	messages      []map[string]string
	errors        []string
}

func newAutomationIngressFeishuServer(t *testing.T) *automationIngressFeishuServer {
	t.Helper()
	return &automationIngressFeishuServer{baseURL: "https://feishu.test"}
}

func (f *automationIngressFeishuServer) URL() string {
	return f.baseURL
}

func (f *automationIngressFeishuServer) Client() *http.Client {
	return &http.Client{Transport: f}
}

func (f *automationIngressFeishuServer) RoundTrip(request *http.Request) (*http.Response, error) {
	target, err := url.Parse(f.baseURL)
	if err != nil {
		return nil, err
	}
	if request.URL.Scheme != target.Scheme || request.URL.Host != target.Host {
		return nil, fmt.Errorf("unexpected feishu host: %s", request.URL.String())
	}
	recorder := httptest.NewRecorder()
	f.handle(recorder, request)
	return recorder.Result(), nil
}

func (f *automationIngressFeishuServer) handle(writer http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case "/open-apis/auth/v3/tenant_access_token/internal":
		f.handleTenantToken(writer, request)
	case "/open-apis/im/v1/messages":
		f.handleMessage(writer, request)
	default:
		f.fail(writer, http.StatusNotFound, "unknown feishu path: %s", request.URL.Path)
	}
}

func (f *automationIngressFeishuServer) handleTenantToken(writer http.ResponseWriter, request *http.Request) {
	var payload map[string]string
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		f.fail(writer, http.StatusBadRequest, "decode token request: %v", err)
		return
	}
	if payload["app_id"] != "cli_test" || payload["app_secret"] != "secret_test" {
		f.fail(writer, http.StatusUnauthorized, "unexpected token credentials: %+v", payload)
		return
	}
	f.mu.Lock()
	f.tokenRequests++
	f.mu.Unlock()
	_, _ = writer.Write([]byte(`{"code":0,"tenant_access_token":"tenant-token","expire":7200}`))
}

func (f *automationIngressFeishuServer) handleMessage(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Query().Get("receive_id_type") != "chat_id" {
		f.fail(writer, http.StatusBadRequest, "unexpected receive_id_type: %s", request.URL.RawQuery)
		return
	}
	if request.Header.Get("Authorization") != "Bearer tenant-token" {
		f.fail(writer, http.StatusUnauthorized, "unexpected authorization: %s", request.Header.Get("Authorization"))
		return
	}
	var payload map[string]string
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		f.fail(writer, http.StatusBadRequest, "decode message request: %v", err)
		return
	}
	f.mu.Lock()
	f.messages = append(f.messages, payload)
	f.mu.Unlock()
	_, _ = writer.Write([]byte(`{"code":0,"msg":"ok"}`))
}

func (f *automationIngressFeishuServer) assertTextMessage(t *testing.T, receiveID string, text string) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.errors) > 0 {
		t.Fatalf("fake Feishu 收到异常请求: %s", strings.Join(f.errors, "; "))
	}
	if f.tokenRequests != 1 {
		t.Fatalf("Feishu token 请求次数不正确: %d", f.tokenRequests)
	}
	if len(f.messages) != 1 {
		t.Fatalf("Feishu 消息请求次数不正确: %d payload=%+v", len(f.messages), f.messages)
	}
	message := f.messages[0]
	if message["receive_id"] != receiveID || message["msg_type"] != "text" {
		t.Fatalf("Feishu 消息目标不正确: %+v", message)
	}
	var content map[string]string
	if err := json.Unmarshal([]byte(message["content"]), &content); err != nil {
		t.Fatalf("解析 Feishu 消息 content 失败: %v", err)
	}
	if content["text"] != text {
		t.Fatalf("Feishu 消息正文不正确: %+v", content)
	}
}

func (f *automationIngressFeishuServer) fail(writer http.ResponseWriter, status int, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	f.mu.Lock()
	f.errors = append(f.errors, message)
	f.mu.Unlock()
	http.Error(writer, message, status)
}
