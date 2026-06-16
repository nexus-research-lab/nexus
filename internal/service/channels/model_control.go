package channels

import (
	"database/sql"
	"time"

	channelmanagement "github.com/nexus-research-lab/nexus/internal/service/channels/management"
)

const (
	ChannelConfigStatusConfigured = channelmanagement.ChannelConfigStatusConfigured
	ChannelConfigStatusConnected  = channelmanagement.ChannelConfigStatusConnected
	ChannelConfigStatusPending    = channelmanagement.ChannelConfigStatusPending
	ChannelConfigStatusError      = channelmanagement.ChannelConfigStatusError
	ChannelConfigStatusDisabled   = channelmanagement.ChannelConfigStatusDisabled

	PairingStatusPending  = channelmanagement.PairingStatusPending
	PairingStatusActive   = channelmanagement.PairingStatusActive
	PairingStatusDisabled = channelmanagement.PairingStatusDisabled
	PairingStatusRejected = channelmanagement.PairingStatusRejected

	PairingSourceManual   = channelmanagement.PairingSourceManual
	PairingSourceIngress  = channelmanagement.PairingSourceIngress
	PairingSourceWeChatQR = channelmanagement.PairingSourceWeChatQR
)

var (
	ErrChannelNotFound         = channelmanagement.ErrChannelNotFound
	ErrChannelAccountNotFound  = channelmanagement.ErrChannelAccountNotFound
	ErrPairingNotFound         = channelmanagement.ErrPairingNotFound
	ErrPairingApprovalRequired = channelmanagement.ErrPairingApprovalRequired
)

type ChannelCredentialField = channelmanagement.ChannelCredentialField

type ChannelCatalogItem = channelmanagement.ChannelCatalogItem

type ChannelStats = channelmanagement.ChannelStats

type ChannelAccountView = channelmanagement.ChannelAccountView

type ChannelConfigView = channelmanagement.ChannelConfigView

type UpsertChannelConfigRequest = channelmanagement.UpsertChannelConfigRequest

type PairingView = channelmanagement.PairingView

type PairingQuery = channelmanagement.PairingQuery

type CreatePairingRequest = channelmanagement.CreatePairingRequest

type UpdatePairingRequest = channelmanagement.UpdatePairingRequest

type channelConfigRow struct {
	OwnerUserID          string
	ChannelType          string
	AgentID              string
	Status               string
	ConfigJSON           string
	CredentialsEncrypted sql.NullString
	LastError            sql.NullString
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type pairingRow struct {
	PairingID     string
	OwnerUserID   string
	ChannelType   string
	AccountID     string
	ChatType      string
	ExternalRef   string
	ThreadID      string
	ExternalName  sql.NullString
	AgentID       string
	Status        string
	Source        string
	LastMessageAt sql.NullTime
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type pairingApprovalError = channelmanagement.PairingApprovalError

type feishuIngressConfig struct {
	OwnerUserID       string
	AppID             string
	VerificationToken string
	EncryptKey        string
}

type sqlScanner interface {
	Scan(dest ...any) error
}
