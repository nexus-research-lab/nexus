// INPUT: Room 成员增删与持久 participation_paused 变更请求。
// OUTPUT: 经成员身份和 Room 类型校验后的最新主 conversation 上下文。
// POS: Room 成员生命周期与参与状态的业务事务边界。
package room

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	deletionsvc "github.com/nexus-research-lab/nexus/internal/service/deletion"
)

// AddRoomMember 向房间追加成员。
func (s *Service) AddRoomMember(ctx context.Context, roomID string, request protocol.AddRoomMemberRequest) (*protocol.ConversationContextAggregate, error) {
	agentValue, err := s.ensureGroupMemberAgent(ctx, request.AgentID)
	if err != nil {
		return nil, err
	}
	normalizedAgentID := agentValue.AgentID

	agentRefs, err := s.loadAgentRefs(ctx, []string{normalizedAgentID})
	if err != nil {
		return nil, err
	}
	contextValue, err := s.repository.AddRoomMember(ctx, authctx.OwnerUserID(ctx), strings.TrimSpace(roomID), agentRefs[0])
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrRoomNotFound
	}
	return contextValue, nil
}

// RemoveRoomMember 从房间移除成员。
func (s *Service) RemoveRoomMember(ctx context.Context, roomID string, agentID string) (*protocol.ConversationContextAggregate, error) {
	roomID = strings.TrimSpace(roomID)
	agentID = strings.TrimSpace(agentID)
	jobTargetID := roomID + ":" + agentID
	if job, payload, err := s.loadRoomDeletionJob(
		ctx,
		deletionsvc.KindRoomMember,
		jobTargetID,
	); err != nil {
		return nil, err
	} else if job != nil {
		return s.applyRoomMemberDeletion(ctx, *job, payload)
	}
	agentValue, err := s.ensureGroupMemberAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	normalizedAgentID := agentValue.AgentID

	roomContexts, err := s.GetRoomContexts(ctx, roomID)
	if err != nil {
		return nil, err
	}
	roomValue := roomContexts[0].Room
	if roomValue.RoomType != protocol.RoomTypeGroup {
		return nil, errors.New("DM room does not support removing members")
	}
	agentCount := 0
	memberFound := false
	for _, member := range roomContexts[0].Members {
		if member.MemberType == protocol.MemberTypeAgent && member.MemberAgentID != "" {
			agentCount++
		}
		if member.MemberType == protocol.MemberTypeAgent && member.MemberAgentID == normalizedAgentID {
			memberFound = true
		}
	}
	if !memberFound {
		return nil, ErrRoomMemberNotFound
	}
	if agentCount <= 1 {
		return nil, errors.New("Room 至少保留一个 agent 成员")
	}

	transcriptReferences, err := s.captureRoomTranscriptReferences(roomContexts)
	if err != nil {
		return nil, err
	}
	payload := roomDeletionPayload{
		AgentID:              normalizedAgentID,
		AgentIDs:             []string{normalizedAgentID},
		Contexts:             roomContexts,
		RoomID:               roomID,
		TranscriptReferences: transcriptReferences,
	}
	job, err := s.ensureRoomDeletionJob(
		ctx,
		deletionsvc.KindRoomMember,
		jobTargetID,
		payload,
	)
	if err != nil {
		return nil, err
	}
	return s.applyRoomMemberDeletion(ctx, job, payload)
}

// SetRoomMemberParticipation 持久化 group Room Agent 的参与暂停状态。
// 活跃 runtime 的中断和恢复调度由 realtime 在 conversation 派发锁内完成。
func (s *Service) SetRoomMemberParticipation(
	ctx context.Context,
	roomID string,
	agentID string,
	paused bool,
) (*protocol.ConversationContextAggregate, error) {
	normalizedRoomID := strings.TrimSpace(roomID)
	normalizedAgentID := strings.TrimSpace(agentID)
	if normalizedRoomID == "" || normalizedAgentID == "" {
		return nil, ErrRoomMemberNotFound
	}
	roomContexts, err := s.GetRoomContexts(ctx, normalizedRoomID)
	if err != nil {
		return nil, err
	}
	if len(roomContexts) == 0 {
		return nil, ErrRoomNotFound
	}
	if roomContexts[0].Room.RoomType != protocol.RoomTypeGroup {
		return nil, errors.New("DM room does not support member participation controls")
	}
	memberFound := false
	for _, member := range roomContexts[0].Members {
		if member.MemberType == protocol.MemberTypeAgent &&
			strings.TrimSpace(member.MemberAgentID) == normalizedAgentID {
			memberFound = true
			break
		}
	}
	if !memberFound {
		return nil, ErrRoomMemberNotFound
	}
	contextValue, err := s.repository.SetRoomMemberParticipation(
		ctx,
		authctx.OwnerUserID(ctx),
		normalizedRoomID,
		normalizedAgentID,
		paused,
	)
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrRoomMemberNotFound
	}
	return contextValue, nil
}
