package room

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
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

	contextValue, err := s.repository.RemoveRoomMember(ctx, authctx.OwnerUserID(ctx), strings.TrimSpace(roomID), normalizedAgentID)
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrRoomNotFound
	}
	runtimeErr := s.closeConversationRuntimeSessions(ctx, roomContexts, false, map[string]struct{}{normalizedAgentID: {}})
	artifactErr := s.cleanupConversationArtifacts(ctx, roomContexts, false, map[string]struct{}{normalizedAgentID: {}})
	goalErr := s.cleanupGoalsForRoomMemberContexts(ctx, roomContexts, normalizedAgentID)
	return contextValue, errors.Join(runtimeErr, artifactErr, goalErr)
}
