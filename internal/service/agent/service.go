package agent

import (
	"context"
	"errors"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/config"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type goalCleaner interface {
	DeleteGoalsForAgent(context.Context, string) (int, error)
}

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
	goals      goalCleaner
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

// SetGoalCleaner 注入 Agent 删除时的 Goal 级联清理器。
func (s *Service) SetGoalCleaner(cleaner goalCleaner) {
	s.goals = cleaner
}
