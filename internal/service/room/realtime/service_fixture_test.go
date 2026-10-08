package realtime

import (
	"testing"

	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// withConstructorDefaults 为直接构造的 Service 字面量补齐 NewService 总会创建的依赖。
// 生产路径不再为这些依赖做 nil 守卫，测试夹具必须满足同一不变量。
func withConstructorDefaults(t *testing.T, s *Service) *Service {
	t.Helper()
	root := s.config.WorkspacePath
	if root == "" {
		root = t.TempDir()
	}
	if s.files == nil {
		s.files = workspacestore.NewSessionFileStore(root)
	}
	if s.history == nil {
		s.history = workspacestore.NewAgentHistoryStore(root)
	}
	if s.roomHistory == nil {
		s.roomHistory = workspacestore.NewRoomHistoryStore(root)
	}
	if s.directedMessages == nil {
		s.directedMessages = workspacestore.NewRoomDirectedMessageStore(root)
	}
	if s.directedWakes == nil {
		s.directedWakes = workspacestore.NewRoomDirectedMessageWakeStore(root)
	}
	if s.publicHandoffs == nil {
		s.publicHandoffs = workspacestore.NewRoomPublicHandoffStore(root)
	}
	if s.inputQueue == nil {
		s.inputQueue = workspacestore.NewInputQueueStore(root)
	}
	return s
}
