package adapters

import (
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
)

// FeishuCallbackSecurity 表示飞书事件订阅回调安全配置。
type FeishuCallbackSecurity struct {
	VerificationToken string
	EncryptKey        string
}

// FeishuIngressPreparation 表示通过通道配置校验后的飞书回调明文。
type FeishuIngressPreparation struct {
	Body        []byte
	OwnerUserID string
	AppID       string
}

// FeishuIngressCallback 表示飞书回调解析后的入站结果。
type FeishuIngressCallback struct {
	Challenge     string
	AppID         string
	Token         string
	Request       *channelcontract.IngressRequest
	IgnoredReason string
}

type feishuEventCallbackPayload struct {
	Challenge string             `json:"challenge"`
	Token     string             `json:"token"`
	Type      string             `json:"type"`
	Header    feishuEventHeader  `json:"header"`
	Event     feishuEventPayload `json:"event"`
}

type feishuEventHeader struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	AppID     string `json:"app_id"`
	Token     string `json:"token"`
}

type feishuEventPayload struct {
	AppID        string              `json:"app_id"`
	Sender       feishuEventSender   `json:"sender"`
	Message      feishuEventMessage  `json:"message"`
	MessageID    string              `json:"message_id"`
	ChatID       string              `json:"chat_id"`
	ChatType     string              `json:"chat_type"`
	ThreadID     string              `json:"thread_id"`
	RootID       string              `json:"root_id"`
	ParentID     string              `json:"parent_id"`
	ReactionType feishuReactionType  `json:"reaction_type"`
	OperatorType string              `json:"operator_type"`
	UserID       feishuEventSenderID `json:"user_id"`
	ActionTime   string              `json:"action_time"`
}

type feishuEventSender struct {
	SenderType string              `json:"sender_type"`
	SenderID   feishuEventSenderID `json:"sender_id"`
}

type feishuEventSenderID struct {
	OpenID  string `json:"open_id"`
	UserID  string `json:"user_id"`
	UnionID string `json:"union_id"`
}

type feishuEventMessage struct {
	MessageID   string `json:"message_id"`
	RootID      string `json:"root_id"`
	ParentID    string `json:"parent_id"`
	ThreadID    string `json:"thread_id"`
	ChatID      string `json:"chat_id"`
	ChatType    string `json:"chat_type"`
	MessageType string `json:"message_type"`
	Content     string `json:"content"`
	CreateTime  string `json:"create_time"`
}

type feishuReactionType struct {
	EmojiType string `json:"emoji_type"`
}
