package communication

import (
	"context"
	"strings"
)

// hostOwnedRoomReply 由 channels 控制面实现：Room 根轮次来自已配对 IM 入站时，回信只由宿主统一转发。
type hostOwnedRoomReply interface {
	HostOwnsRoomExternalReply(context.Context, string, string) (bool, error)
}

// rejectModelRoutedRoomReply 阻止 IM 入站的 Room 轮次由模型自行回投外部会话，
// 否则回复会落进 IM session 而不是群聊，并与宿主转发重复或互相遮蔽。
func (s *Service) rejectModelRoutedRoomReply(ctx context.Context, actor Actor) error {
	owner, ok := s.external.(hostOwnedRoomReply)
	if !ok || actor.ContextKind != ContextKindRoom || strings.TrimSpace(actor.RoundID) == "" {
		return nil
	}
	owned, err := owner.HostOwnsRoomExternalReply(ctx, actor.OwnerUserID, actor.RoundID)
	if err != nil {
		return err
	}
	if owned {
		return newInputError("本轮来自已配对的外部用户，群里的公开回复会由系统自动转回对方；请直接在群里回答，不要自行发回外部会话")
	}
	return nil
}
