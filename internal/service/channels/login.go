package channels

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/connectors/appregistration"
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
	ErrChannelLoginNotFound            = errors.New("channel login not found")
	ErrChannelLoginUnsupported         = errors.New("channel login is not supported")
	ErrChannelLoginState               = errors.New("channel login state does not accept this operation")
	ErrChannelLoginAuthorizationCommit = errors.New("channel login authorization is no longer valid")
	ErrChannelRuntimeReload            = errors.New("channel runtime reload failed")
)

// ChannelLoginAuthorizationCommit identifies the exact conversational
// authorization whose lease must remain valid through credential persistence
// and candidate-runtime publication.
type ChannelLoginAuthorizationCommit struct {
	OwnerUserID          string
	ChannelType          string
	LoginID              string
	AuthorizationBinding string
	StartControlVersion  int64
}

// ChannelLoginAuthorizationCommitGuard is injected by the higher-level
// conversational authorization service. The returned release function keeps a
// successful validation leased until persistence and hot reload both finish.
type ChannelLoginAuthorizationCommitGuard interface {
	AcquireChannelLoginAuthorizationCommit(
		context.Context,
		ChannelLoginAuthorizationCommit,
	) (release func(), err error)
}

type ChannelLoginView struct {
	LoginID                 string     `json:"login_id"`
	ChannelType             string     `json:"channel_type"`
	Status                  string     `json:"status"`
	Command                 string     `json:"command,omitempty"`
	QRPayload               string     `json:"qr_payload,omitempty"`
	QRPayloadType           string     `json:"qr_payload_type,omitempty"`
	Output                  string     `json:"output,omitempty"`
	Error                   string     `json:"error,omitempty"`
	AccountID               string     `json:"account_id,omitempty"`
	UserID                  string     `json:"user_id,omitempty"`
	StartControlVersion     int64      `json:"start_control_version"`
	CommittedControlVersion int64      `json:"committed_control_version,omitempty"`
	StartedAt               time.Time  `json:"started_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
	ExpiresAt               time.Time  `json:"expires_at"`
	FinishedAt              *time.Time `json:"finished_at,omitempty"`
	VerifyCodeHint          string     `json:"verify_code_hint,omitempty"`
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
	mu                   sync.Mutex
	ownerUserID          string
	channelType          string
	activeKey            string
	expectedAccountID    string
	authorizationBinding string
	cancel               context.CancelFunc
	cancelRequested      bool
	committing           bool
	verifyCode           string
	verifyCh             chan struct{}
	client               personalWeixinLoginClient
	registrationClient   appregistration.Client
	deviceCode           string
	pollInterval         time.Duration
	qrcode               string
	view                 ChannelLoginView
	done                 chan struct{}
	doneOnce             sync.Once
}

func newChannelLoginStore() *channelLoginStore {
	return &channelLoginStore{
		active:   map[string]string{},
		sessions: map[string]*channelLoginSession{},
	}
}
