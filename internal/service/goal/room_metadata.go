// INPUT: Room Goal metadata 与当前模型 Agent 身份。
// OUTPUT: creator/lead 归属、权限校验与协作状态判定。
// POS: Room Goal metadata 业务语义的唯一解释入口。
package goal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// 这些函数把 Room Goal 存在 metadata 里的键解释成业务判定（谁是负责人、是否要求/已有协作证据）。
// protocol 只负责定义键的词汇（常量）和通用的 typed-map 取值；这层"读键 → 业务语义"的解释属于 goal 域。

// RoomLeadAgentID 返回 Room Goal 的负责人 Agent。
func RoomLeadAgentID(goal protocol.Goal) string {
	return protocol.GoalMetadataString(goal.Metadata, protocol.GoalMetadataRoomGoalLeadAgentID)
}

// RoomLeadAgentName 返回 Room Goal 的负责人展示名。
func RoomLeadAgentName(goal protocol.Goal) string {
	return protocol.GoalMetadataString(goal.Metadata, protocol.GoalMetadataRoomGoalLeadAgentName)
}

func initializeRoomGoalOwnershipMetadata(
	sessionKey string,
	metadata map[string]any,
	creatorAgentID string,
	creatorAgentName string,
	leadAgentID string,
	leadAgentName string,
) map[string]any {
	if !protocol.IsRoomSharedSessionKey(sessionKey) {
		return metadata
	}
	metadata = cloneMap(metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata[protocol.GoalMetadataRoomGoalScope] = "room"
	delete(metadata, protocol.GoalMetadataRoomGoalCreatorAgentID)
	delete(metadata, protocol.GoalMetadataRoomGoalLeadAgentID)
	delete(metadata, protocol.GoalMetadataRoomGoalLeadAgentName)
	creatorAgentID = strings.TrimSpace(creatorAgentID)
	if creatorAgentID != "" {
		metadata[protocol.GoalMetadataRoomGoalCreatorAgentID] = creatorAgentID
	}
	leadAgentID = strings.TrimSpace(leadAgentID)
	if leadAgentID == "" {
		leadAgentID = creatorAgentID
		leadAgentName = creatorAgentName
	}
	if leadAgentID != "" {
		metadata[protocol.GoalMetadataRoomGoalLeadAgentID] = leadAgentID
	}
	if leadAgentName = strings.TrimSpace(leadAgentName); leadAgentName != "" {
		metadata[protocol.GoalMetadataRoomGoalLeadAgentName] = leadAgentName
	}
	return metadata
}

func verifiedRoomLeadAgentID(runtimeAgentID string, verifiedAgentID string) string {
	if runtimeAgentID = strings.TrimSpace(runtimeAgentID); runtimeAgentID != "" {
		return runtimeAgentID
	}
	return strings.TrimSpace(verifiedAgentID)
}

func verifiedRoomAgentName(
	runtimeAgentID string,
	verifiedAgentID string,
	verifiedAgentName string,
) string {
	if strings.TrimSpace(runtimeAgentID) != strings.TrimSpace(verifiedAgentID) {
		return ""
	}
	return strings.TrimSpace(verifiedAgentName)
}

func preserveRoomGoalOwnershipMetadata(current protocol.Goal, replacement map[string]any) map[string]any {
	replacement = cloneMap(replacement)
	if !protocol.IsRoomSharedSessionKey(current.SessionKey) {
		return replacement
	}
	if replacement == nil {
		replacement = map[string]any{}
	}
	for _, key := range []string{
		protocol.GoalMetadataRoomGoalScope,
		protocol.GoalMetadataRoomGoalCreatorAgentID,
		protocol.GoalMetadataRoomGoalLeadAgentID,
		protocol.GoalMetadataRoomGoalLeadAgentName,
	} {
		if value, exists := current.Metadata[key]; exists {
			replacement[key] = value
		} else {
			delete(replacement, key)
		}
	}
	return replacement
}

func authorizeRoomGoalModelMutation(goal protocol.Goal, agentID string) error {
	if !protocol.IsRoomSharedSessionKey(goal.SessionKey) {
		return nil
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return fmt.Errorf("%w: shared Room Goal mutation requires the current agent identity", ErrGoalForbidden)
	}
	leadAgentID := RoomLeadAgentID(goal)
	if leadAgentID == "" {
		return fmt.Errorf("%w: shared Room Goal has no assigned lead", ErrGoalForbidden)
	}
	if leadAgentID != agentID {
		return fmt.Errorf("%w: only Room Goal lead %s may retarget, complete, or block this Goal", ErrGoalForbidden, leadAgentID)
	}
	return nil
}

// SetRoomGoalLead 由 Room 编排层请求设置或修复共享 Goal 负责人。
// Agent 的 Room 成员身份和展示名必须在 Goal 服务边界通过 owner-scoped
// session ownership verifier 重新证明，不信任调用方携带的成员目录投影。
func (s *Service) SetRoomGoalLead(ctx context.Context, goalID string, agentID string) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	if s.sessionOwnership == nil {
		return nil, fmt.Errorf(
			"%w: Room Goal lead assignment requires the session ownership verifier",
			ErrGoalForbidden,
		)
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, newGoalInvalidInputError("room goal lead agent id must not be empty")
	}
	item, err := s.repo.GetGoal(ctx, strings.TrimSpace(goalID))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrGoalNotFound
	}
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		if !protocol.IsRoomSharedSessionKey(current.SessionKey) || !protocol.IsCurrentGoalStatus(current.Status) {
			return nil, ErrGoalInvalidState
		}
		ownerUserID := protocol.GoalMetadataString(
			current.Metadata,
			protocol.GoalMetadataOwnerUserID,
		)
		_, verifiedAgentID, agentName, verifyErr := s.verifyGoalSessionOwnership(
			ctx,
			current.SessionKey,
			ownerUserID,
			agentID,
		)
		if verifyErr != nil {
			return nil, verifyErr
		}
		if verifiedAgentID != agentID {
			return nil, fmt.Errorf(
				"%w: Room Goal lead identity was not verified",
				ErrGoalForbidden,
			)
		}
		if RoomLeadAgentID(*current) == agentID && RoomLeadAgentName(*current) == agentName {
			return current, nil
		}
		expectedVersion := current.Version
		previousAgentID := RoomLeadAgentID(*current)
		current.Metadata = cloneMap(current.Metadata)
		if current.Metadata == nil {
			current.Metadata = map[string]any{}
		}
		current.Metadata[protocol.GoalMetadataRoomGoalScope] = "room"
		current.Metadata[protocol.GoalMetadataRoomGoalLeadAgentID] = agentID
		if agentName == "" {
			delete(current.Metadata, protocol.GoalMetadataRoomGoalLeadAgentName)
		} else {
			current.Metadata[protocol.GoalMetadataRoomGoalLeadAgentName] = agentName
		}
		current.Version++
		current.UpdatedAt = s.nowFn()
		updated, updateErr := s.repo.UpdateGoal(ctx, *current, expectedVersion)
		if errors.Is(updateErr, sql.ErrNoRows) {
			return nil, ErrGoalVersionStale
		}
		if updateErr != nil {
			return nil, updateErr
		}
		if eventErr := s.appendEvent(ctx, *updated, "room_lead_changed", protocol.GoalUpdateSourceSystem, "", map[string]any{
			"previous_agent_id": previousAgentID,
			"agent_id":          agentID,
		}); eventErr != nil {
			return nil, eventErr
		}
		return updated, nil
	})
}

// RoomCollaborationRequired 判断 Room Goal 是否要求非负责人可见协作。
func RoomCollaborationRequired(goal protocol.Goal) bool {
	return protocol.GoalMetadataBool(goal.Metadata, protocol.GoalMetadataRoomGoalCollaborationRequired)
}

// RoomCollaborationObserved 判断 Room Goal 是否已有非负责人可见协作证据。
func RoomCollaborationObserved(goal protocol.Goal) bool {
	return protocol.GoalMetadataBool(goal.Metadata, protocol.GoalMetadataRoomGoalCollaborationObserved)
}
