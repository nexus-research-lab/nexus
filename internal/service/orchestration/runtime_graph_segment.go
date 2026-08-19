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
	case "work_binding", "assign_work_receipt", "take_over_work_receipt", "legacy_assign_interval":
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
	resolvedBoundarySegments := make(map[string]runtimeExecutionSegment)
	conflictingBoundaryGroups := make(map[string]struct{})
	for _, node := range nodes {
		if strings.TrimSpace(node.AgentRoundID) != identity.AgentRoundID ||
			(node.AgentID != "" && strings.TrimSpace(node.AgentID) != identity.AgentID) {
			continue
		}
		candidates = append(candidates, node)
		if node.Kind != protocol.ExecutionRuntimeNodeTool ||
			runtimeGraphAssignmentBoundaryOperationForNode(node) == "" ||
			runtimeGraphMetadataString(node, runtimeGraphSegmentBoundaryKey) !=
				runtimeGraphSegmentBoundaryAssign {
			continue
		}
		segment := runtimeExecutionSegmentFromNode(node)
		if !segment.valid() ||
			(identity.ExecutionID != "" && segment.ExecutionID != identity.ExecutionID) {
			continue
		}
		groupKey := runtimeGraphAssignmentBoundaryGroupKey(node)
		if previous := resolvedBoundarySegments[groupKey]; previous.valid() &&
			previous.AttemptID != segment.AttemptID {
			delete(resolvedBoundarySegments, groupKey)
			conflictingBoundaryGroups[groupKey] = struct{}{}
			continue
		}
		if _, conflict := conflictingBoundaryGroups[groupKey]; !conflict {
			resolvedBoundarySegments[groupKey] = segment
		}
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
			if resolved := resolvedBoundarySegments[runtimeGraphAssignmentBoundaryGroupKey(node)]; resolved.valid() {
				active = resolved
				continue
			}
			active = runtimeExecutionSegment{}
			continue
		}
		segment := runtimeExecutionSegmentFromNode(node)
		if !segment.valid() ||
			(identity.ExecutionID != "" && segment.ExecutionID != identity.ExecutionID) {
			continue
		}
		if node.Kind == protocol.ExecutionRuntimeNodeTool &&
			runtimeGraphAssignmentBoundaryOperationForNode(node) != "" &&
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
// attempt refs issued by a successful Nexus assignment-boundary result, then resolves
// them against the authoritative DM snapshot. Room never enters this path:
// its coordinator and workers keep their explicit Lead/WorkBinding lanes.
func (s *Service) runtimeExecutionSegmentFromMutation(
	ctx context.Context,
	actor ActorContext,
	identity runtimeGraphIdentity,
	operation string,
	evidence runtimeGraphNodeEvidence,
) runtimeExecutionSegment {
	if !runtimeGraphAssignmentBoundaryOperation(operation) ||
		evidence.mutationOutcome != protocol.MutationResultApplied {
		return runtimeExecutionSegment{}
	}
	return s.runtimeExecutionSegmentFromChangedRefs(
		ctx,
		actor,
		identity,
		evidence.executionID,
		evidence.changed,
		runtimeGraphAssignmentBoundarySource(operation),
	)
}

func (s *Service) runtimeExecutionSegmentFromChangedRefs(
	ctx context.Context,
	actor ActorContext,
	identity runtimeGraphIdentity,
	executionID string,
	changed []string,
	source string,
) runtimeExecutionSegment {
	executionID = strings.TrimSpace(executionID)
	if s == nil || s.repository == nil ||
		actor.ScopeKind != protocol.ExecutionScopeDM || executionID == "" ||
		(identity.ExecutionID != "" && executionID != identity.ExecutionID) {
		return runtimeExecutionSegment{}
	}
	assignmentID, assignmentOK := uniqueRuntimeMutationRef(changed, "assignment:")
	attemptID, attemptOK := uniqueRuntimeMutationRef(changed, "attempt:")
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
		Source:       strings.TrimSpace(source),
	}
}

func runtimeGraphAssignmentBoundaryOperation(operation string) bool {
	switch strings.TrimSpace(operation) {
	case "assign_work", "take_over_work":
		return true
	default:
		return false
	}
}

func runtimeGraphAssignmentBoundarySource(operation string) string {
	operation = strings.TrimSpace(operation)
	if !runtimeGraphAssignmentBoundaryOperation(operation) {
		return ""
	}
	return operation + "_receipt"
}

func runtimeGraphAssignmentBoundaryOperationForNode(
	node protocol.ExecutionRuntimeNodeRun,
) string {
	operation := strings.TrimSpace(runtimeGraphMetadataString(
		node,
		runtimeGraphCommandOperationMetadataKey,
	))
	if runtimeGraphAssignmentBoundaryOperation(operation) && runtimeGraphIsCommandTransport(node) {
		return operation
	}
	if !runtimeGraphIsLegacyManagedTransport(node.Name) {
		return ""
	}
	leaf := runtimeGraphCanonicalToolLeaf(node.Name)
	switch leaf {
	case "assignwork":
		return "assign_work"
	case "takeoverwork":
		return "take_over_work"
	default:
		return ""
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
// assignment-boundary lifecycle must uniquely contain the durable root Attempt creation
// for the same Agent round. The provider Tool lifecycle and its host receipt can be
// persisted as two rows for one exact command request; they form one semantic boundary,
// not two consecutive assignments. Ambiguous boundaries clear the active segment.
func recoverDMSelfAssignmentRuntimeSegments(
	view *protocol.ExecutionView,
	nodes []protocol.ExecutionRuntimeNodeRun,
) {
	if view == nil || view.ScopeKind != protocol.ExecutionScopeDM || len(nodes) == 0 {
		return
	}
	segmentsByAttempt := make(map[string]runtimeExecutionSegment)
	attempts := make([]runtimeSegmentAttempt, 0)
	for _, item := range view.WorkItems {
		for _, attempt := range item.Attempts {
			if strings.TrimSpace(attempt.ParentAttemptID) != "" ||
				strings.TrimSpace(attempt.ID) == "" ||
				strings.TrimSpace(attempt.AssignmentID) == "" {
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
			attempts = append(attempts, runtimeSegmentAttempt{
				segment:      segment,
				agentID:      firstNonEmpty(attempt.ExecutorAgentID, item.OwnerAgentID),
				agentRoundID: strings.TrimSpace(attempt.AgentRoundID),
				createdAt:    attempt.CreatedAt,
			})
		}
	}

	// A receipt can arrive before its provider Tool lifecycle at an assistant
	// checkpoint. Both rows carry the exact host request identity. Group them so
	// the later companion cannot erase the segment established by the first row.
	boundaryNodesByGroup := make(map[string][]int)
	for index, node := range nodes {
		if node.Kind != protocol.ExecutionRuntimeNodeTool ||
			runtimeGraphAssignmentBoundaryOperationForNode(node) == "" {
			continue
		}
		groupKey := runtimeGraphAssignmentBoundaryGroupKey(node)
		boundaryNodesByGroup[groupKey] = append(boundaryNodesByGroup[groupKey], index)
	}

	recoveredByNode := make(map[string]runtimeExecutionSegment)
	candidateByGroup := make(map[string]runtimeExecutionSegment)
	matchCountByAttempt := make(map[string]int)
	for groupKey, nodeIndexes := range boundaryNodesByGroup {
		var matched runtimeExecutionSegment
		matchedAttempts := make(map[string]struct{})
		for _, nodeIndex := range nodeIndexes {
			node := nodes[nodeIndex]
			if node.Status != protocol.ExecutionRuntimeNodeSucceeded || node.FinishedAt == nil {
				continue
			}
			for _, attempt := range attempts {
				if !runtimeGraphAttemptMayMatchAssignmentBoundary(view, node, attempt, groupKey) {
					continue
				}
				if attempt.createdAt.Before(node.StartedAt) || attempt.createdAt.After(*node.FinishedAt) {
					continue
				}
				matched = attempt.segment
				matchedAttempts[attempt.segment.AttemptID] = struct{}{}
			}
		}
		if len(matchedAttempts) == 1 {
			candidateByGroup[groupKey] = matched
			matchCountByAttempt[matched.AttemptID]++
		}
	}
	for groupKey, candidate := range candidateByGroup {
		if matchCountByAttempt[candidate.AttemptID] == 1 {
			for _, nodeIndex := range boundaryNodesByGroup[groupKey] {
				recoveredByNode[nodes[nodeIndex].ID] = candidate
			}
		}
	}
	// Newer receipt rows may already have the exact segment even when their
	// provider lifecycle companion does not. Propagate only a single consistent
	// verified segment across that exact request group.
	for groupKey, nodeIndexes := range boundaryNodesByGroup {
		var resolved runtimeExecutionSegment
		conflict := false
		for _, nodeIndex := range nodeIndexes {
			node := nodes[nodeIndex]
			segment := runtimeExecutionSegmentFromNode(node)
			expected, ok := segmentsByAttempt[segment.AttemptID]
			operation := runtimeGraphAssignmentBoundaryOperationForNode(node)
			if !ok || segment.ExecutionID != expected.ExecutionID ||
				segment.WorkItemID != expected.WorkItemID ||
				segment.AssignmentID != expected.AssignmentID ||
				segment.Source != runtimeGraphAssignmentBoundarySource(operation) ||
				runtimeGraphMetadataString(node, runtimeGraphSegmentBoundaryKey) != runtimeGraphSegmentBoundaryAssign {
				continue
			}
			if resolved.valid() && resolved.AttemptID != segment.AttemptID {
				conflict = true
				break
			}
			resolved = segment
		}
		if conflict || !resolved.valid() {
			continue
		}
		for _, nodeIndex := range nodeIndexes {
			recoveredByNode[nodes[nodeIndex].ID] = resolved
		}
		delete(candidateByGroup, groupKey)
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
		isAssignmentBoundary := node.Kind == protocol.ExecutionRuntimeNodeTool &&
			runtimeGraphAssignmentBoundaryOperationForNode(*node) != ""
		if isAssignmentBoundary {
			if node.Metadata == nil {
				node.Metadata = make(map[string]any)
			}
			recovered := recoveredByNode[node.ID]
			boundary := runtimeGraphMetadataString(*node, runtimeGraphSegmentBoundaryKey)
			switch {
			case explicitMatches &&
				explicit.Source == runtimeGraphAssignmentBoundarySource(
					runtimeGraphAssignmentBoundaryOperationForNode(*node),
				) &&
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
	segment      runtimeExecutionSegment
	agentID      string
	agentRoundID string
	createdAt    time.Time
}

func runtimeGraphAssignmentBoundaryGroupKey(
	node protocol.ExecutionRuntimeNodeRun,
) string {
	domain := runtimeGraphMetadataString(node, runtimeGraphCommandDomainMetadataKey)
	operation := runtimeGraphMetadataString(node, runtimeGraphCommandOperationMetadataKey)
	requestID := runtimeGraphMetadataString(node, runtimeGraphCommandRequestIDMetadataKey)
	if domain != "" && operation != "" && requestID != "" {
		return strings.Join([]string{
			"request",
			strings.TrimSpace(node.AgentRoundID),
			strings.TrimSpace(node.AgentID),
			domain,
			operation,
			requestID,
		}, "\x00")
	}
	return "node\x00" + strings.TrimSpace(node.ID)
}

func runtimeGraphAttemptMayMatchAssignmentBoundary(
	view *protocol.ExecutionView,
	node protocol.ExecutionRuntimeNodeRun,
	attempt runtimeSegmentAttempt,
	groupKey string,
) bool {
	if attempt.agentID != "" && node.AgentID != "" && attempt.agentID != node.AgentID {
		return false
	}
	if attempt.agentRoundID != "" {
		return attempt.agentRoundID == strings.TrimSpace(node.AgentRoundID)
	}
	// block_work can release the first self Assignment before the runtime has
	// persisted its round identity. Recover that historical Attempt only from an
	// exact CLI request and only for the DM coordinator acting as the same Agent.
	return strings.HasPrefix(groupKey, "request\x00") &&
		strings.TrimSpace(view.CoordinatorAgentID) != "" &&
		strings.TrimSpace(node.AgentID) == strings.TrimSpace(view.CoordinatorAgentID) &&
		attempt.agentID == strings.TrimSpace(view.CoordinatorAgentID)
}
