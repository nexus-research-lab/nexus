package workspace

import (
	"context"
	"errors"

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
	return &Service{
		config: cfg,
		agents: agents,
		live:   newLiveManager(),
	}
}

// SubscribeLive 订阅指定 Agent 的 workspace 实时事件。
func (s *Service) SubscribeLive(ctx context.Context, agentID string, listener LiveListener) (string, error) {
	agentValue, err := s.ensureAgentWorkspace(ctx, agentID)
	if err != nil {
		return "", err
	}
	return s.live.Subscribe(agentValue.AgentID, agentValue.WorkspacePath, listener)
}

// UnsubscribeLive 取消某个 workspace 实时订阅。
func (s *Service) UnsubscribeLive(token string) {
	if s.live == nil {
		return
	}
	s.live.Unsubscribe(token)
}

// FlushLiveWrites 立即结算指定 Agent 尚未发出结束事件的实时写入。
func (s *Service) FlushLiveWrites(agentID string) {
	if s.live == nil {
		return
	}
	s.live.FlushActiveWrites(agentID)
}
