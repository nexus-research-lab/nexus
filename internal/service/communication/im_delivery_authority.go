// INPUT: Host-owned IM delivery records and verified communication identity.
// OUTPUT: Exact-session feedback without transferring previous round authority.
// POS: IM scenario adapter; existing contact and Room messaging remain independent.
package communication

import (
	"context"
	"errors"
	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/imdelivery"
	"time"
)

func (s *Service) validateIMReturn(ctx context.Context, d imdelivery.Delivery) error {
	if d.ReturnRevoked {
		return errors.New("投递来源关联已因配对变化失效")
	}
	if d.Source.SessionKey == "" || d.Source.SessionCreatedAt == "" || d.Source.SessionKey == d.TargetSessionKey {
		return errors.New("原会话不可回传")
	}
	if d.State != "sent" && d.State != "unknown" {
		return errors.New("该投递没有可确认的发送事实")
	}
	pairing, err := s.im.inputs.IMDeliveryPairing(ctx, d.OwnerUserID, d.TargetAgentID, d.TargetSessionKey)
	if err != nil {
		return err
	}
	if pairing != d.PairingID {
		return errors.New("投递配对已失效")
	}
	target, err := s.im.sessions.ResolveDeliverySession(ctx, d.TargetSessionKey)
	if err != nil {
		return err
	}
	if target == nil || target.CreatedAt.UTC().Format(time.RFC3339Nano) != d.TargetCreatedAt {
		return errors.New("IM 会话已失效")
	}
	source, err := s.resolveIMSourceSession(ctx, d.Source.SessionKey)
	if err != nil {
		return err
	}
	if source == nil || source.CreatedAt.UTC().Format(time.RFC3339Nano) != d.Source.SessionCreatedAt {
		return errors.New("原会话已删除或被替换")
	}
	agent, err := s.agents.GetAgent(ctx, d.Source.AgentID)
	if err != nil {
		return err
	}
	if agent == nil || agent.OwnerUserID != d.OwnerUserID {
		return errors.New("原智能体不可用")
	}
	if d.Source.Kind == "automation" {
		if s.im.automationPolicy == nil {
			return errors.New("自动化来源准入未装配")
		}
		if _, err := s.im.automationPolicy(ctx, d.Source); err != nil {
			return err
		}
		if source.ChatType == protocol.RoomTypeGroup {
			return errors.New("自动化群来源不能以普通群权限接收反馈")
		}
	}
	if source.ChatType == protocol.RoomTypeGroup {
		if d.Source.RoomID == "" || d.Source.ConversationID == "" {
			return errors.New("缺少原群会话定位")
		}
		room, err := s.rooms.GetConversationContext(ctx, d.Source.ConversationID)
		if err != nil {
			return err
		}
		if room.Room.ID != d.Source.RoomID || !room.Room.PrivateMessagesEnabled {
			return errors.New("原群会话私域接收不可用")
		}
		if !roomdomain.IsMemberAgent(room.Members, d.Source.AgentID) {
			return errors.New("来源成员已离开群")
		}
	} else if source.AgentID != d.Source.AgentID {
		return errors.New("原会话智能体不匹配")
	}
	return nil
}

// ValidateIMReply rechecks persisted origin and live grants before queue dispatch.
func (s *Service) ValidateIMReply(ctx context.Context, owner, id string) error {
	if s.im == nil || authctx.OwnerUserID(ctx) != owner {
		return imdelivery.ErrUnavailable
	}
	reply, err := s.im.store.GetReply(ctx, owner, id)
	if err != nil {
		return err
	}
	d, err := s.im.store.GetDelivery(ctx, owner, reply.DeliveryID)
	if err != nil {
		return err
	}
	return s.validateIMReturn(ctx, d)
}

func (s *Service) resolveIMSourceSession(ctx context.Context, key string) (*protocol.Session, error) {
	if resolver, ok := s.im.sessions.(interface {
		GetSession(context.Context, string) (*protocol.Session, error)
	}); ok {
		return resolver.GetSession(ctx, key)
	}
	return s.im.sessions.ResolveDeliverySession(ctx, key)
}
