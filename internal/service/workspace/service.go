package workspace

import (
	"context"
	"errors"
	"hash/fnv"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/config"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
)

var (
	// ErrFileNotFound 表示 workspace 文件不存在。
	ErrFileNotFound = errors.New("workspace file not found")

	// ErrFileRevisionConflict 表示条件写入基线已经过期，本次内容没有落盘。
	ErrFileRevisionConflict = errors.New("workspace file revision conflict")

	// ErrFileTooLarge 表示文件不能安全地作为单个正文载荷读取。
	ErrFileTooLarge = errors.New("workspace file is too large for whole-content access")

	// ErrMutationInvalid 表示修改在任何 workspace 文件落盘前因参数或受保护目标被拒绝。
	ErrMutationInvalid = errors.New("workspace mutation invalid")

	// ErrLocalFileRevealUnavailable 表示当前运行模式不支持本机文件定位。
	ErrLocalFileRevealUnavailable = errors.New("workspace local file reveal unavailable")
)

type workspaceMutationInvalidError struct {
	detail string
}

func (e workspaceMutationInvalidError) Error() string {
	return e.detail
}

func (e workspaceMutationInvalidError) Unwrap() error {
	return ErrMutationInvalid
}

func invalidWorkspaceMutation(err error) error {
	if err == nil || errors.Is(err, ErrMutationInvalid) {
		return err
	}
	return workspaceMutationInvalidError{detail: err.Error()}
}

// Service 提供 workspace 文件读写能力。
type Service struct {
	config            config.Config
	agents            *agentsvc.Service
	live              *liveManager
	fileMutationLocks [workspaceFileMutationLockShards]sync.Mutex
}

const workspaceFileMutationLockShards = 64

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

// lockFileMutation 只串行化同一分片内极短的 API 文件提交窗口。
// 固定分片避免为用户可控路径保留无界锁表，也不会改变 Agent/runtime 身份。
func (s *Service) lockFileMutation(agentID string, relativePath string) func() {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(strings.TrimSpace(agentID)))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(relativePath))
	lock := &s.fileMutationLocks[hash.Sum32()%workspaceFileMutationLockShards]
	lock.Lock()
	return lock.Unlock
}
