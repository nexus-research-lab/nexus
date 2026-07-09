package automation

import (
	"context"
	"testing"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"

	_ "modernc.org/sqlite"
)

func TestDeliverJobObservationUsesTaskOwnerContext(t *testing.T) {
	delivery := &fakeDeliveryRouter{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		nil,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		delivery,
	)
	job := automationdomain.CronJob{
		JobID:       "job-owner",
		AgentID:     "agent-1",
		OwnerUserID: "user-1",
		Delivery: automationdomain.DeliveryTarget{
			Mode:    automationdomain.DeliveryModeExplicit,
			Channel: "feishu",
			To:      "oc_group",
		},
	}

	deliveryResult := service.deliverJobObservation(context.Background(), job, "", automationexec.ExecutionObservation{
		Status:     automationdomain.RunStatusSucceeded,
		ResultText: "今日新闻摘要",
	})
	if deliveryResult.Status != automationdomain.DeliveryStatusSucceeded || deliveryResult.Error != nil {
		t.Fatalf("投递状态异常: status=%s err=%v", deliveryResult.Status, deliveryResult.Error)
	}
	if deliveryResult.deliveryTo(job.Delivery) != "explicit:feishu:oc_group" {
		t.Fatalf("投递应记录实际目标，实际 %q", deliveryResult.deliveryTo(job.Delivery))
	}
	owners := delivery.OwnerUserIDs()
	if len(owners) != 1 || owners[0] != "user-1" {
		t.Fatalf("投递应使用任务 owner 上下文，实际 owners=%+v", owners)
	}
}

func TestDeliverJobObservationRecordsDeliveryReceipt(t *testing.T) {
	delivery := &fakeDeliveryRouter{
		receipt: channelmessage.NewReceipt(channelmessage.ReceiptParams{
			Channel:  channels.ChannelTypeTelegram,
			Target:   "-1001",
			ThreadID: "12",
			Parts:    []channelmessage.ReceiptPart{channelmessage.TextPart("42")},
		}),
	}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		nil,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		delivery,
	)
	job := automationdomain.CronJob{
		JobID:   "job-receipt",
		AgentID: "agent-1",
		Delivery: automationdomain.DeliveryTarget{
			Mode:     automationdomain.DeliveryModeExplicit,
			Channel:  channels.ChannelTypeTelegram,
			To:       "-1001",
			ThreadID: "12",
		},
	}

	deliveryResult := service.deliverJobObservation(context.Background(), job, "", automationexec.ExecutionObservation{
		Status:     automationdomain.RunStatusSucceeded,
		ResultText: "今日新闻摘要",
	})
	if deliveryResult.Receipt == nil || deliveryResult.Receipt.PrimaryPlatformMessageID != "42" {
		t.Fatalf("投递结果未记录平台回执: %+v", deliveryResult.Receipt)
	}
	if got := deliveryResult.deliveryTo(job.Delivery); got != "explicit:telegram:-1001:thread:12:message:42" {
		t.Fatalf("投递摘要未记录平台 message_id，实际 %q", got)
	}
}

func TestDeliverJobObservationPassesSourceSessionForLastDelivery(t *testing.T) {
	delivery := &fakeDeliveryRouter{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		nil,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		delivery,
	)
	sourceSessionKey := protocol.BuildAgentSessionKey("agent-1", channels.ChannelTypeWeixinPersonal, "dm", "wx-user-1", "")
	job := automationdomain.CronJob{
		JobID:   "job-session-last",
		AgentID: "agent-1",
		Delivery: automationdomain.DeliveryTarget{
			Mode: automationdomain.DeliveryModeLast,
		},
		Source: automationdomain.Source{
			SessionKey: sourceSessionKey,
		},
	}

	deliveryResult := service.deliverJobObservation(context.Background(), job, "", automationexec.ExecutionObservation{
		Status:     automationdomain.RunStatusSucceeded,
		ResultText: "定时提醒",
	})
	if deliveryResult.Status != automationdomain.DeliveryStatusSucceeded || deliveryResult.Error != nil {
		t.Fatalf("投递状态异常: status=%s err=%v", deliveryResult.Status, deliveryResult.Error)
	}
	calls := delivery.Calls()
	if len(calls) != 1 ||
		calls[0].Mode != channels.DeliveryModeLast ||
		calls[0].SessionKey != sourceSessionKey {
		t.Fatalf("last 投递应携带来源 IM session_key: %+v", calls)
	}
}
