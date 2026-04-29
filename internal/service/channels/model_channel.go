package channels

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
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
	result.Channel = normalizeChannelType(result.Channel)
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

// MessageChannel 定义通道生命周期。
type MessageChannel interface {
	ChannelType() string
	Start(context.Context) error
	Stop(context.Context) error
}

// DeliveryChannel 定义统一文本投递能力。
type DeliveryChannel interface {
	MessageChannel
	SendDeliveryText(context.Context, DeliveryTarget, string) error
}

func normalizeChannelType(channel string) string {
	return protocol.NormalizeStoredChannelType(channel)
}
