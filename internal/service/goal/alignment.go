// INPUT: 当前 Goal objective/revision、服务端 completion criteria 与模型逐项 evidence report。
// OUTPUT: durable Objective Alignment 审计记录及 Goal completion 的同 round aligned gate。
// POS: Goal 对共享 objectivealignment 契约的生命周期适配；判定内核不依赖 Goal。
package goal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/objectivealignment"
)

// AuditObjectiveAlignmentByModel 验证并保存当前 objective revision 的结构化审计。
func (s *Service) AuditObjectiveAlignmentByModel(
	ctx context.Context,
	goalID string,
	request protocol.AuditGoalObjectiveAlignmentRequest,
) (*protocol.GoalObjectiveAlignmentRecord, error) {
	item, err := s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	if err = authorizeRoomGoalModelMutation(*item, request.AgentID); err != nil {
		return nil, err
	}
	roundID := strings.TrimSpace(request.RoundID)
	if roundID == "" {
		return nil, newGoalInvalidInputError(
			"objective alignment audit requires the current runtime round identity",
		)
	}
	ctx = withBudgetLimitSteeringSuppressed(ctx)
	if err = s.prepareExternalMutation(ctx, strings.TrimSpace(goalID)); err != nil {
		return nil, err
	}
	item, err = s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}

	auditID := s.idFactory("objective_alignment")
	var saved protocol.GoalObjectiveAlignmentRecord
	_, err = s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		if !protocol.IsCurrentGoalStatus(current.Status) {
			return nil, ErrGoalInvalidState
		}
		if authErr := authorizeRoomGoalModelMutation(*current, request.AgentID); authErr != nil {
			return nil, authErr
		}
		if pendingErr := rejectPendingObjectiveTransition(
			*current,
			"audit Goal objective alignment",
		); pendingErr != nil {
			return nil, pendingErr
		}
		if !objectiveRevisionMatches(*current, request.ExpectedObjectiveRevision) {
			return nil, ErrGoalRevisionStale
		}
		target := goalObjectiveAlignmentTarget(*current)
		report, auditErr := objectivealignment.Audit(target, request.Report)
		if auditErr != nil {
			return nil, fmt.Errorf("%w: %v", ErrGoalInvalidInput, auditErr)
		}
		fingerprint, fingerprintErr := objectivealignment.Fingerprint(target)
		if fingerprintErr != nil {
			return nil, fmt.Errorf("%w: %v", ErrGoalInvalidState, fingerprintErr)
		}
		agentID := strings.TrimSpace(request.AgentID)
		if existing, ok := objectiveAlignmentRecordFromGoal(*current); ok &&
			existing.ObjectiveRevision == current.ObjectiveRevision() &&
			existing.TargetFingerprint == fingerprint &&
			existing.RoundID == roundID &&
			existing.AgentID == agentID &&
			objectivealignment.ReportsEqual(existing.Report, report) {
			saved = existing
			return current, nil
		}

		record := protocol.GoalObjectiveAlignmentRecord{
			ID:                auditID,
			ObjectiveRevision: current.ObjectiveRevision(),
			TargetFingerprint: fingerprint,
			RoundID:           roundID,
			AgentID:           agentID,
			Report:            report,
			AuditedAt:         s.nowFn(),
		}
		expectedVersion := current.Version
		current.Metadata = cloneMap(current.Metadata)
		if current.Metadata == nil {
			current.Metadata = map[string]any{}
		}
		current.Metadata[protocol.GoalMetadataObjectiveAlignment] = record
		current.Version++
		current.UpdatedAt = record.AuditedAt
		updated, updateErr := s.persistGoalUpdateWithEvent(
			ctx,
			*current,
			expectedVersion,
			"objective_alignment_audited",
			protocol.GoalUpdateSourceModel,
			roundID,
			map[string]any{
				"objective_revision": record.ObjectiveRevision,
				"target_fingerprint": record.TargetFingerprint,
				"decision":           record.Report.Decision,
				"report":             record.Report,
				"source_agent_id":    record.AgentID,
			},
		)
		if updateErr != nil {
			return nil, updateErr
		}
		saved = record
		return updated, nil
	})
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

func (s *Service) ensureGoalObjectiveAlignmentReady(
	ctx context.Context,
	item protocol.Goal,
	agentID string,
	roundID string,
) error {
	resolution, err := s.resolveGoalExecutionBinding(ctx, item)
	if err != nil {
		return fmt.Errorf("%w: resolve Goal Execution binding: %v", ErrGoalInvalidState, err)
	}
	switch resolution.State {
	case protocol.GoalExecutionBindingStateStandalone,
		protocol.GoalExecutionBindingStateReserved:
		return nil
	case protocol.GoalExecutionBindingStatePending,
		protocol.GoalExecutionBindingStateConflict:
		return fmt.Errorf(
			"%w: Goal Execution binding is %s",
			ErrGoalInvalidState,
			resolution.State,
		)
	case protocol.GoalExecutionBindingStateConfirmed:
	default:
		return fmt.Errorf("%w: Goal Execution binding state is unknown", ErrGoalInvalidState)
	}
	record, ok := objectiveAlignmentRecordFromGoal(item)
	if !ok {
		return fmt.Errorf(
			"%w: %w: a Goal with a confirmed managed WorkGraph binding requires an objective alignment audit in the current round",
			ErrGoalInvalidState,
			ErrGoalAlignmentRefreshRequired,
		)
	}
	if record.ObjectiveRevision != item.ObjectiveRevision() ||
		record.RoundID != strings.TrimSpace(roundID) ||
		(strings.TrimSpace(agentID) != "" &&
			record.AgentID != "" &&
			record.AgentID != strings.TrimSpace(agentID)) {
		return fmt.Errorf(
			"%w: %w: objective alignment audit is stale or belongs to another round or Agent",
			ErrGoalInvalidState,
			ErrGoalAlignmentRefreshRequired,
		)
	}
	target := goalObjectiveAlignmentTarget(item)
	fingerprint, err := objectivealignment.Fingerprint(target)
	if err != nil || fingerprint != record.TargetFingerprint {
		return fmt.Errorf(
			"%w: %w: objective alignment target changed after the audit",
			ErrGoalInvalidState,
			ErrGoalAlignmentRefreshRequired,
		)
	}
	if _, err = objectivealignment.RequireAligned(target, record.Report); err != nil {
		return fmt.Errorf(
			"%w: objective alignment does not prove completion: %v",
			ErrGoalInvalidState,
			err,
		)
	}
	return nil
}

func goalObjectiveAlignmentTarget(item protocol.Goal) objectivealignment.Target {
	return objectivealignment.Target{
		Objective: strings.TrimSpace(item.Objective),
		Criteria: goalMetadataStrings(
			item.Metadata,
			protocol.GoalMetadataCompletionCriteria,
		),
	}
}

func objectiveAlignmentRecordFromGoal(
	item protocol.Goal,
) (protocol.GoalObjectiveAlignmentRecord, bool) {
	var record protocol.GoalObjectiveAlignmentRecord
	if item.Metadata == nil {
		return record, false
	}
	value, exists := item.Metadata[protocol.GoalMetadataObjectiveAlignment]
	if !exists || value == nil {
		return record, false
	}
	if typed, ok := value.(protocol.GoalObjectiveAlignmentRecord); ok {
		record = typed
	} else {
		encoded, err := json.Marshal(value)
		if err != nil || json.Unmarshal(encoded, &record) != nil {
			return protocol.GoalObjectiveAlignmentRecord{}, false
		}
	}
	if strings.TrimSpace(record.ID) == "" ||
		record.ObjectiveRevision <= 0 ||
		strings.TrimSpace(record.TargetFingerprint) == "" ||
		strings.TrimSpace(record.RoundID) == "" ||
		record.AuditedAt.IsZero() {
		return protocol.GoalObjectiveAlignmentRecord{}, false
	}
	return record, true
}
