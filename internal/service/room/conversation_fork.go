// INPUT: 单 Agent DM conversation 与一个已完成 round_id。
// OUTPUT: 同 Room 下待首次输入物化独立 SDK transcript 的新 conversation。
// POS: Conversation SQL 身份与 DM runtime fork 的事务补偿编排层。
package room

import (
	"context"
	"errors"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
)

// ForkConversation 从 source 的已完成消息轮次创建一个新对话。
func (s *Service) ForkConversation(
	ctx context.Context,
	roomID string,
	sourceConversationID string,
	request protocol.ForkConversationRequest,
) (*protocol.ConversationContextAggregate, error) {
	roomID = strings.TrimSpace(roomID)
	sourceConversationID = strings.TrimSpace(sourceConversationID)
	targetRoundID := strings.TrimSpace(request.RoundID)
	if targetRoundID == "" {
		return nil, errors.New("round_id 不能为空")
	}

	source, err := s.GetConversationContext(ctx, sourceConversationID)
	if err != nil {
		return nil, err
	}
	if source.Room.ID != roomID {
		return nil, ErrConversationNotFound
	}
	if source.Room.RoomType != protocol.RoomTypeDM || len(source.Sessions) != 1 {
		return nil, ErrConversationForkUnsupported
	}
	if s.sessionForker == nil {
		return nil, errors.New("conversation session fork capability is unavailable")
	}

	sourceSession := source.Sessions[0]
	targetConversationID := roomdomain.NewEntityID()
	targetSession := protocol.SessionRecord{
		ID:             roomdomain.NewEntityID(),
		ConversationID: targetConversationID,
		AgentID:        sourceSession.AgentID,
		RuntimeID:      sourceSession.RuntimeID,
		VersionNo:      1,
		BranchKey:      "main",
		IsPrimary:      true,
		Options:        forkConversationSessionOptions(sourceSession.Options),
		Status:         "active",
	}
	created, err := s.repository.CreateConversation(ctx, roomrepo.CreateConversationBundle{
		OwnerUserID: authctx.OwnerUserID(ctx),
		RoomID:      roomID,
		Conversation: protocol.ConversationRecord{
			ID:               targetConversationID,
			RoomID:           roomID,
			ConversationType: protocol.ConversationTypeTopic,
			Title:            source.Conversation.Title,
			IsDraft:          false,
		},
		Sessions: []protocol.SessionRecord{targetSession},
	})
	if err != nil {
		return nil, err
	}
	if created == nil {
		return nil, ErrRoomNotFound
	}

	sourceSessionKey := protocol.BuildRoomAgentSessionKey(
		sourceConversationID,
		sourceSession.AgentID,
		source.Room.RoomType,
	)
	targetSessionKey := protocol.BuildRoomAgentSessionKey(
		targetConversationID,
		sourceSession.AgentID,
		source.Room.RoomType,
	)
	if err = s.sessionForker.ForkConversationSession(
		ctx,
		sourceSessionKey,
		targetSessionKey,
		targetRoundID,
	); err != nil {
		if _, cleanupErr := s.DeleteConversation(ctx, roomID, targetConversationID); cleanupErr != nil {
			return nil, errors.Join(err, cleanupErr)
		}
		return nil, err
	}
	return s.GetConversationContext(ctx, targetConversationID)
}

func forkConversationSessionOptions(source map[string]any) map[string]any {
	options := protocol.WithSessionRuntimeSettings(
		nil,
		protocol.SessionRuntimeSettingsFromOptions(source),
	)
	return protocol.WithSessionAdditionalDirectories(
		options,
		protocol.SessionAdditionalDirectoriesFromOptions(source),
	)
}
