package contract

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
)

const (
	// DeliveryModeNone 表示不做外部投递。
	DeliveryModeNone = "none"
	// DeliveryModeLast 表示投递到最近一次成功目标。
	DeliveryModeLast = "last"
	// DeliveryModeExplicit 表示投递到显式目标。
	DeliveryModeExplicit = "explicit"

	// ChannelTypeWebSocket 表示浏览器 WebSocket 面板。
	ChannelTypeWebSocket = "websocket"
	// ChannelTypeDiscord 表示 Discord 通道。
	ChannelTypeDiscord = "discord"
	// ChannelTypeTelegram 表示 Telegram 通道。
	ChannelTypeTelegram = "telegram"
	// ChannelTypeDingTalk 表示钉钉通道。
	ChannelTypeDingTalk = "dingtalk"
	// ChannelTypeWeChat 表示微信通道。
	ChannelTypeWeChat = "wechat"
	// ChannelTypeWeixinPersonal 表示内置个人微信 iLink 通道。
	ChannelTypeWeixinPersonal = protocol.SessionChannelWeixinPersonal
	// ChannelTypeFeishu 表示飞书通道。
	ChannelTypeFeishu = "feishu"
	// ChannelTypeInternal 表示内部系统会话。
	ChannelTypeInternal = "internal"
)

// DeliveryTarget 表示通道无关的投递目标。
type DeliveryTarget struct {
	Mode       string `json:"mode"`
	Channel    string `json:"channel,omitempty"`
	To         string `json:"to,omitempty"`
	AccountID  string `json:"account_id,omitempty"`
	ThreadID   string `json:"thread_id,omitempty"`
	SessionKey string `json:"session_key,omitempty"`
}

// Normalized 返回带默认值的副本。
func (t DeliveryTarget) Normalized() DeliveryTarget {
	result := t
	result.Mode = strings.TrimSpace(result.Mode)
	if result.Mode == "" {
		result.Mode = DeliveryModeNone
	}
	result.Channel = NormalizeChannelType(result.Channel)
	result.To = strings.TrimSpace(result.To)
	result.AccountID = strings.TrimSpace(result.AccountID)
	result.ThreadID = strings.TrimSpace(result.ThreadID)
	result.SessionKey = strings.TrimSpace(result.SessionKey)
	if (result.Channel == ChannelTypeWebSocket || result.Channel == ChannelTypeInternal) && result.SessionKey == "" {
		result.SessionKey = result.To
	}
	if result.To == "" && result.SessionKey != "" {
		result.To = result.SessionKey
	}
	return result
}

// Validate 校验目标是否合法。
func (t DeliveryTarget) Validate() error {
	normalized := t.Normalized()
	switch normalized.Mode {
	case DeliveryModeNone, DeliveryModeLast:
		return nil
	case DeliveryModeExplicit:
	default:
		return errors.New("delivery.mode must be one of none, last, explicit")
	}

	if normalized.Channel == "" {
		return errors.New("delivery target requires channel")
	}
	if normalized.To == "" {
		return errors.New("delivery target requires to")
	}
	if (normalized.Channel == ChannelTypeWebSocket || normalized.Channel == ChannelTypeInternal) && normalized.SessionKey == "" {
		return errors.New("delivery target requires session_key")
	}
	return nil
}

// DeliveryResult 表示一次通道投递的目标解析结果与平台回执。
type DeliveryResult struct {
	Target  DeliveryTarget          `json:"target"`
	Receipt *channelmessage.Receipt `json:"receipt,omitempty"`
}

// NewDeliveryResult 返回规范化后的投递结果。
func NewDeliveryResult(target DeliveryTarget, receipt *channelmessage.Receipt) DeliveryResult {
	return DeliveryResult{
		Target:  target.Normalized(),
		Receipt: receipt,
	}
}

// MessageChannel 定义通道生命周期。
type MessageChannel interface {
	ChannelType() string
	Start(context.Context) error
	Stop(context.Context) error
}

// DeliveryChannel 定义统一文本投递能力。
type DeliveryChannel interface {
	MessageChannel
	SendDeliveryMessage(context.Context, DeliveryTarget, string) (DeliveryResult, error)
}

// AgentScopedDeliveryChannel 表示需要按 agent 维度投递的内部通道。
type AgentScopedDeliveryChannel interface {
	SendAgentDeliveryMessage(context.Context, string, DeliveryTarget, string) (DeliveryResult, error)
}

// TypingDeliveryChannel 表示支持 typing 状态投递的通道。
type TypingDeliveryChannel interface {
	SendDeliveryTyping(context.Context, DeliveryTarget, bool) error
}

// IngressRequest 表示一条来自外部通道的标准化消息。
type IngressRequest struct {
	Channel          string                  `json:"channel,omitempty"`
	OwnerUserID      string                  `json:"owner_user_id,omitempty"`
	AccountID        string                  `json:"account_id,omitempty"`
	SessionKey       string                  `json:"session_key,omitempty"`
	AgentID          string                  `json:"agent_id,omitempty"`
	ChatType         string                  `json:"chat_type,omitempty"`
	Ref              string                  `json:"ref,omitempty"`
	ThreadID         string                  `json:"thread_id,omitempty"`
	ExternalName     string                  `json:"external_name,omitempty"`
	Content          string                  `json:"content"`
	RoundID          string                  `json:"round_id,omitempty"`
	ReqID            string                  `json:"req_id,omitempty"`
	PermissionMode   string                  `json:"permission_mode,omitempty"`
	AutoApproveAll   bool                    `json:"auto_approve_all,omitempty"`
	AutoApproveTools []string                `json:"auto_approve_tools,omitempty"`
	Delivery         *DeliveryTarget         `json:"delivery,omitempty"`
	Message          *channelmessage.Inbound `json:"message,omitempty"`
}

// IngressResult 描述入口受理结果。
type IngressResult struct {
	Channel            string                  `json:"channel"`
	AgentID            string                  `json:"agent_id"`
	SessionKey         string                  `json:"session_key"`
	RoundID            string                  `json:"round_id"`
	ReqID              string                  `json:"req_id"`
	Duplicate          bool                    `json:"duplicate,omitempty"`
	RememberedDelivery *DeliveryTarget         `json:"remembered_delivery,omitempty"`
	Message            *channelmessage.Inbound `json:"message,omitempty"`
}

// IngressAcceptor 表示通道入站消息的统一受理器。
type IngressAcceptor interface {
	Accept(context.Context, IngressRequest) (*IngressResult, error)
}

// IngressAwareChannel 表示可注入统一入站受理器的通道。
type IngressAwareChannel interface {
	SetIngress(IngressAcceptor)
}

// NormalizeChannelType 统一持久化的通道类型命名。
func NormalizeChannelType(channel string) string {
	return protocol.NormalizeStoredChannelType(channel)
}
