package realtime_test

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/app"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtime "github.com/nexus-research-lab/nexus/internal/runtime"
	permission "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	management "github.com/nexus-research-lab/nexus/internal/service/channels/management"
	dm "github.com/nexus-research-lab/nexus/internal/service/dm"
	realtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	workspace "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type bindingDM struct{ called atomic.Int32 }

func (d *bindingDM) HandleChat(context.Context, dm.Request) error { d.called.Add(1); return nil }

type bindingChannel struct{ sent chan string }

func (*bindingChannel) ChannelType() string         { return channels.ChannelTypeFeishu }
func (*bindingChannel) Start(context.Context) error { return nil }
func (*bindingChannel) Stop(context.Context) error  { return nil }
func (c *bindingChannel) SendDeliveryMessage(_ context.Context, target channels.DeliveryTarget, text string) (channels.DeliveryResult, error) {
	c.sent <- text
	return channels.DeliveryResult{Target: target}, nil
}

func TestIMRoomBindingQueuesOneMemberAndStopsOldRepliesAfterSwitch(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agents, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: "im-owner", Role: authctx.RoleOwner})
	agent := createTestAgent(t, agents, ctx, "负责人")
	other := createTestAgent(t, agents, ctx, "开发")
	rooms := app.NewRoomServiceWithDB(cfg, db, agents)
	room, err := rooms.CreateRoom(ctx, protocol.CreateRoomRequest{AgentIDs: []string{agent.AgentID, other.AgentID}, Name: "项目", PrivateMessagesEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	client := newFakeRoomClient()
	started := make(chan string, 4)
	release := make(chan struct{}, 4)
	var calls atomic.Int32
	client.onQuery = func(_ context.Context, prompt string) error {
		n := calls.Add(1)
		started <- prompt
		go func() {
			<-release
			sendFakeAssistantResult(client, fmt.Sprintf("im-answer-%d", n), fmt.Sprintf("处理完成-%d", n))
		}()
		return nil
	}
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{client}}
	manager := runtime.NewManager()
	permissions := permission.NewContext()
	service := realtime.NewServiceWithFactory(cfg, rooms, agents, manager, permissions, factory)
	router := channels.NewRouter(cfg, db, agents, permissions)
	transport := &bindingChannel{sent: make(chan string, 8)}
	router.RegisterForOwner("im-owner", transport)
	if err := router.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer router.Stop(ctx)
	control := channels.NewControlService(cfg, db, agents, router)
	control.SetRoomService(rooms)
	plainDM := &bindingDM{}
	ingress := channels.NewIngressService(cfg, agents, plainDM, router)
	ingress.SetControlService(control)
	ingress.SetRuntimePermissionContext(permissions)
	ingress.SetRoomRealtime(service)
	pairing, err := control.CreatePairing(ctx, "im-owner", channels.CreatePairingRequest{ChannelType: channels.ChannelTypeFeishu, ChatType: "dm", ExternalRef: "phone", AgentID: agent.AgentID})
	if err != nil {
		t.Fatal(err)
	}
	target := management.PairingSessionTarget{RoomID: room.Room.ID, ConversationID: room.Conversation.ID}
	pairing, err = control.UpdatePairing(ctx, "im-owner", pairing.PairingID, channels.UpdatePairingRequest{SessionTarget: &target, BindingVersion: &pairing.BindingVersion})
	if err != nil {
		t.Fatal(err)
	}
	request := channels.IngressRequest{OwnerUserID: "im-owner", Channel: channels.ChannelTypeFeishu, ChatType: "dm", Ref: "phone", Content: "只处理手机任务", ReqID: "phone-1"}
	first, err := ingress.Accept(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case prompt := <-started:
		if !strings.Contains(prompt, request.Content) {
			t.Fatalf("缺少外部输入: %s", prompt)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Room 未启动")
	}

	decisionCtx, cancelDecision := context.WithTimeout(ctx, 5*time.Second)
	defer cancelDecision()
	decisions := make(chan sdkpermission.Decision, 1)
	go func() {
		decision, _ := factory.LastOptions().Callbacks.PermissionHandler(decisionCtx, sdkpermission.Request{ToolName: "Write", Input: map[string]any{"file_path": "test.txt", "content": "test"}})
		decisions <- decision
	}()
	select {
	case text := <-transport.sent:
		if !strings.Contains(text, "/y") {
			t.Fatalf("权限通知不正确: %s", text)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("未收到成员权限通知")
	}
	approval := request
	approval.Content = "/y"
	approval.ReqID = "phone-approval"
	if _, err := ingress.Accept(ctx, approval); err != nil {
		t.Fatal(err)
	}
	select {
	case decision := <-decisions:
		if decision.Behavior != sdkpermission.BehaviorAllow {
			t.Fatalf("未允许原成员请求: %+v", decision)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("成员权限决定未返回")
	}
	select {
	case <-transport.sent:
	case <-time.After(5 * time.Second):
		t.Fatal("缺少权限决定回执")
	}
	if duplicate, err := ingress.Accept(ctx, request); err != nil || duplicate.RoundID != first.RoundID {
		t.Fatalf("重复入站: %+v %v", duplicate, err)
	}
	second := request
	second.ReqID = "phone-2"
	second.Content = "第二条任务"
	if _, err := ingress.Accept(ctx, second); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || plainDM.called.Load() != 0 {
		t.Fatal("忙碌时必须排队且不能启动独立 DM")
	}
	release <- struct{}{}
	select {
	case text := <-transport.sent:
		if text != "处理完成-1" {
			t.Fatalf("回信错误: %s", text)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("未收到完成回复")
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("第二条输入未接力")
	}
	independent := management.PairingSessionTarget{}
	if _, err := control.UpdatePairing(ctx, "im-owner", pairing.PairingID, channels.UpdatePairingRequest{SessionTarget: &independent, BindingVersion: &pairing.BindingVersion}); err != nil {
		t.Fatal(err)
	}
	release <- struct{}{}
	deadline := time.Now().Add(5 * time.Second)
	for service.CountRunningTasks(agent.AgentID) > 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if service.CountRunningTasks(agent.AgentID) > 0 {
		t.Fatal("原 Room 任务未完成")
	}
	select {
	case text := <-transport.sent:
		t.Fatalf("切换后不应发送旧回复: %s", text)
	default:
	}
	if len(factory.Options()) != 1 {
		t.Fatalf("应复用唯一成员 runtime: %d", len(factory.Options()))
	}
	history, err := workspace.NewRoomHistoryStore(cfg.WorkspacePath).ReadMessages("im-owner", room.Conversation.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range history {
		if strings.Contains(fmt.Sprint(entry), request.Content) {
			t.Fatal("私域输入泄漏到公区")
		}
	}
}
