package channels

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

type ControlService struct {
	config                   config.Config
	db                       *sql.DB
	driver                   string
	key                      []byte
	agents                   agentWorkspaceResolver
	router                   *Router
	httpClient               *http.Client
	idFactory                func(string) string
	loginStore               *channelLoginStore
	loginTimeout             time.Duration
	weixinLoginClientFactory func(string, map[string]string) personalWeixinLoginClient
	keyErr                   error
}

func NewControlService(
	cfg config.Config,
	db *sql.DB,
	agents agentWorkspaceResolver,
	router *Router,
) *ControlService {
	key, err := credentials.DecodeKey(cfg.ConnectorCredentialsKey)
	return &ControlService{
		config:       cfg,
		db:           db,
		driver:       storage.NormalizeSQLDriver(cfg.DatabaseDriver),
		key:          key,
		agents:       agents,
		router:       router,
		idFactory:    newDeliveryID,
		loginStore:   newChannelLoginStore(),
		loginTimeout: 8 * time.Minute,
		keyErr:       err,
	}
}

// SetHTTPClient 注入 IM 通道主动投递使用的 HTTP client，主要用于测试或统一出站链路配置。
func (s *ControlService) SetHTTPClient(client *http.Client) {
	s.httpClient = client
}
