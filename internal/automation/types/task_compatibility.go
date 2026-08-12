// INPUT: 从历史数据库或旧客户端读取的 ScheduledTask 快照。
// OUTPUT: 不丢失用户配置、符合当前投递与权限模型的兼容任务视图。
// POS: Automation 旧任务线格式的唯一兼容归一化入口；数据库迁移与运行时读取语义同构。
package types

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// NormalizeScheduledTaskCompatibility 把历史任务映射到当前模型。
//
// 旧版本曾把外部 IM 的结构化 session_key 作为 websocket 显式目标保存。
// 当前模型把这类目标表达为 last + session_key，由宿主在每次投递前重新验证
// active pairing。这里只转换能够从结构化 key 精确恢复的配置，不能精确判断的
// 显式平台目标继续保留旧语义，避免猜错账号或收件人。
func NormalizeScheduledTaskCompatibility(task ScheduledTask) ScheduledTask {
	task.ExecutionKind = NormalizeExecutionKind(task.ExecutionKind)
	task.PermissionMode = NormalizePermissionMode(task.PermissionMode)
	task.SessionTarget = task.SessionTarget.Normalized()
	if strings.TrimSpace(task.DeliveryGrant.Kind) == "" {
		// migration 96 会持久复制；这里同时为内存构造、部分旧 schema 与
		// migration 前的恢复路径提供同构映射。
		task.DeliveryGrant = task.Source
	}
	task.DeliveryGrant = task.DeliveryGrant.Normalized()
	task.Source = task.Source.Normalized()
	task.Delivery = normalizeCompatibleDelivery(task.Delivery, task.Source)
	task.OverlapPolicy = NormalizeOverlapPolicy(task.OverlapPolicy)
	return NormalizeScheduledTaskSessionBinding(task)
}

func normalizeCompatibleDelivery(delivery DeliveryTarget, source Source) DeliveryTarget {
	delivery = delivery.Normalized()
	if delivery.SessionKey == "" {
		switch {
		case delivery.Mode == DeliveryModeExplicit && isStructuredAutomationSessionKey(delivery.To):
			delivery.SessionKey = delivery.To
		case delivery.Mode == DeliveryModeLast && isStructuredAutomationSessionKey(source.SessionKey):
			delivery.SessionKey = strings.TrimSpace(source.SessionKey)
		}
	}

	if delivery.Mode == DeliveryModeExplicit &&
		strings.TrimSpace(delivery.To) == delivery.SessionKey &&
		isLegacyExternalSessionEnvelope(delivery) {
		return DeliveryTarget{
			Mode:       DeliveryModeLast,
			SessionKey: delivery.SessionKey,
		}
	}
	return delivery
}

func isStructuredAutomationSessionKey(value string) bool {
	parsed := protocol.ParseSessionKey(value)
	return parsed.IsStructured &&
		(parsed.Kind == protocol.SessionKeyKindAgent || parsed.Kind == protocol.SessionKeyKindRoom)
}

func isLegacyExternalSessionEnvelope(delivery DeliveryTarget) bool {
	parsed := protocol.ParseSessionKey(delivery.SessionKey)
	if !parsed.IsStructured || parsed.Kind != protocol.SessionKeyKindAgent {
		return false
	}
	channel := protocol.NormalizeStoredChannelType(parsed.Channel)
	if channel == protocol.SessionChannelWebSocket || channel == protocol.SessionChannelInternalSegment {
		return false
	}
	legacyEnvelopeChannel := protocol.NormalizeStoredChannelType(delivery.Channel)
	return legacyEnvelopeChannel == "" ||
		legacyEnvelopeChannel == protocol.SessionChannelWebSocket ||
		legacyEnvelopeChannel == protocol.SessionChannelInternalSegment
}
