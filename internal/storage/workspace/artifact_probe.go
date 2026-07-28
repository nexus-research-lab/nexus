// INPUT: Room owner + conversation ID，或 owner-bound Agent workspace + session key。
// OUTPUT: 受 confined filesystem 约束的持久化目录存在性。
// POS: 空会话维护使用的只读文件证据探测边界。
package workspace

import (
	"errors"
	"os"
)

// RoomConversationArtifactsExist 判断 Room conversation 的共享持久化目录是否存在。
//
// 目录即证据：即使目录内只有附件、directed message 或未完整写入的历史，
// 维护命令也必须把该 conversation 视为已经使用，不能自动删除。
func (s *SessionFileStore) RoomConversationArtifactsExist(ownerUserID string, conversationID string) (bool, error) {
	root, err := s.openRoomRoot(ownerUserID, false)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer root.Close()

	if _, err = root.Lstat(encodeConversationDirName(conversationID)); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// SessionArtifactsExist 判断指定 Agent session 的持久化目录是否存在。
//
// 不只检查 meta.json；overlay 或 input queue 可以先于 meta 出现。
func (s *SessionFileStore) SessionArtifactsExist(workspacePath string, sessionKey string) (bool, error) {
	root, err := s.openWorkspaceRoot(workspacePath, false)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer root.Close()

	sessionsRoot, err := root.OpenRootNoSymlink(".agents/sessions")
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer sessionsRoot.Close()

	if _, err = sessionsRoot.Lstat(encodeSessionDirName(sessionKey)); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}
