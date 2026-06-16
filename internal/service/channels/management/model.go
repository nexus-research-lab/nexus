package management

import (
	"errors"
	"strings"
	"time"

	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
)

const (
	ChannelConfigStatusConfigured = "configured"
	ChannelConfigStatusConnected  = "connected"
	ChannelConfigStatusPending    = "pending"
	ChannelConfigStatusError      = "error"
	ChannelConfigStatusDisabled   = "disabled"

	PairingStatusPending  = "pending"
	PairingStatusActive   = "active"
	PairingStatusDisabled = "disabled"
	PairingStatusRejected = "rejected"

	PairingSourceManual   = "manual"
	PairingSourceIngress  = "ingress"
	PairingSourceWeChatQR = "wechat_qr"
)

var (
	ErrChannelNotFound         = errors.New("channel not found")
	ErrChannelAccountNotFound  = errors.New("channel account not found")
	ErrPairingNotFound         = errors.New("pairing not found")
	ErrPairingApprovalRequired = errors.New("im pairing requires approval")
)

const PairingApprovalNoticeBody = "已收到你的消息，但当前 IM 用户或群聊尚未授权访问 Nexus 智能体。\n请管理员打开 Nexus 配对控制台（能力 → 配对），批准该配对后我就能继续回复。"

type PairingApprovalError struct {
	PairingID string
	Message   string
}

func (e *PairingApprovalError) Error() string {
	return e.Message
}

func (e *PairingApprovalError) Unwrap() error {
	return ErrPairingApprovalRequired
}

func IsPairingApprovalRequired(err error) bool {
	return errors.Is(err, ErrPairingApprovalRequired)
}

func PairingApprovalNoticeText(err error) string {
	if !IsPairingApprovalRequired(err) {
		return ""
	}
	var approval *PairingApprovalError
	if errors.As(err, &approval) && strings.TrimSpace(approval.PairingID) != "" {
		return PairingApprovalNoticeBody + "\n配对 ID：" + strings.TrimSpace(approval.PairingID)
	}
	return PairingApprovalNoticeBody
}

type ChannelCredentialField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Kind        string `json:"kind"`
	Required    bool   `json:"required"`
	Secret      bool   `json:"secret"`
	Placeholder string `json:"placeholder,omitempty"`
}

type ChannelCatalogItem struct {
	ChannelType       string                      `json:"channel_type"`
	Title             string                      `json:"title"`
	BotLabel          string                      `json:"bot_label"`
	Description       string                      `json:"description"`
	DocsURL           string                      `json:"docs_url,omitempty"`
	RuntimeStatus     string                      `json:"runtime_status"`
	RuntimeNote       string                      `json:"runtime_note,omitempty"`
	SupportsGroup     bool                        `json:"supports_group"`
	SupportsQRCode    bool                        `json:"supports_qr_code"`
	SupportsOAuthLink bool                        `json:"supports_oauth_link"`
	Capabilities      []channelmessage.Capability `json:"capabilities"`
	CredentialFields  []ChannelCredentialField    `json:"credential_fields"`
}

type ChannelStats struct {
	PairedUserCount  int `json:"paired_user_count"`
	PairedGroupCount int `json:"paired_group_count"`
	PendingCount     int `json:"pending_count"`
}

type ChannelAccountView struct {
	AccountID string    `json:"account_id"`
	UserID    string    `json:"user_id,omitempty"`
	Status    string    `json:"status"`
	LastError string    `json:"last_error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ChannelConfigView struct {
	ChannelCatalogItem
	Configured      bool                 `json:"configured"`
	ConnectionState string               `json:"connection_state"`
	Status          string               `json:"status"`
	AgentID         string               `json:"agent_id,omitempty"`
	AgentName       string               `json:"agent_name,omitempty"`
	PublicConfig    map[string]string    `json:"public_config,omitempty"`
	HasCredentials  bool                 `json:"has_credentials"`
	LastError       string               `json:"last_error,omitempty"`
	QRPayload       string               `json:"qr_payload,omitempty"`
	UpdatedAt       *time.Time           `json:"updated_at,omitempty"`
	Stats           ChannelStats         `json:"stats"`
	Accounts        []ChannelAccountView `json:"accounts,omitempty"`
}

type UpsertChannelConfigRequest struct {
	AgentID     string            `json:"agent_id"`
	Config      map[string]string `json:"config"`
	Credentials map[string]string `json:"credentials"`
}

type PairingView struct {
	PairingID     string     `json:"pairing_id"`
	ChannelType   string     `json:"channel_type"`
	AccountID     string     `json:"account_id,omitempty"`
	ChatType      string     `json:"chat_type"`
	ExternalRef   string     `json:"external_ref"`
	ThreadID      string     `json:"thread_id,omitempty"`
	SessionKey    string     `json:"session_key"`
	ExternalName  string     `json:"external_name,omitempty"`
	AgentID       string     `json:"agent_id"`
	AgentName     string     `json:"agent_name,omitempty"`
	Status        string     `json:"status"`
	Source        string     `json:"source"`
	LastMessageAt *time.Time `json:"last_message_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type PairingQuery struct {
	ChannelType string
	Status      string
	AgentID     string
}

type CreatePairingRequest struct {
	ChannelType  string `json:"channel_type"`
	AccountID    string `json:"account_id,omitempty"`
	ChatType     string `json:"chat_type"`
	ExternalRef  string `json:"external_ref"`
	ThreadID     string `json:"thread_id,omitempty"`
	ExternalName string `json:"external_name,omitempty"`
	AgentID      string `json:"agent_id"`
	Status       string `json:"status,omitempty"`
	Source       string `json:"source,omitempty"`
}

type UpdatePairingRequest struct {
	AgentID      *string `json:"agent_id,omitempty"`
	Status       *string `json:"status,omitempty"`
	ExternalName *string `json:"external_name,omitempty"`
}
