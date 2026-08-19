// INPUT: Room 成员增删、持久 participation_paused 与可选 configuration_version 请求。
// OUTPUT: 经成员身份、Room 类型与 CAS 校验后的最新主 conversation 上下文。
// POS: Room 成员生命周期与参与状态的业务事务边界。
package room

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
)

// AddRoomMember 向房间追加成员。
func (s *Service) AddRoomMember(ctx context.Context, roomID string, request protocol.AddRoomMemberRequest) (*protocol.ConversationContextAggregate, error) {
	return s.addRoomMember(ctx, roomID, request, nil)
}

// AddRoomMemberAtVersion 使用 Room 资源版本追加成员。
func (s *Service) AddRoomMemberAtVersion(
	ctx context.Context,
	roomID string,
	request protocol.AddRoomMemberRequest,
	expectedVersion int64,
) (*protocol.ConversationContextAggregate, error) {
	return s.addRoomMember(ctx, roomID, request, &expectedVersion)
}

func (s *Service) addRoomMember(
	ctx context.Context,
	roomID string,
	request protocol.AddRoomMemberRequest,
	expectedVersion *int64,
) (*protocol.ConversationContextAggregate, error) {
	agentValue, err := s.ensureGroupMemberAgent(ctx, request.AgentID)
	if err != nil {
		return nil, err
	}
	normalizedAgentID := agentValue.AgentID

	agentRefs, err := s.loadAgentRefs(ctx, []string{normalizedAgentID})
	if err != nil {
		return nil, err
	}
	var contextValue *protocol.ConversationContextAggregate
	if expectedVersion == nil {
		contextValue, err = s.repository.AddRoomMember(
			ctx, authctx.OwnerUserID(ctx), strings.TrimSpace(roomID), agentRefs[0],
		)
	} else {
		versioned, ok := s.repository.(interface {
			AddRoomMemberAtVersion(
				context.Context, string, string, roomrepo.AgentRuntimeRef, int64,
			) (*protocol.ConversationContextAggregate, error)
		})
		if !ok {
			return nil, errors.New("Room repository 不支持成员资源版本")
		}
		contextValue, err = versioned.AddRoomMemberAtVersion(
			ctx, authctx.OwnerUserID(ctx), strings.TrimSpace(roomID), agentRefs[0], *expectedVersion,
		)
	}
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
	return s.removeRoomMember(ctx, roomID, agentID, nil)
}

// RemoveRoomMemberAtVersion 使用 Room 资源版本移除成员。
func (s *Service) RemoveRoomMemberAtVersion(
	ctx context.Context,
	roomID string,
	agentID string,
	expectedVersion int64,
) (*protocol.ConversationContextAggregate, error) {
	return s.removeRoomMember(ctx, roomID, agentID, &expectedVersion)
}

func (s *Service) removeRoomMember(
	ctx context.Context,
	roomID string,
	agentID string,
	expectedVersion *int64,
) (*protocol.ConversationContextAggregate, error) {
	roomID = strings.TrimSpace(roomID)
	agentID = strings.TrimSpace(agentID)
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
	if expectedVersion == nil {
		if cleanupErr := s.cleanupCommittedRoomDeletion(ctx, payload, false); cleanupErr != nil {
			return nil, cleanupErr
		}
	}
	var contextValue *protocol.ConversationContextAggregate
	if expectedVersion == nil {
		contextValue, err = s.repository.RemoveRoomMember(
			ctx, authctx.OwnerUserID(ctx), roomID, normalizedAgentID,
		)
	} else {
		versioned, ok := s.repository.(interface {
			RemoveRoomMemberAtVersion(
				context.Context, string, string, string, int64,
			) (*protocol.ConversationContextAggregate, error)
		})
		if !ok {
			return nil, errors.New("Room repository 不支持成员资源版本")
		}
		contextValue, err = versioned.RemoveRoomMemberAtVersion(
			ctx, authctx.OwnerUserID(ctx), roomID, normalizedAgentID, *expectedVersion,
		)
	}
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrRoomNotFound
	}
	if expectedVersion != nil {
		if cleanupErr := s.cleanupCommittedRoomDeletion(ctx, payload, false); cleanupErr != nil {
			return contextValue, &RoomMemberDeletionReconcileError{cause: cleanupErr}
		}
	}
	return contextValue, nil
}

// SetRoomMemberParticipation 持久化 group Room Agent 的参与暂停状态。
// 活跃 runtime 的中断和恢复调度由 realtime 在 conversation 派发锁内完成。
func (s *Service) SetRoomMemberParticipation(
	ctx context.Context,
	roomID string,
	agentID string,
	paused bool,
) (*protocol.ConversationContextAggregate, error) {
	return s.setRoomMemberParticipation(ctx, roomID, agentID, paused, nil)
}

// SetRoomMemberParticipationAtVersion 使用 Room configuration_version CAS
// 持久化 group Room 成员参与状态。
func (s *Service) SetRoomMemberParticipationAtVersion(
	ctx context.Context,
	roomID string,
	agentID string,
	paused bool,
	expectedVersion int64,
) (*protocol.ConversationContextAggregate, error) {
	if expectedVersion < 1 {
		return nil, errors.New("expected Room configuration_version 必须大于 0")
	}
	return s.setRoomMemberParticipation(
		ctx,
		roomID,
		agentID,
		paused,
		&expectedVersion,
	)
}

func (s *Service) setRoomMemberParticipation(
	ctx context.Context,
	roomID string,
	agentID string,
	paused bool,
	expectedVersion *int64,
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
	var contextValue *protocol.ConversationContextAggregate
	if expectedVersion == nil {
		contextValue, err = s.repository.SetRoomMemberParticipation(
			ctx,
			authctx.OwnerUserID(ctx),
			normalizedRoomID,
			normalizedAgentID,
			paused,
		)
	} else {
		versioned, ok := s.repository.(interface {
			SetRoomMemberParticipationAtVersion(
				context.Context,
				string,
				string,
				string,
				bool,
				int64,
			) (*protocol.ConversationContextAggregate, error)
		})
		if !ok {
			return nil, errors.New("Room repository 不支持成员参与状态资源版本")
		}
		contextValue, err = versioned.SetRoomMemberParticipationAtVersion(
			ctx,
			authctx.OwnerUserID(ctx),
			normalizedRoomID,
			normalizedAgentID,
			paused,
			*expectedVersion,
		)
	}
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrRoomMemberNotFound
	}
	return contextValue, nil
}
