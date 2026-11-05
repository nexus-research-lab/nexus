package room

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// ConversationAssetFile 返回 Room conversation 公共附件目录内相对路径的绝对路径与已有文件大小；
// 不存在时 size 为 -1。只读取 confinedfs 根，不跟随越界路径。
func (s *Service) ConversationAssetFile(ctx context.Context, roomID, conversationID, relativePath string) (string, int64, error) {
	contextValue, err := s.GetConversationContext(ctx, conversationID)
	if err != nil {
		return "", -1, err
	}
	if contextValue.Room.ID != roomID {
		return "", -1, ErrConversationNotFound
	}
	root, err := workspacestore.New(s.config.WorkspacePath).OpenRoomConversationAssetRoot(contextValue.Room.OwnerUserID, conversationID, true)
	if err != nil {
		return "", -1, err
	}
	defer root.Close()
	absolute := filepath.Join(root.Name(), filepath.FromSlash(relativePath))
	info, err := root.Stat(relativePath)
	if errors.Is(err, fs.ErrNotExist) {
		return absolute, -1, nil
	}
	if err != nil {
		return "", -1, err
	}
	return absolute, info.Size(), nil
}

// UploadConversationAttachment 上传 Room conversation 级公共附件。
func (s *Service) UploadConversationAttachment(
	ctx context.Context,
	roomID string,
	conversationID string,
	filename string,
	destination string,
	reader io.Reader,
) (*workspacepkg.UploadResult, error) {
	contextValue, err := s.GetConversationContext(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(contextValue.Room.ID) != strings.TrimSpace(roomID) {
		return nil, ErrConversationNotFound
	}
	if contextValue.Room.RoomType == protocol.RoomTypeDM {
		return nil, errors.New("DM conversation does not support room attachments")
	}

	store := workspacestore.New(s.config.WorkspacePath)
	root, err := store.OpenRoomConversationAssetRoot(
		contextValue.Room.OwnerUserID,
		conversationID,
		true,
	)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return workspacepkg.UploadFileWithRoot(
		root.Name(),
		root,
		filename,
		destination,
		reader,
	)
}
