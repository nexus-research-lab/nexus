package channels

import (
	"context"
	"errors"
	"sync"
	"time"

	channeladapters "github.com/nexus-research-lab/nexus/internal/service/channels/adapters"
)

const (
	ChannelLoginStatusRunning            = "running"
	ChannelLoginStatusVerifyCodeRequired = "verify_code_required"
	ChannelLoginStatusSucceeded          = "succeeded"
	ChannelLoginStatusError              = "error"
	ChannelLoginStatusExpired            = "expired"
	ChannelLoginStatusCancelled          = "cancelled"

	channelLoginOutputLimit = 64 * 1024
)

var (
	ErrChannelLoginNotFound    = errors.New("channel login not found")
	ErrChannelLoginUnsupported = errors.New("channel login is not supported")
)

type ChannelLoginView struct {
	LoginID        string     `json:"login_id"`
	ChannelType    string     `json:"channel_type"`
	Status         string     `json:"status"`
	Command        string     `json:"command,omitempty"`
	QRPayload      string     `json:"qr_payload,omitempty"`
	QRPayloadType  string     `json:"qr_payload_type,omitempty"`
	Output         string     `json:"output,omitempty"`
	Error          string     `json:"error,omitempty"`
	AccountID      string     `json:"account_id,omitempty"`
	UserID         string     `json:"user_id,omitempty"`
	StartedAt      time.Time  `json:"started_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	VerifyCodeHint string     `json:"verify_code_hint,omitempty"`
}

type SubmitChannelLoginVerifyCodeRequest struct {
	VerifyCode string `json:"verify_code"`
}

type personalWeixinLoginClient interface {
	StartQRCode(context.Context, []string) (channeladapters.PersonalWeixinQRCodeResponse, error)
	PollQRCodeStatus(context.Context, string, string) (channeladapters.PersonalWeixinQRStatusResponse, error)
}

type channelLoginStore struct {
	mu       sync.Mutex
	active   map[string]string
	sessions map[string]*channelLoginSession
}

type channelLoginSession struct {
	mu          sync.Mutex
	ownerUserID string
	channelType string
	activeKey   string
	cancel      context.CancelFunc
	verifyCode  string
	verifyCh    chan struct{}
	client      personalWeixinLoginClient
	qrcode      string
	view        ChannelLoginView
}

func newChannelLoginStore() *channelLoginStore {
	return &channelLoginStore{
		active:   map[string]string{},
		sessions: map[string]*channelLoginSession{},
	}
}
