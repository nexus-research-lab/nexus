package connectors

import (
	"database/sql"
	"net/http"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/connectors/appregistration"
	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// Service 提供连接器目录、授权与状态能力。
type Service struct {
	config                    config.Config
	db                        *sql.DB
	driver                    string
	httpClient                *http.Client
	registrationClientFactory func() appregistration.Client
	authorizationControl      *AuthorizationControl
	credentialKeyring         *credentials.Keyring
	credentialKeyringErr      error
	richMailBaseURL           string
	richMailMCPURL            string
	mutations                 sync.Map
}

// NewService 创建连接器服务。
func NewService(cfg config.Config, db *sql.DB) *Service {
	driver := storage.NormalizeSQLDriver(cfg.DatabaseDriver)
	credentialKeyring, credentialKeyringErr := credentials.NewKeyring(
		cfg.ConnectorCredentialsKey,
		cfg.ConnectorCredentialsLegacyKeys,
	)
	return &Service{
		config:               cfg,
		db:                   db,
		driver:               driver,
		credentialKeyring:    credentialKeyring,
		credentialKeyringErr: credentialKeyringErr,
		richMailBaseURL:      richMailDefaultBaseURL,
		richMailMCPURL:       richMailDefaultMCPURL,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}
