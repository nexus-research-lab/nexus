package workspace

import (
	"errors"
	"path/filepath"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

// EnsureRoomConversationAssetDir 在 owner workspace 内创建 Room 公共资产目录。
//
// 目录创建从固定的 owner workspace 根进入 confinedfs；调用方不能用
// conversation id 把附件根重定向到另一用户或宿主 state。
func (s *Store) EnsureRoomConversationAssetDir(ownerUserID string, conversationID string) (string, error) {
	root, err := s.OpenRoomConversationAssetRoot(ownerUserID, conversationID, true)
	if err != nil {
		return "", err
	}
	defer root.Close()
	return s.RoomConversationAssetDir(ownerUserID, conversationID), nil
}

// OpenRoomConversationAssetRoot 打开 owner 绑定的 Room 公共附件根。
func (s *Store) OpenRoomConversationAssetRoot(
	ownerUserID string,
	conversationID string,
	create bool,
) (*confinedfs.Root, error) {
	if s == nil {
		return nil, errors.New("workspace storage root is nil")
	}
	workspaceRootPath := appfs.UserWorkspaceRootAt(s.StateRoot, ownerUserID)
	workspaceRoot, err := s.openOwnerWorkspaceRoot(ownerUserID, create)
	if err != nil {
		return nil, err
	}
	target := s.RoomConversationAssetDir(ownerUserID, conversationID)
	relative, err := filepath.Rel(workspaceRootPath, target)
	if err != nil {
		workspaceRoot.Close()
		return nil, err
	}
	relative = filepath.ToSlash(relative)
	if create {
		assetRoot, openErr := workspaceRoot.OpenOrCreateRootNoSymlink(relative, storageDirectoryMode())
		workspaceRoot.Close()
		return assetRoot, openErr
	}
	assetRoot, openErr := workspaceRoot.OpenRootNoSymlink(relative)
	workspaceRoot.Close()
	return assetRoot, openErr
}
