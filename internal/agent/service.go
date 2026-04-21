// =====================================================
// @File   ：service.go
// @Date   ：2026/04/16 13:44:49
// @Author ：leemysw
// 2026/04/16 13:44:49   Create
// =====================================================

package agent

import (
	"errors"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/config"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

var (
	// ErrAgentNotFound 表示 Agent 不存在。
	ErrAgentNotFound = errors.New("agent not found")
)

// Service 提供 Agent 业务能力。
type Service struct {
	config     config.Config
	repository Repository
	history    *workspacestore.AgentHistoryStore
	prompts    *promptBuilder
	readyMu    sync.Mutex
}

// NewService 创建 Agent 服务。
func NewService(cfg config.Config, repository Repository) *Service {
	return &Service{
		config:     cfg,
		repository: repository,
		history:    workspacestore.NewAgentHistoryStore(cfg.WorkspacePath),
		prompts:    newPromptBuilder(cfg),
	}
}
