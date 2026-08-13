// INPUT: 调用 Agent、由当前可信会话签发的投递 grant 与标准化 DeliveryTarget。
// OUTPUT: 普通 Agent 只能投递到自身会话或 grant 精确绑定的当前 Room/外部会话。
// POS: Automation 对话写入与后台实际投递共用的最小目标权限规则。
package types

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ValidateSelfScopedDeliveryTarget 验证普通 Agent 的投递目标。
//
// 自身 Agent session 可用于当前 DM 或已明确选择的真实会话。共享 Room 与
// 外部 Channel 必须精确匹配创建/最近一次对话修改时保存的可信 source session；
// 因而仅知道另一个 Agent、群或外部收件人的 ID 不能扩大投递范围。
func ValidateSelfScopedDeliveryTarget(
	agentID string,
	grantedSessionKey string,
	target DeliveryTarget,
) error {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return errors.New("self-scoped automation delivery requires an agent_id")
	}
	target = target.Normalized()
	switch target.Mode {
	case DeliveryModeNone:
		return nil
	case DeliveryModeLast:
		return validateSelfScopedRouteSession(agentID, grantedSessionKey, target.SessionKey)
	case DeliveryModeExplicit:
	default:
		return errors.New("self-scoped automation delivery requires none, last, or an explicit target")
	}

	channel := protocol.NormalizeStoredChannelType(target.Channel)
	switch channel {
	case protocol.SessionChannelWebSocket, protocol.SessionChannelInternalSegment:
		return validateSelfSessionDelivery(agentID, grantedSessionKey, target)
	case protocol.SessionChannelDiscord,
		protocol.SessionChannelTelegram,
		protocol.SessionChannelDingTalk,
		protocol.SessionChannelWeChat,
		protocol.SessionChannelWeixinPersonal,
		protocol.SessionChannelFeishu:
		return validateGrantedExternalDelivery(agentID, grantedSessionKey, target, channel)
	default:
		return fmt.Errorf("self-scoped automation cannot deliver to channel %q", target.Channel)
	}
}

func validateSelfScopedRouteSession(
	agentID string,
	grantedSessionKey string,
	targetSessionKey string,
) error {
	targetSessionKey = strings.TrimSpace(targetSessionKey)
	parsed := protocol.ParseSessionKey(targetSessionKey)
	if !parsed.IsStructured {
		return errors.New("self-scoped last delivery requires a structured session_key")
	}
	switch parsed.Kind {
	case protocol.SessionKeyKindRoom:
		if targetSessionKey != strings.TrimSpace(grantedSessionKey) {
			return errors.New("Room automation delivery is limited to the current granted Room conversation")
		}
		return nil
	case protocol.SessionKeyKindAgent:
		if strings.TrimSpace(parsed.AgentID) != agentID {
			return fmt.Errorf(
				"agent %s cannot deliver automation output to another agent %s",
				agentID,
				strings.TrimSpace(parsed.AgentID),
			)
		}
		channel := protocol.NormalizeStoredChannelType(parsed.Channel)
		if channel == protocol.SessionChannelWebSocket || channel == protocol.SessionChannelInternalSegment {
			return nil
		}
		if targetSessionKey != strings.TrimSpace(grantedSessionKey) {
			return errors.New("external automation delivery is limited to the current explicitly granted conversation")
		}
		return nil
	default:
		return errors.New("last delivery session_key must identify an Agent or Room conversation")
	}
}

func validateSelfSessionDelivery(
	agentID string,
	grantedSessionKey string,
	target DeliveryTarget,
) error {
	parsed := protocol.ParseSessionKey(target.To)
	if !parsed.IsStructured {
		return errors.New("self-scoped automation delivery requires a structured session target")
	}
	switch parsed.Kind {
	case protocol.SessionKeyKindAgent:
		if strings.TrimSpace(parsed.AgentID) != agentID {
			return fmt.Errorf(
				"agent %s cannot deliver automation output to another agent %s",
				agentID,
				strings.TrimSpace(parsed.AgentID),
			)
		}
		return nil
	case protocol.SessionKeyKindRoom:
		if strings.TrimSpace(target.To) != strings.TrimSpace(grantedSessionKey) {
			return errors.New("Room automation delivery is limited to the current granted Room conversation")
		}
		granted := protocol.ParseSessionKey(grantedSessionKey)
		if !granted.IsStructured || granted.Kind != protocol.SessionKeyKindRoom {
			return errors.New("Room automation delivery is missing a trusted current-conversation grant")
		}
		return nil
	default:
		return errors.New("self-scoped automation delivery requires an Agent or current Room session")
	}
}

func validateGrantedExternalDelivery(
	agentID string,
	grantedSessionKey string,
	target DeliveryTarget,
	channel string,
) error {
	granted := protocol.ParseSessionKey(grantedSessionKey)
	if !granted.IsStructured ||
		granted.Kind != protocol.SessionKeyKindAgent ||
		strings.TrimSpace(granted.AgentID) != agentID ||
		protocol.NormalizeStoredChannelType(granted.Channel) != channel ||
		strings.TrimSpace(granted.Ref) != strings.TrimSpace(target.To) ||
		strings.TrimSpace(granted.ThreadID) != strings.TrimSpace(target.ThreadID) ||
		strings.TrimSpace(granted.AccountID) != strings.TrimSpace(target.AccountID) {
		return errors.New("external automation delivery is limited to the current explicitly granted conversation")
	}
	return nil
}
