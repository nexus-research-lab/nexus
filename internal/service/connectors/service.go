package connectors

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// Service 提供连接器目录、授权与状态能力。
type Service struct {
	config     config.Config
	db         *sql.DB
	driver     string
	httpClient *http.Client
}

// NewService 创建连接器服务。
func NewService(cfg config.Config, db *sql.DB) *Service {
	driver := storage.NormalizeSQLDriver(cfg.DatabaseDriver)
	return &Service{
		config: cfg,
		db:     db,
		driver: driver,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}
