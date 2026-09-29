package relay

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	relaycontract "github.com/nexus-research-lab/nexus/internal/relay"
)

// GetRoom 读取当前 active 真人成员可见的在线 Room 管理快照。
func (c *Client) GetRoom(ctx context.Context, token, roomID string) (relaycontract.RoomDetails, error) {
	roomID, err := requireResourceID(roomID, "room_id")
	if err != nil {
		return relaycontract.RoomDetails{}, err
	}
	var result relaycontract.RoomDetails
	err = c.do(ctx, http.MethodGet, "/rooms/"+url.PathEscape(roomID), nil, token, "", nil, &result)
	if err == nil {
		result.Conversation.StreamEpoch, err = responseStreamEpoch(result.Conversation.StreamEpoch, "")
	}
	return result, err
}

// ListInvitations 读取当前真人尚未处理的在线 Room 邀请。
func (c *Client) ListInvitations(ctx context.Context, token string) (relaycontract.RoomInvitationList, error) {
	var result relaycontract.RoomInvitationList
	err := c.do(ctx, http.MethodGet, "/invitations", nil, token, "", nil, &result)
	return result, err
}

// InviteUser 邀请一名已由 Nexus Gateway 确认属于当前 Organization 的真人。
func (c *Client) InviteUser(ctx context.Context, token, roomID, key string, input relaycontract.InviteRoomMemberInput) (relaycontract.RoomMembershipMutation, error) {
	return c.membershipMutation(ctx, token, roomID, key, http.MethodPost, "/invitations", input)
}

// AddAgent 将已验证归当前真人所有的 Agent 加入 Room。
func (c *Client) AddAgent(ctx context.Context, token, roomID, key string, input relaycontract.AddRoomAgentInput) (relaycontract.RoomMembershipMutation, error) {
	return c.membershipMutation(ctx, token, roomID, key, http.MethodPost, "/agents", input)
}

// RemoveAgent 将 Agent 成员移出 Room。
func (c *Client) RemoveAgent(ctx context.Context, token, roomID, agentID, key string, input relaycontract.RemoveRoomAgentInput) (relaycontract.RoomMembershipMutation, error) {
	agentID, err := requireResourceID(agentID, "agent_id")
	if err != nil {
		return relaycontract.RoomMembershipMutation{}, err
	}
	return c.membershipMutation(ctx, token, roomID, key, http.MethodDelete, "/agents/"+url.PathEscape(agentID), input)
}

// UpdateAgent 暂停或恢复当前真人拥有的在线 Agent。
func (c *Client) UpdateAgent(ctx context.Context, token, roomID, agentID, key string, input relaycontract.UpdateRoomAgentInput) (relaycontract.RoomMembershipMutation, error) {
	agentID, err := requireResourceID(agentID, "agent_id")
	if err != nil {
		return relaycontract.RoomMembershipMutation{}, err
	}
	return c.membershipMutation(ctx, token, roomID, key, http.MethodPatch, "/agents/"+url.PathEscape(agentID), input)
}

// UpdateRoom 更换当前在线 Room 的主持 Agent。
func (c *Client) UpdateRoom(ctx context.Context, token, roomID, key string, input relaycontract.UpdateRoomInput) (relaycontract.RoomConfigurationMutation, error) {
	roomID, err := requireResourceID(roomID, "room_id")
	if err != nil {
		return relaycontract.RoomConfigurationMutation{}, err
	}
	key = strings.TrimSpace(key)
	if !validCommandID(key) {
		return relaycontract.RoomConfigurationMutation{}, errors.New("Idempotency-Key 必须为 1-128 字节的可见 ASCII")
	}
	var result relaycontract.RoomConfigurationMutation
	err = c.do(ctx, http.MethodPatch, "/rooms/"+url.PathEscape(roomID), nil, token, key, input, &result)
	return result, err
}

// AcceptInvitation 接受当前真人自己的在线 Room 邀请。
func (c *Client) AcceptInvitation(ctx context.Context, token, roomID, key string, input relaycontract.ResolveRoomInvitationInput) (relaycontract.RoomMembershipMutation, error) {
	return c.membershipMutation(ctx, token, roomID, key, http.MethodPost, "/invitations/accept", input)
}

// RejectInvitation 拒绝当前真人自己的在线 Room 邀请。
func (c *Client) RejectInvitation(ctx context.Context, token, roomID, key string, input relaycontract.ResolveRoomInvitationInput) (relaycontract.RoomMembershipMutation, error) {
	return c.membershipMutation(ctx, token, roomID, key, http.MethodPost, "/invitations/reject", input)
}

// RevokeInvitation 撤销目标真人尚未接受的邀请。
func (c *Client) RevokeInvitation(ctx context.Context, token, roomID, userID, key string, input relaycontract.ResolveRoomInvitationInput) (relaycontract.RoomMembershipMutation, error) {
	userID, err := requireResourceID(userID, "user_id")
	if err != nil {
		return relaycontract.RoomMembershipMutation{}, err
	}
	return c.membershipMutation(ctx, token, roomID, key, http.MethodDelete, "/invitations/"+url.PathEscape(userID), input)
}

// UpdateMember 修改一名 active 真人成员。
func (c *Client) UpdateMember(ctx context.Context, token, roomID, userID, key string, input relaycontract.UpdateRoomMemberInput) (relaycontract.RoomMembershipMutation, error) {
	userID, err := requireResourceID(userID, "user_id")
	if err != nil {
		return relaycontract.RoomMembershipMutation{}, err
	}
	return c.membershipMutation(ctx, token, roomID, key, http.MethodPatch, "/members/"+url.PathEscape(userID), input)
}

// TransferOwnership 将唯一真人群主移交给另一名 active 真人成员。
func (c *Client) TransferOwnership(ctx context.Context, token, roomID, key string, input relaycontract.TransferRoomOwnershipInput) (relaycontract.RoomMembershipMutation, error) {
	return c.membershipMutation(ctx, token, roomID, key, http.MethodPost, "/transfer", input)
}

func (c *Client) membershipMutation(
	ctx context.Context,
	token string,
	roomID string,
	key string,
	method string,
	suffix string,
	input any,
) (relaycontract.RoomMembershipMutation, error) {
	roomID, err := requireResourceID(roomID, "room_id")
	if err != nil {
		return relaycontract.RoomMembershipMutation{}, err
	}
	key = strings.TrimSpace(key)
	if !validCommandID(key) {
		return relaycontract.RoomMembershipMutation{}, errors.New("Idempotency-Key 必须为 1-128 字节的可见 ASCII")
	}
	var result relaycontract.RoomMembershipMutation
	err = c.do(ctx, method, "/rooms/"+url.PathEscape(roomID)+suffix, nil, token, key, input, &result)
	return result, err
}

// MarkRead 推进当前真人的阅读水位，幂等性由 Relay 的单调更新保证。
func (c *Client) MarkRead(ctx context.Context, token, roomID string, input relaycontract.MarkReadInput) (relaycontract.ReadState, error) {
	roomID, err := requireResourceID(roomID, "room_id")
	if err != nil {
		return relaycontract.ReadState{}, err
	}
	var result relaycontract.ReadState
	err = c.do(ctx, http.MethodPut, "/rooms/"+url.PathEscape(roomID)+"/read-state", nil, token, "", input, &result)
	return result, err
}

// RoomDeliveryStatuses 仅查询调用方已加载消息的公开投递状态。
func (c *Client) RoomDeliveryStatuses(ctx context.Context, token, roomID string, messageIDs []string) ([]relaycontract.DeliveryStatus, error) {
	roomID, err := requireResourceID(roomID, "room_id")
	if err != nil {
		return nil, err
	}
	if len(messageIDs) == 0 || len(messageIDs) > 100 {
		return nil, errors.New("投递查询需提供 1 至 100 条消息")
	}
	query := url.Values{}
	for _, id := range messageIDs {
		id, err := requireResourceID(id, "message_id")
		if err != nil {
			return nil, err
		}
		query.Add("message_id", id)
	}
	var result []relaycontract.DeliveryStatus
	err = c.do(ctx, http.MethodGet, "/rooms/"+url.PathEscape(roomID)+"/deliveries", query, token, "", nil, &result)
	return result, err
}

func (c *Client) RoomMembers(ctx context.Context, token, roomID, cursor, epoch string, version int64) (relaycontract.RoomMemberPage, error) {
	roomID, err := requireResourceID(roomID, "room_id")
	if err != nil {
		return relaycontract.RoomMemberPage{}, err
	}
	query := url.Values{"after": {cursor}, "stream_epoch": {epoch}, "membership_version": {strconv.FormatInt(version, 10)}}
	var result relaycontract.RoomMemberPage
	err = c.do(ctx, http.MethodGet, "/rooms/"+url.PathEscape(roomID)+"/members", query, token, "", nil, &result)
	return result, err
}

// GetRoomWithMembers 为本机执行映射读取完整成员集合，每页仍携带版本与世代栅栏。
func (c *Client) GetRoomWithMembers(ctx context.Context, token, roomID string) (relaycontract.RoomDetails, error) {
	result, err := c.GetRoom(ctx, token, roomID)
	if err != nil {
		return result, err
	}
	for result.NextMemberCursor != "" {
		page, err := c.RoomMembers(ctx, token, roomID, result.NextMemberCursor, result.Conversation.StreamEpoch, result.Room.MembershipVersion)
		if err != nil {
			return relaycontract.RoomDetails{}, err
		}
		if page.NextCursor == result.NextMemberCursor {
			return relaycontract.RoomDetails{}, errors.New("成员分页游标未推进")
		}
		result.Members = append(result.Members, page.Members...)
		result.NextMemberCursor = page.NextCursor
	}
	return result, nil
}
