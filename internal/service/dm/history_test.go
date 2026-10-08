// 测试专用入口：生产路径已不再调用，仅供本包测试复用。
package dm

import "github.com/nexus-research-lab/nexus/internal/protocol"

func (s *Service) refreshSessionMetaAfterMessage(
	workspacePath string,
	current protocol.Session,
	message protocol.Message,
) (*protocol.Session, error) {
	return s.refreshSessionMetaAfterMessageForOwner("", workspacePath, current, message)
}

func (s *Service) refreshSessionMetaRuntimeState(
	workspacePath string,
	current protocol.Session,
) (*protocol.Session, error) {
	return s.refreshSessionMetaRuntimeStateForOwner("", workspacePath, current)
}
