// INPUT: Room Goal metadata 与当前模型 Agent 身份。
// OUTPUT: creator/lead 归属、权限校验与协作审计状态判定。
// POS: Room Goal metadata 业务语义的唯一解释入口。
package goal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// 这些函数把 Room Goal 存在 metadata 里的键解释成业务判定（谁是负责人、是否已有协作审计证据）。
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
		updated, updateErr := s.persistGoalUpdateWithEvent(ctx, *current, expectedVersion, "room_lead_changed", protocol.GoalUpdateSourceSystem, "", map[string]any{
			"previous_agent_id": previousAgentID,
			"agent_id":          agentID,
		})
		if updateErr != nil {
			return nil, updateErr
		}
		return updated, nil
	})
}

// RoomCollaborationObserved 判断同一 Room Goal 生命周期内是否已有非负责人可见协作审计事实。
func RoomCollaborationObserved(goal protocol.Goal) bool {
	return protocol.GoalMetadataBool(goal.Metadata, protocol.GoalMetadataRoomGoalCollaborationObserved)
}

// CurrentModelMutationAuthority 返回当前 Agent 负责的 current Goal 快照。
func (s *Service) CurrentModelMutationAuthority(
	ctx context.Context,
	sessionKey string,
	ownerUserID string,
	agentID string,
) (*protocol.Goal, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, fmt.Errorf(
			"%w: Goal mutation authority requires the current agent identity",
			ErrGoalForbidden,
		)
	}
	item, err := s.CurrentOptionalForOwner(ctx, sessionKey, ownerUserID)
	if err != nil || item == nil {
		return item, err
	}
	parsed := protocol.ParseSessionKey(item.SessionKey)
	switch parsed.Kind {
	case protocol.SessionKeyKindRoom:
		if err = authorizeRoomGoalModelMutation(*item, agentID); err != nil {
			return nil, err
		}
	case protocol.SessionKeyKindAgent:
		if strings.TrimSpace(parsed.AgentID) != agentID {
			return nil, fmt.Errorf(
				"%w: only the Goal session Agent %s may mutate this Goal",
				ErrGoalForbidden,
				strings.TrimSpace(parsed.AgentID),
			)
		}
	default:
		return nil, fmt.Errorf(
			"%w: unsupported Goal session identity",
			ErrGoalForbidden,
		)
	}
	return item, nil
}

func applyServerRoomGoalUpdate(
	item *protocol.Goal,
	request protocol.UpdateGoalRequest,
	mutation *goalUpdateMutation,
) {
	if item == nil || mutation == nil || !protocol.IsRoomSharedSessionKey(item.SessionKey) {
		return
	}
	leadAgentID := strings.TrimSpace(request.RoomLeadAgentID)
	leadAgentName := strings.TrimSpace(request.RoomLeadAgentName)
	if leadAgentID != "" &&
		(RoomLeadAgentID(*item) != leadAgentID || RoomLeadAgentName(*item) != leadAgentName) {
		item.Metadata = cloneMap(item.Metadata)
		if item.Metadata == nil {
			item.Metadata = map[string]any{}
		}
		item.Metadata[protocol.GoalMetadataRoomGoalScope] = "room"
		item.Metadata[protocol.GoalMetadataRoomGoalLeadAgentID] = leadAgentID
		if leadAgentName == "" {
			delete(item.Metadata, protocol.GoalMetadataRoomGoalLeadAgentName)
		} else {
			item.Metadata[protocol.GoalMetadataRoomGoalLeadAgentName] = leadAgentName
		}
		mutation.changed = true
		mutation.payload["room_lead_agent_id"] = leadAgentID
	}
}

// RecordRoomGoalCollaborationEvidence 记录非负责人在房间可见回复中参与了 Room Goal。
func (s *Service) RecordRoomGoalCollaborationEvidence(ctx context.Context, goalID string, roundID string, agentID string, expectedRevision ...int64) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	item, err := s.repo.GetGoal(ctx, strings.TrimSpace(goalID))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrGoalNotFound
	}
	return s.recordRoomGoalCollaborationEvidenceForGoal(ctx, item, strings.TrimSpace(roundID), strings.TrimSpace(agentID), firstExpectedObjectiveRevision(expectedRevision))
}

func (s *Service) recordRoomGoalCollaborationEvidenceForGoal(ctx context.Context, item *protocol.Goal, roundID string, agentID string, expectedRevision int64) (*protocol.Goal, error) {
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		if err := rejectPendingObjectiveTransition(*current, "record Room collaboration evidence"); err != nil {
			return nil, err
		}
		if !objectiveRevisionMatches(*current, expectedRevision) {
			return nil, ErrGoalRevisionStale
		}
		return s.recordRoomGoalCollaborationEvidenceForLoadedGoal(ctx, current, roundID, agentID)
	})
}

func (s *Service) recordRoomGoalCollaborationEvidenceForLoadedGoal(ctx context.Context, item *protocol.Goal, roundID string, agentID string) (*protocol.Goal, error) {
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive ||
		!protocol.IsRoomSharedSessionKey(item.SessionKey) ||
		agentID == "" ||
		agentID == RoomLeadAgentID(*item) {
		return item, nil
	}
	if RoomCollaborationObserved(*item) {
		return item, nil
	}
	expectedVersion := item.Version
	item.Metadata = cloneMap(item.Metadata)
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	item.Metadata[protocol.GoalMetadataRoomGoalCollaborationObserved] = true
	item.Metadata[protocol.GoalMetadataRoomGoalCollaborationAgentID] = agentID
	if roundID != "" {
		item.Metadata[protocol.GoalMetadataRoomGoalCollaborationRoundID] = roundID
	}
	item.Metadata[protocol.GoalMetadataRoomGoalCollaborationObservedAt] = s.nowFn().UTC().Format(time.RFC3339)
	item.Version++
	item.UpdatedAt = s.nowFn()
	updated, err := s.persistGoalUpdateWithEvent(ctx, *item, expectedVersion, "room_collaboration_observed", protocol.GoalUpdateSourceSystem, roundID, map[string]any{
		"agent_id": agentID,
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

type roomGoalCompletionReadiness interface {
	RoomGoalCompletionReport(context.Context, protocol.Goal, string, string) (RoomGoalCompletionReport, error)
}

// RoomGoalCompletionReport 是 Room 当前事实对 Goal complete 的单次一致读取。
type RoomGoalCompletionReport struct {
	Blocker string
}

// SetRoomGoalCompletionReadiness 注入 Room 运行中工作检查。
func (s *Service) SetRoomGoalCompletionReadiness(readiness roomGoalCompletionReadiness) {
	s.roomCompletion = readiness
}

func (s *Service) ensureRoomGoalCompletionReady(
	ctx context.Context,
	item protocol.Goal,
	agentID string,
	roundID string,
) (RoomGoalCompletionReport, error) {
	if !protocol.IsRoomSharedSessionKey(item.SessionKey) || s.roomCompletion == nil {
		return RoomGoalCompletionReport{}, nil
	}
	readiness, err := s.roomCompletion.RoomGoalCompletionReport(
		ctx,
		item,
		strings.TrimSpace(agentID),
		strings.TrimSpace(roundID),
	)
	if err != nil {
		return RoomGoalCompletionReport{}, fmt.Errorf("check Room Goal completion readiness: %w", err)
	}
	if blocker := strings.TrimSpace(readiness.Blocker); blocker != "" {
		return readiness, fmt.Errorf("%w: Room Goal still has outstanding work: %s", ErrGoalInvalidState, blocker)
	}
	return readiness, nil
}
