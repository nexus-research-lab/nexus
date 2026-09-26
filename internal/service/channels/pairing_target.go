// INPUT: 已授权的 IM 配对、目标 Room 话题与绑定版本。
// OUTPUT: 不迁移历史的目标切换，以及收发两端的版本校验。
// POS: IM 传输身份与 Room 执行身份之间的唯一绑定边界。
package channels

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	channelmanagement "github.com/nexus-research-lab/nexus/internal/service/channels/management"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
)

// SetRoomService 注入目标绑定的 owner 及成员校验来源。
func (s *ControlService) SetRoomService(rooms *roomsvc.Service) { s.rooms = rooms }

func (s *ControlService) validatePairingRoom(ctx context.Context, agentID string, target channelmanagement.PairingSessionTarget) error {
	if target.RoomID == "" && target.ConversationID == "" {
		return nil
	}
	if s.rooms == nil || target.RoomID == "" || target.ConversationID == "" {
		return errors.New("Room 会话目标不完整")
	}
	value, err := s.rooms.GetConversationContext(ctx, target.ConversationID)
	if err != nil {
		return err
	}
	if value.Room.ID != target.RoomID || value.Room.RoomType != protocol.RoomTypeGroup || !value.Room.PrivateMessagesEnabled {
		return errors.New("Room 不存在或未开启成员私域消息")
	}
	for _, member := range value.Members {
		if member.MemberAgentID == agentID && member.MemberType == protocol.MemberTypeAgent && !member.ParticipationPaused {
			return nil
		}
	}
	return errors.New("绑定的智能体不是该 Room 成员")
}

func (s *ControlService) validatePairingTargetPatch(ctx context.Context, owner, id string, request UpdatePairingRequest) error {
	if request.SessionTarget == nil {
		return nil
	}
	if request.BindingVersion == nil {
		return invalidChannelControl(errors.New("切换会话需要当前 binding_version"))
	}
	row, err := s.getPairingRow(ctx, owner, id)
	if err != nil {
		return err
	}
	if row == nil {
		return ErrPairingNotFound
	}
	// 第一版只向已配对真人私聊开放成员会话，群聊不能取得成员私域。
	if row.ChatType != protocol.RoomTypeDM {
		return invalidChannelControl(errors.New("Room 绑定仅支持 IM 私聊"))
	}
	target := request.SessionTarget
	target.RoomID = strings.TrimSpace(target.RoomID)
	target.ConversationID = strings.TrimSpace(target.ConversationID)
	agent := row.AgentID
	if request.AgentID != nil {
		agent = *request.AgentID
	}
	return s.validatePairingRoom(contextWithIngressOwner(ctx, owner), agent, *target)
}

func (s *ControlService) patchPairingTarget(ctx context.Context, tx *sql.Tx, row *pairingRow, request UpdatePairingRequest) error {
	if request.BindingVersion != nil && *request.BindingVersion != row.BindingVersion {
		return invalidChannelControl(errors.New("配对目标已变更，请刷新后重试"))
	}
	target := channelmanagement.PairingSessionTarget{RoomID: row.TargetRoomID, ConversationID: row.TargetConversationID}
	agentChanged := request.AgentID != nil && *request.AgentID != row.AgentID
	if agentChanged {
		target = channelmanagement.PairingSessionTarget{}
	}
	if request.SessionTarget != nil {
		target = *request.SessionTarget
	}
	changed := target.RoomID != row.TargetRoomID || target.ConversationID != row.TargetConversationID
	if !changed && !agentChanged && (request.Status == nil || *request.Status == row.Status) {
		return nil
	}
	if _, err := tx.ExecContext(ctx, "UPDATE im_pairings SET target_room_id="+s.bind(1)+", target_conversation_id="+s.bind(2)+", binding_version=binding_version+1 WHERE owner_user_id="+s.bind(3)+" AND pairing_id="+s.bind(4), target.RoomID, target.ConversationID, row.OwnerUserID, row.PairingID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "UPDATE im_deliveries SET return_revoked=1 WHERE owner_user_id="+s.bind(1)+" AND pairing_id="+s.bind(2), row.OwnerUserID, row.PairingID)
	return err
}

func (s *ControlService) ingressPairing(ctx context.Context, owner, agent, session string) (*pairingRow, error) {
	if err := s.ValidateExternalSessionGrant(ctx, owner, agent, session); err != nil {
		return nil, err
	}
	p := protocol.ParseSessionKey(session)
	row, err := s.findPairingBySessionKey(ctx, owner, session, PairingStatusActive)
	if row == nil && err == nil {
		row, err = s.findIngressPairingByTarget(ctx, owner, normalizeIMChannelType(p.Channel), p.AccountID, p.ChatType, p.Ref, ingressPairingThreadID(p.ChatType, p.ThreadID), PairingStatusActive)
	}
	if err != nil {
		return nil, err
	}
	if row == nil || row.AgentID != agent {
		return nil, ErrExternalSessionGrantUnavailable
	}
	return row, nil
}

// ValidateBindingDelivery 在物理发送前重新校验，不因切回旧目标恢复旧版本的回信资格。
func (s *ControlService) ValidateBindingDelivery(ctx context.Context, owner, agent string, target DeliveryTarget) (bool, error) {
	row, err := s.ingressPairing(ctx, owner, agent, target.SessionKey)
	if err != nil {
		return false, err
	}
	if row.PairingID != target.PairingID || row.BindingVersion != target.BindingVersion {
		return false, ErrExternalSessionGrantUnavailable
	}
	binding := channelmanagement.PairingSessionTarget{RoomID: row.TargetRoomID, ConversationID: row.TargetConversationID}
	if err := s.validatePairingRoom(contextWithIngressOwner(ctx, owner), agent, binding); err != nil {
		return false, err
	}
	return binding.RoomID != "", nil
}

func (s *ControlService) pairingTargetView(ctx context.Context, row pairingRow) channelmanagement.PairingSessionTarget {
	target := channelmanagement.PairingSessionTarget{RoomID: row.TargetRoomID, ConversationID: row.TargetConversationID}
	if s.rooms == nil || target.RoomID == "" {
		return target
	}
	value, err := s.rooms.GetConversationContext(contextWithIngressOwner(ctx, row.OwnerUserID), target.ConversationID)
	if err == nil && value.Room.ID == target.RoomID {
		target.RoomName = value.Room.Name
		target.ConversationTitle = value.Conversation.Title
	}
	return target
}

// AcquireBindingDelivery 持有配对写锁直到物理发送结束，改绑完成后不会再发旧版本。
func (s *ControlService) AcquireBindingDelivery(ctx context.Context, owner, agent string, target DeliveryTarget) (func(), error) {
	unlock := s.lockPairingMutation(owner)
	if _, err := s.ValidateBindingDelivery(ctx, owner, agent, target); err != nil {
		unlock()
		return nil, err
	}
	return unlock, nil
}
