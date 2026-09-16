package team

import (
	"context"

	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"
)

// GetRoom 返回在线 Room 的管理快照。
func (s *Service) GetRoom(ctx context.Context, access Access, roomID string) (relaycontract.RoomDetails, error) {
	return s.relay.GetRoom(ctx, access.Token, roomID)
}

// ListInvitations 返回当前真人的待处理在线 Room 邀请。
func (s *Service) ListInvitations(ctx context.Context, access Access) (relaycontract.RoomInvitationList, error) {
	return s.relay.ListInvitations(ctx, access.Token)
}

// InviteUser 创建真人 Room 邀请。
func (s *Service) InviteUser(ctx context.Context, access Access, roomID, key string, input relaycontract.InviteRoomMemberInput) (relaycontract.RoomMembershipMutation, error) {
	return s.relay.InviteUser(ctx, access.Token, roomID, key, input)
}

// AcceptInvitation 接受当前真人自己的邀请。
func (s *Service) AcceptInvitation(ctx context.Context, access Access, roomID, key string, input relaycontract.ResolveRoomInvitationInput) (relaycontract.RoomMembershipMutation, error) {
	return s.relay.AcceptInvitation(ctx, access.Token, roomID, key, input)
}

// RejectInvitation 拒绝当前真人自己的邀请。
func (s *Service) RejectInvitation(ctx context.Context, access Access, roomID, key string, input relaycontract.ResolveRoomInvitationInput) (relaycontract.RoomMembershipMutation, error) {
	return s.relay.RejectInvitation(ctx, access.Token, roomID, key, input)
}

// RevokeInvitation 撤销一条 pending 邀请。
func (s *Service) RevokeInvitation(ctx context.Context, access Access, roomID, userID, key string, input relaycontract.ResolveRoomInvitationInput) (relaycontract.RoomMembershipMutation, error) {
	return s.relay.RevokeInvitation(ctx, access.Token, roomID, userID, key, input)
}

// UpdateMember 修改真人角色或移除成员。
func (s *Service) UpdateMember(ctx context.Context, access Access, roomID, userID, key string, input relaycontract.UpdateRoomMemberInput) (relaycontract.RoomMembershipMutation, error) {
	return s.relay.UpdateMember(ctx, access.Token, roomID, userID, key, input)
}

// TransferOwnership 移交唯一真人群主。
func (s *Service) TransferOwnership(ctx context.Context, access Access, roomID, key string, input relaycontract.TransferRoomOwnershipInput) (relaycontract.RoomMembershipMutation, error) {
	return s.relay.TransferOwnership(ctx, access.Token, roomID, key, input)
}
