// INPUT: run 开始时冻结的 DeliveryTarget 与持久 AutomationPermissionRequest。
// OUTPUT: 唯一冻结的权限审批 SessionKey；优先结果接收 Session，其次任务来源 Session。
// POS: Automation 权限看板、Nexus Session 与 IM transport 共用的接收路由真相。
package automation

import (
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

func automationPermissionApprovalSessionKey(
	target automationdomain.DeliveryTarget,
	source automationdomain.Source,
) string {
	target = target.Normalized()
	if target.Mode != automationdomain.DeliveryModeNone {
		if sessionKey := strings.TrimSpace(target.SessionKey); sessionKey != "" {
			return sessionKey
		}
	}
	return strings.TrimSpace(source.SessionKey)
}

func automationPermissionRunRecipientSessionKey(
	job automationdomain.ScheduledTask,
	run automationdomain.ScheduledTaskRun,
) string {
	return automationPermissionApprovalSessionKey(deliveryTargetForRun(job, run), job.Source)
}

func automationPermissionRequestRecipientSessionKey(
	request automationdomain.AutomationPermissionRequest,
) string {
	return strings.TrimSpace(request.DeliverySessionKey)
}
