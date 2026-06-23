package sqlite

import (
	"database/sql"

	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

// AgentRepository 提供 SQLite 的 Agent 仓储实现。
type AgentRepository = agentrepo.SQLRepository

// NewAgentRepository 创建 Agent 仓储。
func NewAgentRepository(db *sql.DB) *AgentRepository {
	return agentrepo.NewSQLRepository("sqlite", db)
}
