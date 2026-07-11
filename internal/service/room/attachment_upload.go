package room

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

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

	root := workspacestore.New(s.config.WorkspacePath).RoomConversationDir(conversationID)
	return workspacepkg.UploadFileToRoot(root, filename, destination, reader)
}
