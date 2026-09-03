package subscription

import (
	"database/sql"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

type AccountEntity struct {
	OwnerUserID       string
	Username          string
	DisplayName       string
	Role              string
	UserStatus        string
	PlanKey           string
	PlanName          string
	MonthlyTokenLimit *int64
	UsedTokens        int64
	SessionCount      int64
	MessageCount      int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type UsageEntity struct {
	ControlUserID string
	UsedTokens    int64
	SessionCount  int64
	MessageCount  int64
}

// Repository 只读取 Control 额度投影与 Nexus 本地用量。
type Repository struct {
	db      *sql.DB
	dialect storage.SQLDialect
}

func NewRepository(cfg config.Config, db *sql.DB) *Repository {
	return &Repository{db: db, dialect: storage.NewSQLDialect(cfg.DatabaseDriver)}
}
