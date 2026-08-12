package channels

import (
	"context"
	"fmt"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

type recordingIngressCommandHandler struct {
	requests     []IngressCommandRequest
	result       IngressCommandResult
	err          error
	pendingCount int
	pendingErr   error
}

func TestIngressControlCommandsCoverEveryExternalIMChannel(t *testing.T) {
	channels := []string{
		ChannelTypeDiscord,
		ChannelTypeTelegram,
		ChannelTypeDingTalk,
		ChannelTypeWeChat,
		ChannelTypeWeixinPersonal,
		ChannelTypeFeishu,
	}
	for index, channelType := range channels {
		t.Run(channelType, func(t *testing.T) {
			cfg := newIngressTestConfig(t)
			db := migrateIngressSQLite(t, cfg.DatabaseURL)
			defer func() { _ = db.Close() }()

			agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
			defaultAgent, err := agentService.GetDefaultAgent(context.Background())
			if err != nil {
				t.Fatalf("初始化默认 Agent 失败: %v", err)
			}
			target := fmt.Sprintf("approval-target-%d", index)
			accountID := ""
			if channelType == ChannelTypeWeixinPersonal {
				accountID = "weixin-account"
			}
			dm := &fakeIngressDMHandler{}
			router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
			deliveryChannel := &recordingDeliveryChannel{channelType: channelType}
			router.RegisterForOwner("", deliveryChannel)
			if err = router.Start(context.Background()); err != nil {
				t.Fatalf("启动 channel router 失败: %v", err)
			}
			defer router.Stop(context.Background())
			control := NewControlService(cfg, db, agentService, router)
			if _, err = control.CreatePairing(context.Background(), "", CreatePairingRequest{
				ChannelType: channelType,
				AccountID:   accountID,
				ChatType:    protocol.RoomTypeDM,
				ExternalRef: target,
				AgentID:     defaultAgent.AgentID,
				Status:      PairingStatusActive,
			}); err != nil {
				t.Fatalf("创建 %s pairing 失败: %v", channelType, err)
			}
			commands := &recordingIngressCommandHandler{result: IngressCommandResult{
				Handled: true,
				Reply:   "权限请求已批准",
			}}
			service := NewIngressService(cfg, agentService, dm, router)
			service.SetControlService(control)
			service.SetCommandHandler(commands)

			_, err = service.Accept(context.Background(), IngressRequest{
				Channel:   channelType,
				AccountID: accountID,
				ChatType:  protocol.RoomTypeDM,
				Ref:       target,
				Content:   "/y permission_123",
				RoundID:   "round-command",
				ReqID:     "message-command",
				Delivery: &DeliveryTarget{
					Mode:      DeliveryModeExplicit,
					Channel:   channelType,
					To:        target,
					AccountID: accountID,
				},
			})
			if err != nil {
				t.Fatalf("%s Accept 控制命令失败: %v", channelType, err)
			}
			if len(commands.requests) != 1 || len(dm.requests) != 0 ||
				deliveryChannel.sentCount() != 1 || deliveryChannel.texts[0] != "权限请求已批准" {
				t.Fatalf("%s 命令链路不完整: commands=%+v dm=%+v sends=%d texts=%+v", channelType, commands.requests, dm.requests, deliveryChannel.sentCount(), deliveryChannel.texts)
			}
		})
	}
}

func (h *recordingIngressCommandHandler) HandleIngressCommand(
	_ context.Context,
	request IngressCommandRequest,
) (IngressCommandResult, error) {
	h.requests = append(h.requests, request)
	return h.result, h.err
}

func (h *recordingIngressCommandHandler) CountPendingIngressPermissionRequests(
	_ context.Context,
	_ IngressCommandRequest,
) (int, error) {
	return h.pendingCount, h.pendingErr
}

func TestIngressServiceConsumesControlCommandBeforeDMAndRepliesToCurrentIM(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	defaultAgent, err := agentService.GetDefaultAgent(context.Background())
	if err != nil {
		t.Fatalf("初始化默认 Agent 失败: %v", err)
	}
	dm := &fakeIngressDMHandler{}
	router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
	channel := &recordingDeliveryChannel{channelType: ChannelTypeFeishu}
	router.RegisterForOwner("", channel)
	if err = router.Start(context.Background()); err != nil {
		t.Fatalf("启动 channel router 失败: %v", err)
	}
	defer router.Stop(context.Background())
	control := NewControlService(cfg, db, agentService, router)
	if _, err = control.CreatePairing(context.Background(), "", CreatePairingRequest{
		ChannelType: ChannelTypeFeishu,
		ChatType:    "group",
		ExternalRef: "oc_approval_group",
		AgentID:     defaultAgent.AgentID,
	}); err != nil {
		t.Fatalf("创建 IM pairing 失败: %v", err)
	}
	commands := &recordingIngressCommandHandler{result: IngressCommandResult{
		Handled: true,
		Reply:   "权限请求已批准",
	}}
	service := NewIngressService(cfg, agentService, dm, router)
	service.SetControlService(control)
	service.SetCommandHandler(commands)

	request := IngressRequest{
		Channel:  ChannelTypeFeishu,
		ChatType: "group",
		Ref:      "oc_approval_group",
		Content:  "/y permission_123",
		RoundID:  "round-command-1",
		ReqID:    "message-command-1",
		Delivery: &DeliveryTarget{
			Mode:    DeliveryModeExplicit,
			Channel: ChannelTypeFeishu,
			To:      "oc_approval_group",
		},
	}
	result, err := service.Accept(context.Background(), request)
	if err != nil {
		t.Fatalf("Accept 控制命令失败: %v", err)
	}
	if result == nil || len(commands.requests) != 1 {
		t.Fatalf("控制命令未被消费: result=%+v requests=%+v", result, commands.requests)
	}
	if len(dm.requests) != 0 {
		t.Fatalf("控制命令不应作为普通聊天进入 Agent runtime: %+v", dm.requests)
	}
	if channel.sentCount() != 1 || len(channel.texts) != 1 || channel.texts[0] != "权限请求已批准" {
		t.Fatalf("命令结果没有回投当前 IM: targets=%+v texts=%+v", channel.targets, channel.texts)
	}
	route, err := router.GetSessionRoute(context.Background(), defaultAgent.AgentID, result.SessionKey)
	if err != nil || route == nil || route.To != "oc_approval_group" {
		t.Fatalf("控制命令处理前未固化最新会话路由: route=%+v err=%v", route, err)
	}

	duplicate, err := service.Accept(context.Background(), request)
	if err != nil || duplicate == nil || !duplicate.Duplicate {
		t.Fatalf("控制命令 req_id 应保持幂等: result=%+v err=%v", duplicate, err)
	}
	if len(commands.requests) != 1 || channel.sentCount() != 1 {
		t.Fatalf("重复控制命令不应重复审批或回复: commands=%d sends=%d", len(commands.requests), channel.sentCount())
	}
}
