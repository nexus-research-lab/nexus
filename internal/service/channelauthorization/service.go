// INPUT: configuration authority verifier, Channel login control, DB repository, and human presenter.
// OUTPUT: owner-main-only persistent authorization service with restart-safe invalidation.
// POS: conversational Channel authorization dependency root.
package channelauthorization

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	channelssvc "github.com/nexus-research-lab/nexus/internal/service/channels"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
	authorizationstore "github.com/nexus-research-lab/nexus/internal/storage/channelauthorization"
)

const (
	defaultFlowTTL         = 8 * time.Minute
	defaultMonitorInterval = 500 * time.Millisecond
)

type authorityVerifier interface {
	Inspect(context.Context, configurationsvc.Actor, []string, bool) (*configurationsvc.Inspection, error)
}

type interactiveHumanVerifier interface {
	VerifyInteractiveHuman(context.Context, *authctx.Principal) (*authctx.Principal, error)
	VerifyBoundInteractiveHuman(context.Context, string, string, string) (*authctx.Principal, error)
	AcquireBoundInteractiveHumanLease(context.Context, string, string, string) (*authctx.Principal, func(), error)
}

type channelLoginControl interface {
	GetChannelControlVersion(context.Context, string) (int64, error)
	StartChannelLoginForAuthorizationAtVersion(context.Context, string, string, string, string, int64) (*channelssvc.ChannelLoginView, error)
	GetChannelLogin(context.Context, string, string, string) (*channelssvc.ChannelLoginView, error)
	CancelChannelLogin(context.Context, string, string, string) (*channelssvc.ChannelLoginView, error)
	CancelChannelLoginAndWait(context.Context, string, string, string) (*channelssvc.ChannelLoginView, error)
	SubmitChannelLoginVerifyCode(context.Context, string, string, string, channelssvc.SubmitChannelLoginVerifyCodeRequest) (*channelssvc.ChannelLoginView, error)
}

// HumanPresenter is a native, authenticated UI boundary. Implementations must
// route by the fixed session metadata and must not feed payloads back to a model.
type HumanPresenter interface {
	PresentChannelAuthorization(context.Context, HumanPresentation) error
}

type Service struct {
	repository        *authorizationstore.Repository
	authority         authorityVerifier
	humanVerifier     interactiveHumanVerifier
	channels          channelLoginControl
	presenter         HumanPresenter
	keyring           *credentials.Keyring
	keyErr            error
	processGeneration string
	now               func() time.Time
	flowTTL           time.Duration
	monitorInterval   time.Duration
	idFactory         func(string) (string, error)

	initializeMu  sync.Mutex
	initialized   bool
	synchronizeMu sync.Mutex
	lifecycleMu   sync.RWMutex
	closing       bool
	closeOnce     sync.Once
	closeErr      error
	monitorMu     sync.Mutex
	monitors      map[string]context.CancelFunc
	monitorWG     sync.WaitGroup
}

func NewService(
	cfg config.Config,
	db *sql.DB,
	authority authorityVerifier,
	humanVerifier interactiveHumanVerifier,
	channels channelLoginControl,
	presenter HumanPresenter,
) *Service {
	keyring, keyErr := credentials.NewKeyring(
		cfg.ConnectorCredentialsKey,
		cfg.ConnectorCredentialsLegacyKeys,
	)
	processGeneration, generationErr := newOpaqueID("process")
	if keyErr == nil && generationErr != nil {
		keyErr = generationErr
	}
	return &Service{
		repository:        authorizationstore.NewRepository(cfg, db),
		authority:         authority,
		humanVerifier:     humanVerifier,
		channels:          channels,
		presenter:         presenter,
		keyring:           keyring,
		keyErr:            keyErr,
		processGeneration: processGeneration,
		now:               func() time.Time { return time.Now().UTC() },
		flowTTL:           defaultFlowTTL,
		monitorInterval:   defaultMonitorInterval,
		idFactory:         newOpaqueID,
		monitors:          make(map[string]context.CancelFunc),
	}
}

func (s *Service) beginOperation() (func(), error) {
	if s == nil {
		return nil, errors.New("channel authorization service is not configured")
	}
	s.lifecycleMu.RLock()
	if s.closing {
		s.lifecycleMu.RUnlock()
		return nil, errors.New("Channel 授权服务正在关闭")
	}
	return s.lifecycleMu.RUnlock, nil
}

// beginCommitOperation excludes every flow mutation while one bound Channel
// login persists credentials and publishes its candidate runtime. In
// particular, expiry/cancel/failure reconciliation cannot terminalize the
// durable flow after validation but before the Channel commit finishes.
func (s *Service) beginCommitOperation() (func(), error) {
	if s == nil {
		return nil, errors.New("channel authorization service is not configured")
	}
	s.lifecycleMu.Lock()
	if s.closing {
		s.lifecycleMu.Unlock()
		return nil, errors.New("Channel 授权服务正在关闭")
	}
	return s.lifecycleMu.Unlock, nil
}

func (s *Service) SetHumanPresenter(presenter HumanPresenter) {
	if s == nil {
		return
	}
	s.presenter = presenter
}

// Initialize explicitly invalidates active flows from an older process. Public
// methods call it lazily too, so a missed app hook cannot resurrect old tokens.
func (s *Service) Initialize(ctx context.Context) error {
	if s == nil {
		return errors.New("channel authorization service is not configured")
	}
	s.initializeMu.Lock()
	defer s.initializeMu.Unlock()
	if s.initialized {
		return nil
	}
	if s.keyErr != nil {
		return fmt.Errorf("初始化 Channel 授权加密: %w", s.keyErr)
	}
	_, err := s.repository.InvalidateStale(
		ctx,
		s.processGeneration,
		s.now(),
		func() (string, error) { return s.idFactory("channel_authorization_audit") },
	)
	if err != nil {
		return fmt.Errorf("使旧 Channel 授权安全失效: %w", err)
	}
	s.initialized = true
	return nil
}

func newOpaqueID(prefix string) (string, error) {
	buffer := make([]byte, 24)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return strings.TrimSpace(prefix) + "_" + base64.RawURLEncoding.EncodeToString(buffer), nil
}
