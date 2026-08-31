package communication

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
)

type recordingExternalSessionGateway struct {
	ownerUserID string
	agentID     string
	sessionKey  string
	content     string
}

func (g *recordingExternalSessionGateway) ListAgentExternalSessions(
	context.Context,
	string,
	string,
	string,
) ([]channels.AgentExternalSession, error) {
	return nil, nil
}

func (g *recordingExternalSessionGateway) SendAgentExternalSessionMessage(
	_ context.Context,
	ownerUserID string,
	agentID string,
	sessionKey string,
	content string,
) (channels.DeliveryResult, error) {
	g.ownerUserID = ownerUserID
	g.agentID = agentID
	g.sessionKey = sessionKey
	g.content = content
	return channels.DeliveryResult{Target: channels.DeliveryTarget{
		Mode: channels.DeliveryModeExplicit, Channel: channels.ChannelTypeWeixinPersonal,
		SessionKey: sessionKey,
	}}, nil
}

func TestSendMessageRoutesExternalSessionThroughAgentGateway(t *testing.T) {
	gateway := &recordingExternalSessionGateway{}
	service := &Service{external: gateway}
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "owner-a", Role: authctx.RoleOwner,
	})
	result, err := service.sendMessage(ctx, "agent-a", SendRequest{
		TargetType: TargetTypeExternalSession,
		TargetID:   "agent:agent-a:weixin-personal:dm:account:recipient",
		Content:    "微信提醒",
	}, "", sendContext{})
	if err != nil {
		t.Fatal(err)
	}
	if gateway.ownerUserID != "owner-a" || gateway.agentID != "agent-a" ||
		gateway.sessionKey != result.SessionKey || gateway.content != "微信提醒" ||
		result.Channel != channels.ChannelTypeWeixinPersonal || result.Status != "delivered" {
		t.Fatalf("外部 Session 路由不正确: gateway=%+v result=%+v", gateway, result)
	}
}
