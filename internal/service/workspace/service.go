package workspace

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
)

var (
	// ErrFileNotFound 表示 workspace 文件不存在。
	ErrFileNotFound = errors.New("workspace file not found")

	// ErrLocalFileRevealUnavailable 表示当前运行模式不支持本机文件定位。
	ErrLocalFileRevealUnavailable = errors.New("workspace local file reveal unavailable")
)

// Service 提供 workspace 文件读写能力。
type Service struct {
	config config.Config
	agents *agentsvc.Service
	live   *liveManager
}

// NewService 创建 workspace 服务。
func NewService(cfg config.Config, agents *agentsvc.Service) *Service {
	service := &Service{
		config: cfg,
		agents: agents,
		live:   newLiveManager(),
	}
	if agents != nil {
		agents.SetWorkspaceManager(service)
	}
	return service
}

// SubscribeLive 订阅指定 Agent 的 workspace 实时事件。
func (s *Service) SubscribeLive(ctx context.Context, agentID string, listener LiveListener) (string, error) {
	// 订阅只需要 owner 记录和现有目录，不得顺带改写模板、Skill 或 shim。
	agentValue, err := s.agents.GetAgent(ctx, strings.TrimSpace(agentID))
	if err != nil {
		return "", err
	}
	root, err := s.openAgentWorkspace(agentValue, false)
	if err != nil {
		return "", err
	}
	return s.live.Subscribe(
		agentValue.AgentID,
		agentValue.WorkspacePath,
		root,
		listener,
	)
}

// UnsubscribeLive 取消某个 workspace 实时订阅。
func (s *Service) UnsubscribeLive(token string) {
	if s.live == nil {
		return
	}
	s.live.Unsubscribe(token)
}
