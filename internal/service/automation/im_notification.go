// INPUT: ScheduledTask 的结果接收 Session、持久 IM 会话 grant、执行结果与交互请求。
// OUTPUT: 发往 Nexus DM/Room 的结构化权限事件，以及经最新 pairing 校验的外部 IM 控制面通知。
// POS: Automation 交互通知路由；普通 Agent 结果不得在此改写。
package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
)

func externalIMSessionKey(sessionKey string) bool {
	parsed := protocol.ParseSessionKey(sessionKey)
	return parsed.IsStructured && parsed.Kind == protocol.SessionKeyKindAgent && externalIMChannel(parsed.Channel)
}

func externalIMChannel(channel string) bool {
	switch protocol.NormalizeStoredChannelType(channel) {
	case protocol.SessionChannelDiscord,
		protocol.SessionChannelTelegram,
		protocol.SessionChannelDingTalk,
		protocol.SessionChannelWeChat,
		protocol.SessionChannelWeixinPersonal,
		protocol.SessionChannelFeishu:
		return true
	default:
		return false
	}
}

func automationIMHeader(job automationdomain.ScheduledTask) string {
	name := firstNonEmpty(job.Name, job.JobID, "未命名任务")
	return fmt.Sprintf("【Nexus 定时任务 · %s】", name)
}

func (s *Service) notifyAutomationPermissionRequest(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	request automationdomain.AutomationPermissionRequest,
) {
	s.notifyAutomationPermissionSessionRequest(ctx, job, request)
	commands := ""
	switch request.Kind {
	case automationdomain.PermissionRequestKindTool, automationdomain.PermissionRequestKindScript:
		commands = "/y：允许本次\n/a：此任务始终允许\n/d：拒绝"
	case automationdomain.PermissionRequestKindConnectorReauth:
		commands = "请先在 Nexus 中重新连接，然后发送 /y\n/d：拒绝"
	case automationdomain.PermissionRequestKindHumanInput:
		commands = "请在 Nexus 中编辑任务补充必要信息，或发送 /d 拒绝本次运行"
	default:
		commands = "/d：拒绝"
	}
	body := strings.Join([]string{
		"需要权限确认",
		firstNonEmpty(request.Title, "任务需要额外权限"),
		strings.TrimSpace(request.Description),
		commands,
	}, "\n")
	s.deliverAutomationIMNotice(ctx, job, request.DeliverySessionKey, body)
}

func (s *Service) notifyAutomationPermissionDecision(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	request automationdomain.AutomationPermissionRequest,
	body string,
) {
	s.notifyAutomationPermissionSessionResolution(ctx, job, request)
	if fromPermissionIMCommand(ctx) {
		return
	}
	s.deliverAutomationIMNotice(ctx, job, request.DeliverySessionKey, body)
}

func (s *Service) notifyAutomationDeliveryDeadLetter(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	run automationdomain.ScheduledTaskRun,
) {
	body := fmt.Sprintf(
		"结果连续投递失败，已停止自动重试。\n运行 ID：%s\n请在 Nexus 的定时任务运行记录中检查通道配置并手动补投递。",
		strings.TrimSpace(run.RunID),
	)
	s.deliverAutomationIMNotice(ctx, job, job.Delivery.SessionKey, body)
}

func fromPermissionIMCommand(ctx context.Context) bool {
	value, _ := ctx.Value(permissionIMCommandContextKey{}).(bool)
	return value
}

func (s *Service) deliverAutomationIMNotice(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	deliverySessionKey string,
	body string,
) {
	deliverySessionKey = strings.TrimSpace(deliverySessionKey)
	if !externalIMSessionKey(deliverySessionKey) || s.delivery == nil || s.deliveryGrants == nil {
		return
	}
	ownerCtx := contextForJobOwner(ctx, job)
	if err := s.deliveryGrants.ValidateAutomationDeliveryGrant(
		ownerCtx,
		job.OwnerUserID,
		job.AgentID,
		deliverySessionKey,
	); err != nil {
		message := "定时任务 IM 通知授权已失效"
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			message = "定时任务 IM 通知校验未完成"
		}
		s.loggerFor(ctx).Warn(message,
			"job_id", job.JobID,
			"session_key", deliverySessionKey,
			"err", err,
		)
		return
	}
	_, err := s.delivery.DeliverMessage(ownerCtx, job.AgentID, automationIMHeader(job)+"\n"+strings.TrimSpace(body), channels.DeliveryTarget{
		Mode:       channels.DeliveryModeLast,
		SessionKey: deliverySessionKey,
	})
	if err != nil {
		s.loggerFor(ctx).Warn("定时任务 IM 通知投递失败",
			"job_id", job.JobID,
			"session_key", deliverySessionKey,
			"err", err,
		)
	}
}
