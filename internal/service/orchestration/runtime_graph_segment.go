// INPUT: trusted WorkBinding or Nexus mutation receipt, durable Runtime NodeRun history, and the managed Execution read model.
// OUTPUT: exact Work Item/Assignment/Attempt segment metadata for tools sharing one physical Agent round.
// POS: Runtime Graph identity layer between provider lifecycle events and UI projection; DM self-assignment may segment one round, while Room remains binding-driven.
package orchestration

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	runtimeGraphExecutionIDMetadataKey    = "execution_id"
	runtimeGraphWorkItemIDMetadataKey     = "work_item_id"
	runtimeGraphAssignmentIDMetadataKey   = "assignment_id"
	runtimeGraphAttemptIDMetadataKey      = "attempt_id"
	runtimeGraphSegmentSourceMetadataKey  = "execution_segment_source"
	runtimeGraphSegmentBoundaryKey        = "execution_segment_boundary"
	runtimeGraphSegmentBoundaryAssign     = "assign_work"
	runtimeGraphSegmentBoundaryUnresolved = "unresolved"
)

type runtimeExecutionSegment struct {
	ExecutionID  string
	WorkItemID   string
	AssignmentID string
	AttemptID    string
	Source       string
}

func (segment runtimeExecutionSegment) valid() bool {
	return strings.TrimSpace(segment.ExecutionID) != "" &&
		strings.TrimSpace(segment.WorkItemID) != "" &&
		strings.TrimSpace(segment.AssignmentID) != "" &&
		strings.TrimSpace(segment.AttemptID) != ""
}

func runtimeExecutionSegmentFromActor(actor ActorContext) runtimeExecutionSegment {
	binding := actor.WorkBinding
	if binding == nil {
		return runtimeExecutionSegment{}
	}
	segment := runtimeExecutionSegment{
		ExecutionID:  strings.TrimSpace(binding.ExecutionID),
		WorkItemID:   strings.TrimSpace(binding.WorkItemID),
		AssignmentID: strings.TrimSpace(binding.AssignmentID),
		AttemptID:    strings.TrimSpace(binding.AttemptID),
		Source:       "work_binding",
	}
	if !segment.valid() {
		return runtimeExecutionSegment{}
	}
	return segment
}

func runtimeExecutionSegmentFromNode(
	node protocol.ExecutionRuntimeNodeRun,
) runtimeExecutionSegment {
	segment := runtimeExecutionSegment{
		ExecutionID:  runtimeGraphMetadataString(node, runtimeGraphExecutionIDMetadataKey),
		WorkItemID:   runtimeGraphMetadataString(node, runtimeGraphWorkItemIDMetadataKey),
		AssignmentID: runtimeGraphMetadataString(node, runtimeGraphAssignmentIDMetadataKey),
		AttemptID:    runtimeGraphMetadataString(node, runtimeGraphAttemptIDMetadataKey),
		Source:       runtimeGraphMetadataString(node, runtimeGraphSegmentSourceMetadataKey),
	}
	if !segment.valid() {
		return runtimeExecutionSegment{}
	}
	return segment
}

func runtimeExecutionSegmentWorkItemID(
	node protocol.ExecutionRuntimeNodeRun,
) string {
	segment := runtimeExecutionSegmentFromNode(node)
	if !segment.valid() {
		return ""
	}
	switch segment.Source {
	case "work_binding", "assign_work_receipt", "legacy_assign_interval":
		return segment.WorkItemID
	default:
		return ""
	}
}

func applyRuntimeExecutionSegment(
	metadata map[string]any,
	segment runtimeExecutionSegment,
) {
	if metadata == nil || !segment.valid() {
		return
	}
	metadata[runtimeGraphExecutionIDMetadataKey] = segment.ExecutionID
	metadata[runtimeGraphWorkItemIDMetadataKey] = segment.WorkItemID
	metadata[runtimeGraphAssignmentIDMetadataKey] = segment.AssignmentID
	metadata[runtimeGraphAttemptIDMetadataKey] = segment.AttemptID
	if strings.TrimSpace(segment.Source) != "" {
		metadata[runtimeGraphSegmentSourceMetadataKey] = strings.TrimSpace(segment.Source)
	}
}

func clearRuntimeExecutionSegment(metadata map[string]any) {
	if metadata == nil {
		return
	}
	delete(metadata, runtimeGraphExecutionIDMetadataKey)
	delete(metadata, runtimeGraphWorkItemIDMetadataKey)
	delete(metadata, runtimeGraphAssignmentIDMetadataKey)
	delete(metadata, runtimeGraphAttemptIDMetadataKey)
	delete(metadata, runtimeGraphSegmentSourceMetadataKey)
	delete(metadata, runtimeGraphSegmentBoundaryKey)
}

func latestRuntimeExecutionSegment(
	nodes []protocol.ExecutionRuntimeNodeRun,
	identity runtimeGraphIdentity,
) runtimeExecutionSegment {
	candidates := make([]protocol.ExecutionRuntimeNodeRun, 0, len(nodes))
	for _, node := range nodes {
		if strings.TrimSpace(node.AgentRoundID) != identity.AgentRoundID ||
			(node.AgentID != "" && strings.TrimSpace(node.AgentID) != identity.AgentID) {
			continue
		}
		candidates = append(candidates, node)
	}
	slices.SortFunc(candidates, func(left, right protocol.ExecutionRuntimeNodeRun) int {
		leftAt := left.UpdatedAt
		if leftAt.IsZero() {
			leftAt = left.StartedAt
		}
		rightAt := right.UpdatedAt
		if rightAt.IsZero() {
			rightAt = right.StartedAt
		}
		if order := leftAt.Compare(rightAt); order != 0 {
			return order
		}
		return strings.Compare(left.ID, right.ID)
	})
	active := runtimeExecutionSegment{}
	for _, node := range candidates {
		if runtimeGraphMetadataString(node, runtimeGraphSegmentBoundaryKey) ==
			runtimeGraphSegmentBoundaryUnresolved {
			active = runtimeExecutionSegment{}
			continue
		}
		segment := runtimeExecutionSegmentFromNode(node)
		if !segment.valid() ||
			(identity.ExecutionID != "" && segment.ExecutionID != identity.ExecutionID) {
			continue
		}
		if node.Kind == protocol.ExecutionRuntimeNodeTool &&
			runtimeGraphCanonicalToolLeaf(node.Name) == "assignwork" &&
			runtimeGraphMetadataString(node, runtimeGraphSegmentBoundaryKey) !=
				runtimeGraphSegmentBoundaryAssign {
			// A started/rejected/unresolved assign may carry the preceding segment
			// while it runs, but only a verified success receipt may establish the
			// next segment for later provider messages.
			continue
		}
		active = segment
	}
	return active
}

// runtimeExecutionSegmentFromMutation accepts only the exact assignment and
// attempt refs issued by a successful Nexus assign_work result, then resolves
// them against the authoritative DM snapshot. Room never enters this path:
// its coordinator and workers keep their explicit Lead/WorkBinding lanes.
func (s *Service) runtimeExecutionSegmentFromMutation(
	ctx context.Context,
	actor ActorContext,
	identity runtimeGraphIdentity,
	toolName string,
	evidence runtimeGraphNodeEvidence,
) runtimeExecutionSegment {
	executionID := strings.TrimSpace(evidence.executionID)
	if s == nil || s.repository == nil ||
		actor.ScopeKind != protocol.ExecutionScopeDM ||
		runtimeGraphCanonicalToolLeaf(toolName) != "assignwork" ||
		evidence.mutationOutcome != protocol.MutationResultApplied ||
		executionID == "" ||
		(identity.ExecutionID != "" &&
			executionID != identity.ExecutionID) {
		return runtimeExecutionSegment{}
	}
	assignmentID, assignmentOK := uniqueRuntimeMutationRef(evidence.changed, "assignment:")
	attemptID, attemptOK := uniqueRuntimeMutationRef(evidence.changed, "attempt:")
	if !assignmentOK || !attemptOK {
		return runtimeExecutionSegment{}
	}
	snapshot, err := s.repository.GetSnapshot(ctx, executionID)
	if err != nil || snapshot == nil ||
		snapshot.Execution.ID != executionID ||
		snapshot.Execution.OwnerUserID != identity.OwnerUserID ||
		snapshot.Execution.SessionKey != identity.SessionKey ||
		snapshot.Execution.ScopeKind != protocol.ExecutionScopeDM ||
		strings.TrimSpace(snapshot.Execution.CoordinatorAgentID) != identity.AgentID {
		return runtimeExecutionSegment{}
	}
	assignment := findAssignmentByID(snapshot, assignmentID)
	attempt := findAttemptByID(snapshot, attemptID)
	if assignment == nil || attempt == nil ||
		assignment.ExecutionID != executionID ||
		assignment.OwnerAgentID != identity.AgentID ||
		assignment.Strategy != protocol.AssignmentStrategySelf ||
		attempt.ExecutionID != executionID ||
		attempt.AssignmentID != assignment.ID ||
		attempt.WorkItemID != assignment.WorkItemID ||
		attempt.ExecutorKind != protocol.AttemptExecutorAgent ||
		attempt.ExecutorAgentID != identity.AgentID ||
		strings.TrimSpace(attempt.ParentAttemptID) != "" ||
		strings.TrimSpace(attempt.RootRoundID) != identity.RootRoundID ||
		strings.TrimSpace(attempt.RuntimeRoundID) != identity.RuntimeRoundID ||
		strings.TrimSpace(attempt.AgentRoundID) != identity.AgentRoundID {
		return runtimeExecutionSegment{}
	}
	return runtimeExecutionSegment{
		ExecutionID:  executionID,
		WorkItemID:   assignment.WorkItemID,
		AssignmentID: assignment.ID,
		AttemptID:    attempt.ID,
		Source:       "assign_work_receipt",
	}
}

func uniqueRuntimeMutationRef(refs []string, prefix string) (string, bool) {
	value := ""
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if !strings.HasPrefix(ref, prefix) {
			continue
		}
		candidate := strings.TrimSpace(strings.TrimPrefix(ref, prefix))
		if candidate == "" || (value != "" && value != candidate) {
			return "", false
		}
		value = candidate
	}
	return value, value != ""
}

// recoverDMSelfAssignmentRuntimeSegments repairs older DM graphs that predate
// persisted segment metadata. This is not nearest-time guessing: a successful
// assign_work lifecycle must uniquely contain the durable root Attempt creation
// for the same Agent round. Ambiguous boundaries clear the active segment.
func recoverDMSelfAssignmentRuntimeSegments(
	view *protocol.ExecutionView,
	nodes []protocol.ExecutionRuntimeNodeRun,
) {
	if view == nil || view.ScopeKind != protocol.ExecutionScopeDM || len(nodes) == 0 {
		return
	}
	segmentsByAttempt := make(map[string]runtimeExecutionSegment)
	attemptsByRound := make(map[string][]runtimeSegmentAttempt)
	for _, item := range view.WorkItems {
		for _, attempt := range item.Attempts {
			if strings.TrimSpace(attempt.ParentAttemptID) != "" ||
				strings.TrimSpace(attempt.ID) == "" ||
				strings.TrimSpace(attempt.AssignmentID) == "" ||
				strings.TrimSpace(attempt.AgentRoundID) == "" {
				continue
			}
			segment := runtimeExecutionSegment{
				ExecutionID:  view.ID,
				WorkItemID:   item.ID,
				AssignmentID: attempt.AssignmentID,
				AttemptID:    attempt.ID,
				Source:       "legacy_assign_interval",
			}
			segmentsByAttempt[attempt.ID] = segment
			attemptsByRound[attempt.AgentRoundID] = append(
				attemptsByRound[attempt.AgentRoundID],
				runtimeSegmentAttempt{
					segment:   segment,
					agentID:   firstNonEmpty(attempt.ExecutorAgentID, item.OwnerAgentID),
					createdAt: attempt.CreatedAt,
				},
			)
		}
	}
	recoveredByNode := make(map[string]runtimeExecutionSegment)
	candidateByNode := make(map[string]runtimeExecutionSegment)
	matchCountByAttempt := make(map[string]int)
	for _, node := range nodes {
		if node.Kind != protocol.ExecutionRuntimeNodeTool ||
			node.Status != protocol.ExecutionRuntimeNodeSucceeded ||
			runtimeGraphCanonicalToolLeaf(node.Name) != "assignwork" ||
			node.FinishedAt == nil {
			continue
		}
		var matched runtimeExecutionSegment
		matches := 0
		for _, attempt := range attemptsByRound[node.AgentRoundID] {
			if attempt.agentID != "" && node.AgentID != "" && attempt.agentID != node.AgentID {
				continue
			}
			if attempt.createdAt.Before(node.StartedAt) || attempt.createdAt.After(*node.FinishedAt) {
				continue
			}
			matched = attempt.segment
			matches++
		}
		if matches == 1 {
			candidateByNode[node.ID] = matched
			matchCountByAttempt[matched.AttemptID]++
		}
	}
	for nodeID, candidate := range candidateByNode {
		if matchCountByAttempt[candidate.AttemptID] == 1 {
			recoveredByNode[nodeID] = candidate
		}
	}

	activeByRound := make(map[string]runtimeExecutionSegment)
	primaryRoundKey := make(map[string]string)
	for index := range nodes {
		node := &nodes[index]
		roundID := strings.TrimSpace(node.AgentRoundID)
		roundKey := roundID + "\x00" + strings.TrimSpace(node.AgentID)
		if node.Kind == protocol.ExecutionRuntimeNodeAgent && strings.TrimSpace(node.AgentID) != "" {
			primaryRoundKey[roundID] = roundKey
		}
		if _, exists := activeByRound[roundKey]; !exists {
			if primaryKey := primaryRoundKey[roundID]; primaryKey != "" {
				if active := activeByRound[primaryKey]; active.valid() {
					activeByRound[roundKey] = active
				}
			}
		}
		explicit := runtimeExecutionSegmentFromNode(*node)
		expected, explicitMatches := segmentsByAttempt[explicit.AttemptID]
		explicitMatches = explicitMatches && explicit.ExecutionID == expected.ExecutionID &&
			explicit.WorkItemID == expected.WorkItemID &&
			explicit.AssignmentID == expected.AssignmentID
		isAssignWork := node.Kind == protocol.ExecutionRuntimeNodeTool &&
			runtimeGraphCanonicalToolLeaf(node.Name) == "assignwork"
		if isAssignWork {
			if node.Metadata == nil {
				node.Metadata = make(map[string]any)
			}
			recovered := recoveredByNode[node.ID]
			boundary := runtimeGraphMetadataString(*node, runtimeGraphSegmentBoundaryKey)
			switch {
			case explicitMatches && explicit.Source == "assign_work_receipt" &&
				boundary == runtimeGraphSegmentBoundaryAssign:
				activeByRound[roundKey] = explicit
				if primaryKey := primaryRoundKey[roundID]; primaryKey != "" {
					activeByRound[primaryKey] = explicit
				}
			case recovered.valid():
				clearRuntimeExecutionSegment(node.Metadata)
				applyRuntimeExecutionSegment(node.Metadata, recovered)
				node.Metadata[runtimeGraphSegmentBoundaryKey] = runtimeGraphSegmentBoundaryAssign
				activeByRound[roundKey] = recovered
				if primaryKey := primaryRoundKey[roundID]; primaryKey != "" {
					activeByRound[primaryKey] = recovered
				}
			case node.Status == protocol.ExecutionRuntimeNodeSucceeded:
				// A succeeded assign without an exact receipt or unique durable
				// Attempt is an unresolved boundary. Do not leak the prior segment
				// into subsequent work.
				clearRuntimeExecutionSegment(node.Metadata)
				node.Metadata[runtimeGraphSegmentBoundaryKey] = runtimeGraphSegmentBoundaryUnresolved
				delete(activeByRound, roundKey)
				if primaryKey := primaryRoundKey[roundID]; primaryKey != "" {
					delete(activeByRound, primaryKey)
				}
			case explicitMatches:
				// A failed/running assign did not establish a new Assignment. Its
				// inherited segment remains the last exact ownership boundary.
				activeByRound[roundKey] = explicit
				if primaryKey := primaryRoundKey[roundID]; primaryKey != "" {
					activeByRound[primaryKey] = explicit
				}
			}
			continue
		}
		if explicitMatches {
			activeByRound[roundKey] = explicit
			if primaryKey := primaryRoundKey[roundID]; primaryKey != "" {
				activeByRound[primaryKey] = explicit
			}
			continue
		}
		if active := activeByRound[roundKey]; active.valid() {
			if node.Metadata == nil {
				node.Metadata = make(map[string]any)
			}
			applyRuntimeExecutionSegment(node.Metadata, active)
		}
	}
}

type runtimeSegmentAttempt struct {
	segment   runtimeExecutionSegment
	agentID   string
	createdAt time.Time
}
